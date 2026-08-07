package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/modules"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/ssestream"
)

type completionRun struct {
	AI             *openai.Client
	Params         openai.ChatCompletionNewParams
	Defs           []modules.Def
	AllowDangerous bool
	MessageId      string
	OnContent      func(string)
	OnReasoning    func(string)
	OnTool         ToolEventFunc
	Approve        ToolApprovalFunc
}

type Completion struct {
	Content   string
	Reasoning string
	Usage     openai.CompletionUsage
}

func (r *completionRun) stream(ctx context.Context, reply, reasoning *strings.Builder) (*openai.ChatCompletionAccumulator, error) {
	var accumulator openai.ChatCompletionAccumulator
	var stream *ssestream.Stream[openai.ChatCompletionChunk]
	var chunk openai.ChatCompletionChunk
	var delta openai.ChatCompletionChunkChoiceDelta
	var thought string

	var err error

	stream = r.AI.Chat.Completions.NewStreaming(ctx, r.Params)

	for stream.Next() {
		chunk = stream.Current()
		accumulator.AddChunk(chunk)
		if len(chunk.Choices) == 0 {
			continue
		}

		delta = chunk.Choices[0].Delta

		thought = deltaReasoning(delta)
		if thought != "" {
			reasoning.WriteString(thought)

			if r.OnReasoning != nil {
				r.OnReasoning(thought)
			}
		}

		if delta.Content == "" {
			continue
		}

		reply.WriteString(delta.Content)

		if r.OnContent != nil {
			r.OnContent(delta.Content)
		}
	}

	err = stream.Err()
	stream.Close()
	if err != nil {
		return nil, err
	}

	if len(accumulator.Choices) == 0 {
		return nil, fmt.Errorf("model returned no choices")
	}

	return &accumulator, nil
}

func (r *completionRun) dispatch(ctx context.Context, message openai.ChatCompletionMessage) error {
	var call openai.ChatCompletionMessageToolCall
	var record *ToolCall

	var err error

	r.Params.Messages = append(r.Params.Messages, assistantToolCallMessage(message))

	for _, call = range message.ToolCalls {
		record, err = toolCallStart(r.MessageId, call)
		if err != nil {
			return err
		}

		if r.OnTool != nil {
			r.OnTool(ToolEvent{Phase: ToolEventStarted, CallId: record.CallId, Name: record.Name,
				Arguments: record.Arguments, Status: record.Status})
		}

		record, err = executeTool(ctx, record, r.Defs, r.AllowDangerous, r.Approve)
		if err != nil {
			return err
		}

		if r.OnTool != nil {
			r.OnTool(ToolEvent{Phase: ToolEventFinished, CallId: record.CallId, Name: record.Name, Arguments: record.Arguments,
				Result: record.Result, Status: record.Status, Error: record.Error})
		}

		r.Params.Messages = append(r.Params.Messages, openai.ToolMessage(record.Result, call.ID))
	}

	return nil
}

func (r *completionRun) execute(ctx context.Context) (*Completion, error) {
	var result Completion
	var reply strings.Builder
	var reasoning strings.Builder
	var accumulator *openai.ChatCompletionAccumulator
	var message openai.ChatCompletionMessage
	var round int

	var err error

	if r.AI == nil {
		return nil, fmt.Errorf("no available provider client")
	}

	for round = 0; round < maxToolRounds; round++ {
		reply.Reset()

		accumulator, err = r.stream(ctx, &reply, &reasoning)
		if err != nil {
			return nil, err
		}

		if accumulator.Usage.TotalTokens != 0 {
			result.Usage = accumulator.Usage
		}

		message = accumulator.Choices[0].Message
		if len(message.ToolCalls) == 0 {
			result.Content = reply.String()
			result.Reasoning = reasoning.String()

			return &result, nil
		}

		err = r.dispatch(ctx, message)
		if err != nil {
			return nil, err
		}
	}

	return nil, fmt.Errorf("tool call limit exceeded after %d rounds", maxToolRounds)
}

func Complete(ctx context.Context, agent *NaruAgent, messages []openai.ChatCompletionMessageParamUnion,
	defs []modules.Def, thinking string, onContent, onReasoning func(string)) (*Completion, error) {
	var params openai.ChatCompletionNewParams
	var run completionRun

	if agent == nil {
		return nil, fmt.Errorf("agent is required to complete")
	}
	if agent.AI == nil {
		return nil, fmt.Errorf("agent %s has no available provider client", agent.Id)
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("at least one message is required")
	}

	if agent.Role != "" || agent.Soul != "" {
		params.Messages = append(params.Messages, openai.SystemMessage(agent.Role+"\n"+agent.Soul))
	}
	params.Messages = append(params.Messages, messages...)

	params.Model = agent.Model
	params.StreamOptions.IncludeUsage = param.NewOpt(true)

	defs = permittedTools(defs)
	if len(defs) > 0 {
		params.Tools = toolParams(defs)
	}

	if thinking != "" && thinking != config.ThinkingOff && config.ThinkingValid(thinking) {
		params.ReasoningEffort = openai.ReasoningEffort(thinking)
	}

	run = completionRun{AI: agent.AI, Params: params, Defs: defs, OnContent: onContent, OnReasoning: onReasoning}

	return run.execute(ctx)
}
