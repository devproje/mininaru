// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/modules/memory"
	"github.com/devproje/mininaru/modules/skill"
	"github.com/google/uuid"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/pagination"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/shared"
)

type ChatMessage struct {
	Role    string
	Content string
	Images  []string
}

var streamIdleTimeout = 2 * time.Minute

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

func ProviderModelNames(ctx context.Context, prov *Provider) ([]string, error) {
	var client openai.Client
	var reqCtx context.Context
	var cancel context.CancelFunc
	var page *pagination.Page[openai.Model]
	var m openai.Model
	var names []string

	var err error

	client = chatClient(prov)

	reqCtx, cancel = context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	page, err = client.Models.List(reqCtx)
	if err != nil {
		return nil, err
	}

	for _, m = range page.Data {
		names = append(names, m.ID)
	}

	return names, nil
}

func modelNamePart(agentModel string) string {
	var modelName string
	var ok bool

	_, modelName, ok = strings.Cut(agentModel, ":")
	if !ok {
		return agentModel
	}

	return modelName
}

func resolveProviderModel(agentModel string) (*Provider, string, error) {
	var provName, modelName string
	var ok bool
	var prov *Provider

	var err error

	provName, modelName, ok = strings.Cut(agentModel, ":")
	if !ok {
		return nil, "", fmt.Errorf("agent model %q is not in \"provider:model\" form", agentModel)
	}

	prov, err = ProviderByName(provName)
	if err != nil {
		return nil, "", err
	}

	return prov, modelName, nil
}

func chatParams(agent *Agent, messages []ChatMessage, modelName string) openai.ChatCompletionNewParams {
	var union []openai.ChatCompletionMessageParamUnion
	var msg ChatMessage

	for _, msg = range messages {
		switch msg.Role {
		case "system":
			union = append(union, openai.SystemMessage(msg.Content))
		case "assistant":
			union = append(union, openai.AssistantMessage(msg.Content))
		default:
			if len(msg.Images) > 0 {
				union = append(union, imageUserMessage(msg.Content, msg.Images))
				continue
			}

			union = append(union, openai.UserMessage(msg.Content))
		}
	}

	return chatParamsUnion(agent, union, nil, modelName)
}

func chatParamsUnion(agent *Agent, messages []openai.ChatCompletionMessageParamUnion, tools []modules.Tool, modelName string) openai.ChatCompletionNewParams {
	var params openai.ChatCompletionNewParams

	params.Model = modelName
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

	params.StreamOptions.IncludeUsage = openai.Bool(true)

	return params
}

type noCacheCtxKey struct{}

func WithNoCache(ctx context.Context) context.Context {
	return context.WithValue(ctx, noCacheCtxKey{}, true)
}

func noCacheFromContext(ctx context.Context) bool {
	var noCache bool

	noCache, _ = ctx.Value(noCacheCtxKey{}).(bool)

	return noCache
}

func cacheBreakpointIndex(messages []openai.ChatCompletionMessageParamUnion) int {
	var last int
	var index int

	last = -1
	for index = range messages {
		if messages[index].OfSystem == nil {
			break
		}

		last = index
	}

	return last
}

func cacheRequestOptions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion) []option.RequestOption {
	var index int
	var text string

	if noCacheFromContext(ctx) {
		return nil
	}

	index = cacheBreakpointIndex(messages)
	if index < 0 {
		return nil
	}

	text = messages[index].OfSystem.Content.OfString.Value
	if text == "" {
		return nil
	}

	return []option.RequestOption{
		option.WithJSONSet(fmt.Sprintf("messages.%d.content", index), []map[string]string{{"type": "text", "text": text}}),
		option.WithJSONSet(fmt.Sprintf("messages.%d.content.0.cache_control", index), map[string]string{"type": "ephemeral"}),
	}
}

