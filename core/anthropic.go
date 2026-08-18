// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"github.com/devproje/mininaru/modules"
	"github.com/openai/openai-go"
)

type openAIWireMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	ToolCallID string          `json:"tool_call_id"`
	ToolCalls  []struct {
		ID       string `json:"id"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	} `json:"tool_calls"`
}

type openAIWireContent struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	ImageURL struct {
		URL string `json:"url"`
	} `json:"image_url"`
}

func splitDataURL(value string) (string, string, error) {
	var header, data string
	var ok bool

	header, data, ok = strings.Cut(value, ",")
	if !ok || !strings.HasPrefix(header, "data:") || !strings.HasSuffix(header, ";base64") {
		return "", "", fmt.Errorf("unsupported image data URL")
	}

	return strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64"), data, nil
}

func anthropicContent(raw json.RawMessage) ([]anthropic.ContentBlockParamUnion, error) {
	var text string
	var parts []openAIWireContent
	var blocks []anthropic.ContentBlockParamUnion
	var part openAIWireContent
	var media, data string

	var err error

	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if raw[0] == '"' {
		err = json.Unmarshal(raw, &text)
		if err != nil {
			return nil, err
		}
		return []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(text)}, nil
	}

	err = json.Unmarshal(raw, &parts)
	if err != nil {
		return nil, err
	}
	for _, part = range parts {
		switch part.Type {
		case "text":
			blocks = append(blocks, anthropic.NewTextBlock(part.Text))
		case "image_url":
			if strings.HasPrefix(part.ImageURL.URL, "data:") {
				media, data, err = splitDataURL(part.ImageURL.URL)
				if err != nil {
					return nil, err
				}
				blocks = append(blocks, anthropic.NewImageBlockBase64(media, data))
			} else {
				blocks = append(blocks, anthropic.NewImageBlock(anthropic.URLImageSourceParam{URL: part.ImageURL.URL}))
			}
		}
	}

	return blocks, nil
}

func anthropicMessages(messages []openai.ChatCompletionMessageParamUnion) ([]anthropic.TextBlockParam, []anthropic.MessageParam, error) {
	var message openai.ChatCompletionMessageParamUnion
	var raw []byte
	var wire openAIWireMessage
	var blocks []anthropic.ContentBlockParamUnion
	var block anthropic.ContentBlockParamUnion
	var system []anthropic.TextBlockParam
	var converted []anthropic.MessageParam
	var result string
	var input any
	var call struct {
		ID       string
		Name     string
		Argument string
	}
	var index int

	var err error

	for _, message = range messages {
		raw, err = json.Marshal(message)
		if err != nil {
			return nil, nil, err
		}
		wire = openAIWireMessage{}
		err = json.Unmarshal(raw, &wire)
		if err != nil {
			return nil, nil, err
		}

		blocks, err = anthropicContent(wire.Content)
		if err != nil {
			return nil, nil, err
		}
		if wire.Role == "system" {
			for _, block = range blocks {
				if block.OfText != nil {
					system = append(system, *block.OfText)
				}
			}
			continue
		}
		if wire.Role == "tool" {
			json.Unmarshal(wire.Content, &result)
			converted = append(converted, anthropic.NewUserMessage(anthropic.NewToolResultBlock(wire.ToolCallID, result, false)))
			continue
		}

		for index = range wire.ToolCalls {
			call.ID = wire.ToolCalls[index].ID
			call.Name = wire.ToolCalls[index].Function.Name
			call.Argument = wire.ToolCalls[index].Function.Arguments
			input = map[string]any{}
			if call.Argument != "" {
				err = json.Unmarshal([]byte(call.Argument), &input)
				if err != nil {
					return nil, nil, err
				}
			}
			blocks = append(blocks, anthropic.NewToolUseBlock(call.ID, input, call.Name))
		}
		if wire.Role == "assistant" {
			converted = append(converted, anthropic.NewAssistantMessage(blocks...))
		} else {
			converted = append(converted, anthropic.NewUserMessage(blocks...))
		}
	}

	return system, converted, nil
}

func anthropicTools(defs []modules.Def) []anthropic.ToolUnionParam {
	var def modules.Def
	var properties any
	var required []string
	var valuesString []string
	var valuesAny []any
	var value any
	var name string
	var ok bool
	var schema anthropic.ToolInputSchemaParam
	var tool anthropic.ToolParam
	var tools []anthropic.ToolUnionParam

	for _, def = range defs {
		properties = def.Parameters["properties"]
		required = nil
		valuesString, ok = def.Parameters["required"].([]string)
		if ok {
			required = valuesString
		} else {
			valuesAny, ok = def.Parameters["required"].([]any)
			if ok {
				for _, value = range valuesAny {
					name, ok = value.(string)
					if ok {
						required = append(required, name)
					}
				}
			}
		}
		schema = anthropic.ToolInputSchemaParam{Properties: properties, Required: required}
		tool = anthropic.ToolParam{Name: def.Name, Description: param.NewOpt(def.Description), InputSchema: schema}
		tools = append(tools, anthropic.ToolUnionParam{OfTool: &tool})
	}

	return tools
}

func anthropicCacheControl(policy string) anthropic.CacheControlEphemeralParam {
	var control anthropic.CacheControlEphemeralParam

	if policy == CacheOff {
		return control
	}

	control = anthropic.NewCacheControlEphemeralParam()
	if policy == CacheEphemeral1h {
		control.TTL = anthropic.CacheControlEphemeralTTLTTL1h
	}

	return control
}

func (r *completionRun) anthropicStream(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
	var stream *ssestream.Stream[anthropic.MessageStreamEventUnion]
	var message anthropic.Message
	var event anthropic.MessageStreamEventUnion

	var err error

	stream = r.Anthropic.Messages.NewStreaming(ctx, params)

	for stream.Next() {
		event = stream.Current()
		err = message.Accumulate(event)
		if err != nil {
			stream.Close()
			return nil, err
		}
		if event.Type == "content_block_delta" && event.Delta.Type == "text_delta" && r.OnContent != nil {
			r.OnContent(event.Delta.Text)
		}
		if event.Type == "content_block_delta" && event.Delta.Type == "thinking_delta" && r.OnReasoning != nil {
			r.OnReasoning(event.Delta.Thinking)
		}
	}

	err = stream.Err()
	stream.Close()
	if err != nil {
		return nil, err
	}

	return &message, nil
}

func (r *completionRun) executeAnthropic(ctx context.Context) (*Completion, error) {
	var params anthropic.MessageNewParams
	var system []anthropic.TextBlockParam
	var messages []anthropic.MessageParam
	var message *anthropic.Message
	var result Completion
	var assistantBlocks []anthropic.ContentBlockParamUnion
	var toolResults []anthropic.ContentBlockParamUnion
	var record *ToolCall
	var call openai.ChatCompletionMessageToolCall
	var input any
	var round int
	var block anthropic.ContentBlockUnion

	var err error

	system, messages, err = anthropicMessages(r.Params.Messages)
	if err != nil {
		return nil, err
	}
	params = anthropic.MessageNewParams{Model: anthropic.Model(r.Params.Model), MaxTokens: 8192, System: system,
		Messages: messages, Tools: anthropicTools(r.Defs), CacheControl: anthropicCacheControl(r.Provider.CachePolicy())}
	if r.Params.ReasoningEffort != "" {
		params.Thinking = anthropic.ThinkingConfigParamUnion{OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{}}
	}

	for round = 0; round < maxToolRounds; round++ {
		result.Content = ""
		message, err = r.anthropicStream(ctx, params)
		if err != nil {
			return nil, err
		}

		result.Usage.PromptTokens += message.Usage.InputTokens + message.Usage.CacheReadInputTokens + message.Usage.CacheCreationInputTokens
		result.Usage.CompletionTokens += message.Usage.OutputTokens
		result.Usage.TotalTokens += message.Usage.InputTokens + message.Usage.CacheReadInputTokens + message.Usage.CacheCreationInputTokens + message.Usage.OutputTokens
		result.Usage.CachedTokens += message.Usage.CacheReadInputTokens
		result.Usage.CacheWriteTokens += message.Usage.CacheCreationInputTokens
		result.ContextTokens = message.Usage.InputTokens + message.Usage.CacheReadInputTokens + message.Usage.CacheCreationInputTokens

		assistantBlocks = nil
		toolResults = nil
		for _, block = range message.Content {
			switch block.Type {
			case "text":
				result.Content += block.Text
				assistantBlocks = append(assistantBlocks, anthropic.NewTextBlock(block.Text))
			case "thinking":
				result.Reasoning += block.Thinking
				assistantBlocks = append(assistantBlocks, anthropic.NewThinkingBlock(block.Signature, block.Thinking))
			case "tool_use":
				input = map[string]any{}
				if len(block.Input) > 0 {
					json.Unmarshal(block.Input, &input)
				}
				assistantBlocks = append(assistantBlocks, anthropic.NewToolUseBlock(block.ID, input, block.Name))
				call = openai.ChatCompletionMessageToolCall{ID: block.ID}
				call.Function.Name = block.Name
				call.Function.Arguments = string(block.Input)
				record, err = toolCallStart(r.MessageId, call)
				if err != nil {
					return nil, err
				}
				if r.OnTool != nil {
					r.OnTool(ToolEvent{Phase: ToolEventStarted, CallId: record.CallId, Name: record.Name, Arguments: record.Arguments, Status: record.Status})
				}
				record, err = executeTool(ctx, r.SessionId, record, r.Defs, r.AllowDangerous, r.AllowPrivileged, r.Approve)
				if err != nil {
					return nil, err
				}
				if r.OnTool != nil {
					r.OnTool(ToolEvent{Phase: ToolEventFinished, CallId: record.CallId, Name: record.Name, Arguments: record.Arguments,
						Result: record.Result, Status: record.Status, Error: record.Error})
				}
				toolResults = append(toolResults, anthropic.NewToolResultBlock(block.ID, record.Result, record.Status != MessageCompleted))
			}
		}
		if len(toolResults) == 0 {
			return &result, nil
		}
		params.Messages = append(params.Messages, anthropic.NewAssistantMessage(assistantBlocks...))
		params.Messages = append(params.Messages, anthropic.NewUserMessage(toolResults...))
	}

	return nil, fmt.Errorf("tool call limit exceeded after %d rounds", maxToolRounds)
}
