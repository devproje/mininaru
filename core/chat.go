// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"fmt"
	"time"

	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/modules/memory"
	"github.com/devproje/mininaru/modules/skill"
	"github.com/google/uuid"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/shared"
)

var streamIdleTimeout = 2 * time.Minute

type ChatMessage struct {
	Role    string
	Content string
}

func chatClient(prov *Provider) openai.Client {
	var opts []option.RequestOption

	if prov.ApiKey != "" {
		opts = append(opts, option.WithAPIKey(prov.ApiKey))
	}

	if prov.BaseUrl != "" {
		opts = append(opts, option.WithBaseURL(prov.BaseUrl))
	}

	return openai.NewClient(opts...)
}

func chatParams(agent *Agent, messages []ChatMessage) openai.ChatCompletionNewParams {
	var union []openai.ChatCompletionMessageParamUnion
	var msg ChatMessage

	for _, msg = range messages {
		switch msg.Role {
		case "system":
			union = append(union, openai.SystemMessage(msg.Content))
		case "assistant":
			union = append(union, openai.AssistantMessage(msg.Content))
		default:
			union = append(union, openai.UserMessage(msg.Content))
		}
	}

	return chatParamsUnion(agent, union, nil)
}

func chatParamsUnion(agent *Agent, messages []openai.ChatCompletionMessageParamUnion, tools []modules.Tool) openai.ChatCompletionNewParams {
	var params openai.ChatCompletionNewParams

	params.Model = agent.Model
	params.Messages = messages

	switch ThinkingLevel(agent.ThinkingLevel) {
	case Low:
		params.ReasoningEffort = shared.ReasoningEffortLow
	case Medium:
		params.ReasoningEffort = shared.ReasoningEffortMedium
	case High, Max:
		params.ReasoningEffort = shared.ReasoningEffortHigh
	}

	if len(tools) > 0 {
		params.Tools = toolParams(tools)
	}

	return params
}

func ChatCompletion(ctx context.Context, agent *Agent, messages []ChatMessage) (*openai.ChatCompletion, error) {
	var prov *Provider
	var client openai.Client
	var params openai.ChatCompletionNewParams
	var resp *openai.ChatCompletion

	var err error

	prov, err = ProviderActive()
	if err != nil {
		return nil, err
	}

	client = chatClient(prov)
	params = chatParams(agent, messages)

	resp, err = client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		err = fmt.Errorf("provider returned no completion choices")
		return nil, err
	}

	resp.Model = agent.Name

	return resp, nil
}

func ChatCompletionStream(ctx context.Context, agent *Agent, messages []ChatMessage, onChunk func(openai.ChatCompletionChunk) error) error {
	var prov *Provider
	var client openai.Client
	var params openai.ChatCompletionNewParams
	var stream *ssestream.Stream[openai.ChatCompletionChunk]
	var chunk openai.ChatCompletionChunk

	var err error

	prov, err = ProviderActive()
	if err != nil {
		return err
	}

	client = chatClient(prov)
	params = chatParams(agent, messages)

	stream = client.Chat.Completions.NewStreaming(ctx, params)
	defer stream.Close()

	for stream.Next() {
		chunk = stream.Current()
		chunk.Model = agent.Name

		err = onChunk(chunk)
		if err != nil {
			return err
		}
	}

	err = stream.Err()
	if err != nil {
		return err
	}

	return nil
}

func chatStreamRound(ctx context.Context, prov *Provider, params openai.ChatCompletionNewParams, onChunk func(openai.ChatCompletionChunk)) (*openai.ChatCompletionAccumulator, error) {
	var client openai.Client
	var stream *ssestream.Stream[openai.ChatCompletionChunk]
	var chunk openai.ChatCompletionChunk
	var accumulator openai.ChatCompletionAccumulator
	var roundCtx context.Context
	var cancel context.CancelFunc
	var idle *time.Timer

	var err error

	client = chatClient(prov)

	roundCtx, cancel = context.WithCancel(ctx)
	defer cancel()

	idle = time.AfterFunc(streamIdleTimeout, cancel)
	defer idle.Stop()

	stream = client.Chat.Completions.NewStreaming(roundCtx, params)
	defer stream.Close()

	for stream.Next() {
		idle.Reset(streamIdleTimeout)

		chunk = stream.Current()
		chunk.Model = params.Model
		accumulator.AddChunk(chunk)

		onChunk(chunk)
	}

	err = stream.Err()
	if err != nil {
		if roundCtx.Err() != nil && ctx.Err() == nil {
			return nil, fmt.Errorf("provider stopped sending data (idle for %s)", streamIdleTimeout)
		}

		return nil, err
	}

	if len(accumulator.Choices) == 0 {
		return nil, fmt.Errorf("provider returned no completion choices")
	}

	return &accumulator, nil
}

