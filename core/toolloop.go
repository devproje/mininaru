// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/devproje/mininaru/modules"
	"github.com/google/uuid"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
)

type ApproveFunc func(ctx context.Context, name, arguments string) (string, error)

const maxToolRounds = 50

const screenshotDataPrefix = "data:image/"
const screenshotPlaceholder = "screenshot captured"

func isScreenshotResult(result string) bool {
	return strings.HasPrefix(result, screenshotDataPrefix)
}

func toolParams(tools []modules.Tool) []openai.ChatCompletionToolParam {
	var tool modules.Tool
	var params []openai.ChatCompletionToolParam

	for _, tool = range tools {
		params = append(params, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: param.NewOpt(tool.Description),
				Parameters:  shared.FunctionParameters(tool.Parameters),
			},
		})
	}

	return params
}

func findTool(tools []modules.Tool, name string) *modules.Tool {
	var index int

	for index = range tools {
		if tools[index].Name == name {
			return &tools[index]
		}
	}

	return nil
}

func assistantToolCallMessage(message openai.ChatCompletionMessage) openai.ChatCompletionMessageParamUnion {
	var assistant openai.ChatCompletionAssistantMessageParam
	var call openai.ChatCompletionMessageToolCall

	if message.Content != "" {
		assistant.Content.OfString = param.NewOpt(message.Content)
	}
	for _, call = range message.ToolCalls {
		assistant.ToolCalls = append(assistant.ToolCalls, openai.ChatCompletionMessageToolCallParam{
			ID: call.ID,
			Function: openai.ChatCompletionMessageToolCallFunctionParam{
				Name:      call.Function.Name,
				Arguments: call.Function.Arguments,
			},
		})
	}

	return openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant}
}

func toolCallStart(messageId string, call openai.ChatCompletionMessageToolCall) (*ToolCall, error) {
	var record ToolCall

	var err error

	record = ToolCall{
		Id: uuid.NewString(), MessageId: messageId, CallId: call.ID,
		Name: call.Function.Name, Arguments: call.Function.Arguments, Status: "pending",
	}

	err = ToolCallCreate(&record)
	if err != nil {
		return nil, fmt.Errorf("recording tool call %s failed: %w", call.ID, err)
	}

	return &record, nil
}

func executeTool(ctx context.Context, tools []modules.Tool, name, arguments string, approve ApproveFunc) (string, error) {
	var tool *modules.Tool
	var decision string

	var err error

	tool = findTool(tools, name)
	if tool == nil {
		return "", fmt.Errorf("unknown tool %q", name)
	}

	if tool.Permission == modules.PermissionDangerous && approve != nil {
		decision, err = approve(ctx, name, arguments)
		if err != nil {
			return "", err
		}
		if decision == "deny" {
			return "", fmt.Errorf("user denied dangerous tool %q", name)
		}
	}

	return tool.Execute(ctx, arguments)
}

func replayableCalls(calls []*ToolCall) bool {
	var call *ToolCall

	if len(calls) == 0 {
		return false
	}

	for _, call = range calls {
		if call.CallId == "" || call.Status == "pending" {
			return false
		}
	}

	return true
}

func storedToolCallMessage(calls []*ToolCall) openai.ChatCompletionMessageParamUnion {
	var call *ToolCall
	var assistant openai.ChatCompletionAssistantMessageParam

	for _, call = range calls {
		assistant.ToolCalls = append(assistant.ToolCalls, openai.ChatCompletionMessageToolCallParam{
			ID: call.CallId,
			Function: openai.ChatCompletionMessageToolCallFunctionParam{
				Name:      call.Name,
				Arguments: call.Arguments,
			},
		})
	}

	return openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant}
}

func historyUnion(history []*Message) ([]openai.ChatCompletionMessageParamUnion, *Message, error) {
	var union []openai.ChatCompletionMessageParamUnion
	var item *Message
	var pending *Message
	var calls []*ToolCall
	var call *ToolCall

	var err error

	for _, item = range history {
		if item.Role == "assistant" {
			union = append(union, openai.AssistantMessage(item.Content))
			continue
		}

		union = append(union, openai.UserMessage(item.Content))

		if item.Status == "pending" {
			pending = item
			continue
		}

		calls, err = ToolCallList(item.Id)
		if err != nil {
			return nil, nil, err
		}
		if !replayableCalls(calls) {
			continue
		}

		union = append(union, storedToolCallMessage(calls))
		for _, call = range calls {
			union = append(union, openai.ToolMessage(call.Result, call.CallId))
		}
	}

	return union, pending, nil
}