func ChatCompletion(ctx context.Context, agent *Agent, messages []ChatMessage) (*openai.ChatCompletion, error) {
	var prov *Provider
	var modelName string
	var client openai.Client
	var params openai.ChatCompletionNewParams
	var resp *openai.ChatCompletion

	var err error

	prov, modelName, err = resolveProviderModel(agent.Model)
	if err != nil {
		return nil, err
	}

	err = ChatContextCheck(agent, messages)
	if err != nil {
		return nil, err
	}

	client = chatClient(prov)
	params = chatParams(agent, messages, modelName)

	resp, err = client.Chat.Completions.New(ctx, params, cacheRequestOptions(ctx, params.Messages)...)
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
	var modelName string
	var client openai.Client
	var params openai.ChatCompletionNewParams
	var stream *ssestream.Stream[openai.ChatCompletionChunk]
	var chunk openai.ChatCompletionChunk

	var err error

	prov, modelName, err = resolveProviderModel(agent.Model)
	if err != nil {
		return err
	}

	err = ChatContextCheck(agent, messages)
	if err != nil {
		return err
	}

	client = chatClient(prov)
	params = chatParams(agent, messages, modelName)

	stream = client.Chat.Completions.NewStreaming(ctx, params, cacheRequestOptions(ctx, params.Messages)...)
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
	var roundCtx context.Context
	var cancel context.CancelFunc
	var idle *time.Timer
	var stream *ssestream.Stream[openai.ChatCompletionChunk]
	var chunk openai.ChatCompletionChunk
	var accumulator openai.ChatCompletionAccumulator

	var err error

	client = chatClient(prov)

	roundCtx, cancel = context.WithCancel(ctx)
	defer cancel()

	idle = time.AfterFunc(streamIdleTimeout, cancel)
	defer idle.Stop()

	stream = client.Chat.Completions.NewStreaming(roundCtx, params, cacheRequestOptions(roundCtx, params.Messages)...)
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

func failedToolResult(result string, err error) string {
	var text string

	text = "error: " + err.Error()
	if result != "" {
		text += "\noutput:\n" + result
	}

	return text
}

func prependSystemContext(union []openai.ChatCompletionMessageParamUnion, summary *Summary, memoryIndex string, skillCatalog string, soul string) []openai.ChatCompletionMessageParamUnion {
	if summary != nil {
		union = append([]openai.ChatCompletionMessageParamUnion{openai.SystemMessage(summary.Content)}, union...)
	}
	if memoryIndex != "" {
		union = append([]openai.ChatCompletionMessageParamUnion{openai.SystemMessage(memoryIndex)}, union...)
	}
	if skillCatalog != "" {
		union = append([]openai.ChatCompletionMessageParamUnion{openai.SystemMessage(skillCatalog)}, union...)
	}
	if soul != "" {
		union = append([]openai.ChatCompletionMessageParamUnion{openai.SystemMessage(soul)}, union...)
	}

	return union
}

func SendChatMessage(ctx context.Context, agent *Agent, session *Session, anchor string, depth int, onChunk func(openai.ChatCompletionChunk), onTool func(name, status, message string), approve ApproveFunc, ask AskFunc) error {
	var history []*Message
	var tail []*Message
	var summary *Summary
	var union []openai.ChatCompletionMessageParamUnion
	var pending *Message
	var memoryIndex string
	var skillCatalog string
	var prov *Provider
	var modelName string
	var tools []modules.Tool
	var contextErr error
	var limitErr *ContextLengthError
	var isContextLimit bool
	var updateErr error
	var round int
	var params openai.ChatCompletionNewParams
	var accumulator *openai.ChatCompletionAccumulator
	var message openai.ChatCompletionMessage
	var assistant Message
	var call openai.ChatCompletionMessageToolCall
	var record *ToolCall
	var result string
	var finishedMessage string

	var err error

	history, err = MessageList(session.Id)
	if err != nil {
		return err
	}

	summary, err = SummaryLoad(session.Id)
	if err != nil {
		return err
	}
	tail = summaryTail(history, summary)

	union, pending, err = historyUnion(tail)
	if err != nil {
		return err
	}
	if pending == nil {
		err = fmt.Errorf("session %s has no pending user message", session.Id)
		return err
	}

	memoryIndex = memory.LoadIndex(agent.Id)
	skillCatalog = skill.Catalog()
	union = prependSystemContext(union, summary, memoryIndex, skillCatalog, agent.Soul)

	prov, modelName, err = resolveProviderModel(agent.Model)
	if err != nil {
		return err
	}

	tools = buildTools(anchor, session.Id, agent, depth, onTool, approve, ask)
	contextErr = contextLimit(agent, union, tools)
	if contextErr != nil {
		isContextLimit = errors.As(contextErr, &limitErr)
		if isContextLimit {
			if onTool != nil {
				onTool("compact", "started", "")
			}

			summary, contextErr = compactHistory(ctx, agent, session, prov, summary, tail, pending)

			if onTool != nil {
				onTool("compact", "finished", "")
			}
		}
		if contextErr != nil {
			updateErr = MessageUpdate(pending.Id, &Message{Status: "failed", Error: contextErr.Error()})
			if updateErr != nil {
				return updateErr
			}

			return contextErr
		}

		tail = summaryTail(history, summary)
		union, pending, err = historyUnion(tail)
		if err != nil {
			return err
		}
		union = prependSystemContext(union, summary, memoryIndex, skillCatalog, agent.Soul)

		contextErr = contextLimit(agent, union, tools)
		if contextErr != nil {
			updateErr = MessageUpdate(pending.Id, &Message{Status: "failed", Error: contextErr.Error()})
			if updateErr != nil {
				return updateErr
			}

			return contextErr
		}
	}

	for round = 0; round < maxToolRounds; round++ {
		contextErr = ctx.Err()
		if contextErr != nil {
			updateErr = MessageUpdate(pending.Id, &Message{Status: "failed", Error: contextErr.Error()})
			if updateErr != nil {
				return updateErr
			}

			return contextErr
		}

		contextErr = contextLimit(agent, union, tools)
		if contextErr != nil {
			updateErr = MessageUpdate(pending.Id, &Message{Status: "failed", Error: contextErr.Error()})
			if updateErr != nil {
				return updateErr
			}

			return contextErr
		}

		params = chatParamsUnion(agent, union, tools, modelName)

		accumulator, err = chatStreamRound(ctx, prov, params, onChunk)
		if err != nil {
			updateErr = MessageUpdate(pending.Id, &Message{Status: "failed", Error: err.Error()})
			if updateErr != nil {
				return updateErr
			}

			return err
		}

		if accumulator.Usage.PromptTokens > 0 {
			err = SessionUsageSave(session.Id, uint64(accumulator.Usage.PromptTokens+accumulator.Usage.CompletionTokens), uint64(accumulator.Usage.PromptTokensDetails.CachedTokens))
			if err != nil {
				return err
			}
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
			contextErr = ctx.Err()
			if contextErr != nil {
				updateErr = MessageUpdate(pending.Id, &Message{Status: "failed", Error: contextErr.Error()})
				if updateErr != nil {
					return updateErr
				}

				return contextErr
			}

			record, err = toolCallStart(pending.Id, call)
			if err != nil {
				return err
			}

			if onTool != nil {
				onTool(record.Name, "started", "")
			}

			result, err = executeTool(ctx, tools, session.Id, anchor, call.Function.Name, call.Function.Arguments, approve)
			if err != nil {
				result = failedToolResult(result, err)
				updateErr = ToolCallUpdate(record.Id, &ToolCall{Status: "failed", Error: err.Error(), Result: result})
				if updateErr != nil {
					return updateErr
				}

				if onTool != nil {
					onTool(record.Name, "failed", err.Error())
				}

				union = append(union, openai.ToolMessage(result, call.ID))

				contextErr = ctx.Err()
				if contextErr != nil {
					updateErr = MessageUpdate(pending.Id, &Message{Status: "failed", Error: contextErr.Error()})
					if updateErr != nil {
						return updateErr
					}

					return contextErr
				}
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
				finishedMessage = ""
				if record.Name == "file_write" || record.Name == "file_edit" {
					finishedMessage = result
				}

				onTool(record.Name, "finished", finishedMessage)
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