func SendChatMessage(ctx context.Context, agent *Agent, session *Session, anchor string, depth int, onChunk func(openai.ChatCompletionChunk), onTool func(name, status, message string), approve ApproveFunc) error {
	var history []*Message
	var union []openai.ChatCompletionMessageParamUnion
	var pending *Message
	var tools []modules.Tool
	var prov *Provider
	var params openai.ChatCompletionNewParams
	var accumulator *openai.ChatCompletionAccumulator
	var message openai.ChatCompletionMessage
	var round int
	var call openai.ChatCompletionMessageToolCall
	var record *ToolCall
	var result string
	var memoryIndex string
	var skillCatalog string
	var assistant Message
	var updateErr error

	var err error

	history, err = MessageList(session.Id)
	if err != nil {
		return err
	}

	union, pending, err = historyUnion(history)
	if err != nil {
		return err
	}
	if pending == nil {
		err = fmt.Errorf("session %s has no pending user message", session.Id)
		return err
	}

	memoryIndex = memory.LoadIndex(agent.Id)
	if memoryIndex != "" {
		union = append([]openai.ChatCompletionMessageParamUnion{openai.SystemMessage(memoryIndex)}, union...)
	}
	skillCatalog = skill.Catalog()
	if skillCatalog != "" {
		union = append([]openai.ChatCompletionMessageParamUnion{openai.SystemMessage(skillCatalog)}, union...)
	}
	if agent.Soul != "" {
		union = append([]openai.ChatCompletionMessageParamUnion{openai.SystemMessage(agent.Soul)}, union...)
	}

	prov, err = ProviderActive()
	if err != nil {
		return err
	}

	tools = buildTools(anchor, session.Id, agent, depth, onTool, approve)

	for round = 0; round < maxToolRounds; round++ {
		params = chatParamsUnion(agent, union, tools)

		accumulator, err = chatStreamRound(ctx, prov, params, onChunk)
		if err != nil {
			updateErr = MessageUpdate(pending.Id, &Message{Status: "failed", Error: err.Error()})
			if updateErr != nil {
				return updateErr
			}

			return err
		}

		message = accumulator.Choices[0].Message
		if len(message.ToolCalls) == 0 {
			err = MessageUpdate(pending.Id, &Message{Status: "completed"})
			if err != nil {
				return err
			}

			assistant = Message{Id: uuid.NewString(), SessionId: session.Id, Role: "assistant", Content: message.Content, Status: "completed"}

			return MessageCreate(&assistant)
		}

		union = append(union, assistantToolCallMessage(message))

		for _, call = range message.ToolCalls {
			record, err = toolCallStart(pending.Id, call)
			if err != nil {
				return err
			}

			if onTool != nil {
				onTool(record.Name, "started", "")
			}

			result, err = executeTool(ctx, tools, call.Function.Name, call.Function.Arguments, approve)
			if err != nil {
				updateErr = ToolCallUpdate(record.Id, &ToolCall{Status: "failed", Error: err.Error(), Result: "error: " + err.Error()})
				if updateErr != nil {
					return updateErr
				}

				if onTool != nil {
					onTool(record.Name, "failed", err.Error())
				}

				union = append(union, openai.ToolMessage("error: "+err.Error(), call.ID))
				continue
			}

			if isScreenshotResult(result) {
				err = ToolCallUpdate(record.Id, &ToolCall{Status: "completed", Result: screenshotPlaceholder})
				if err != nil {
					return err
				}

				if onTool != nil {
					onTool(record.Name, "finished", "")
				}

				union = append(union, openai.ToolMessage(screenshotPlaceholder, call.ID))
				union = append(union, openai.UserMessage([]openai.ChatCompletionContentPartUnionParam{
					openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{URL: result}),
				}))
				continue
			}

			err = ToolCallUpdate(record.Id, &ToolCall{Status: "completed", Result: result})
			if err != nil {
				return err
			}

			if record.Name == skill.ToolName {
				skillUseRecord(session.Id, record)
			}

			if onTool != nil {
				onTool(record.Name, "finished", "")
			}

			union = append(union, openai.ToolMessage(result, call.ID))
		}
	}

	err = fmt.Errorf("tool call limit exceeded after %d rounds", maxToolRounds)
	updateErr = MessageUpdate(pending.Id, &Message{Status: "failed", Error: err.Error()})
	if updateErr != nil {
		return updateErr
	}

	return err
}
