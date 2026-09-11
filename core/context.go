// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/modules/memory"
	"github.com/devproje/mininaru/modules/skill"
	"github.com/openai/openai-go"
)

type ContextLengthError struct {
	Limit  uint64
	Tokens uint64
}

type ContextUsage struct {
	Used       uint64 `json:"used"`
	Limit      uint64 `json:"limit"`
	MaxContext uint64 `json:"max_context"`
}

const contextReservePercent = 20
const imageContextTokens = 1024
const maxSummaryChars = 2048
const defaultMaxContext = 24000

var dataImagePattern = regexp.MustCompile(`data:image/[^"\\]+`)

func (err *ContextLengthError) Error() string {
	return fmt.Sprintf("context length exceeds the %d token input budget (estimated %d tokens)", err.Limit, err.Tokens)
}

func contextWindow(agent *Agent) uint64 {
	if agent.MaxContext == 0 {
		return defaultMaxContext
	}

	return agent.MaxContext
}

func contextInputLimit(agent *Agent) uint64 {
	return contextWindow(agent) * (100 - contextReservePercent) / 100
}

func contextTokenEstimate(agent *Agent, union []openai.ChatCompletionMessageParamUnion, tools []modules.Tool) (uint64, error) {
	var params openai.ChatCompletionNewParams
	var buf []byte
	var text string
	var images int
	var tokens uint64

	var err error

	params = chatParamsUnion(agent, union, tools)
	buf, err = json.Marshal(params)
	if err != nil {
		return 0, err
	}

	text = string(buf)
	images = len(dataImagePattern.FindAllString(text, -1))
	text = dataImagePattern.ReplaceAllString(text, "data:image")
	tokens = uint64(len([]rune(text))) + uint64(images*imageContextTokens)

	return tokens, nil
}

func contextLimit(agent *Agent, union []openai.ChatCompletionMessageParamUnion, tools []modules.Tool) error {
	var limit uint64
	var tokens uint64

	var err error

	limit = contextInputLimit(agent)
	if limit == 0 {
		return fmt.Errorf("agent max_context must be greater than zero")
	}

	tokens, err = contextTokenEstimate(agent, union, tools)
	if err != nil {
		return err
	}
	if tokens > limit {
		return &ContextLengthError{Limit: limit, Tokens: tokens}
	}

	return nil
}

func ChatContextCheck(agent *Agent, messages []ChatMessage) error {
	var params openai.ChatCompletionNewParams

	params = chatParams(agent, messages)

	return contextLimit(agent, params.Messages, nil)
}

func SessionContextUsage(agent *Agent, session *Session) (*ContextUsage, error) {
	var history []*Message
	var summary *Summary
	var tail []*Message
	var union []openai.ChatCompletionMessageParamUnion
	var tools []modules.Tool
	var tokens uint64
	var usage ContextUsage
	var memoryIndex string
	var skillCatalog string

	var err error

	history, err = MessageList(session.Id)
	if err != nil {
		return nil, err
	}
	summary, err = SummaryLoad(session.Id)
	if err != nil {
		return nil, err
	}

	tail = summaryTail(history, summary)
	union, _, err = historyUnion(tail)
	if err != nil {
		return nil, err
	}
	if summary != nil {
		union = append([]openai.ChatCompletionMessageParamUnion{openai.SystemMessage(summary.Content)}, union...)
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

	tools = buildTools(session.Cwd, session.Id, agent, 0, nil, nil)
	tokens, err = contextTokenEstimate(agent, union, tools)
	if err != nil {
		return nil, err
	}

	usage = ContextUsage{Used: tokens, Limit: contextInputLimit(agent), MaxContext: contextWindow(agent)}

	return &usage, nil
}

func summaryTail(history []*Message, summary *Summary) []*Message {
	var index int

	if summary == nil {
		return history
	}

	for index = range history {
		if history[index].Id == summary.ThroughMessageId {
			return history[index+1:]
		}
	}

	return history
}

func summaryTranscript(previous string, dropped []*Message) (string, error) {
	var builder strings.Builder
	var item *Message
	var calls []*ToolCall
	var call *ToolCall
	var attachments []*Attachment

	var err error

	builder.WriteString("Write a concise running summary of the conversation. Preserve user decisions, constraints, established facts, completed work, and unfinished work. Replace the previous summary rather than appending to it. Return only the summary.\n\n")
	if previous != "" {
		builder.WriteString("Previous summary:\n")
		builder.WriteString(previous)
		builder.WriteString("\n\n")
	}

	for _, item = range dropped {
		fmt.Fprintf(&builder, "%s: %s\n", item.Role, item.Content)
		if item.Role != "user" {
			continue
		}

		attachments, err = AttachmentList(item.Id)
		if err != nil {
			return "", err
		}
		if len(attachments) > 0 {
			fmt.Fprintf(&builder, "attachments: %d image(s)\n", len(attachments))
		}

		calls, err = ToolCallList(item.Id)
		if err != nil {
			return "", err
		}
		for _, call = range calls {
			fmt.Fprintf(&builder, "tool %s arguments: %s\ntool %s result: %s\n", call.Name, call.Arguments, call.Name, call.Result)
		}
	}

	return builder.String(), nil
}

func compactPrefix(history []*Message, pending *Message) []*Message {
	var pendingIndex int
	var keepIndex int
	var index int

	pendingIndex = -1
	keepIndex = -1
	for index = range history {
		if history[index].Id == pending.Id {
			pendingIndex = index
			break
		}
	}
	if pendingIndex < 1 {
		return nil
	}

	for index = pendingIndex - 1; index >= 0; index-- {
		if history[index].Role == "user" {
			keepIndex = index
			break
		}
	}
	if keepIndex > 0 {
		return history[:keepIndex]
	}

	return history[:pendingIndex]
}

func summaryGroups(history []*Message) [][]*Message {
	var groups [][]*Message
	var current []*Message
	var item *Message

	for _, item = range history {
		if item.Role == "user" && len(current) > 0 {
			groups = append(groups, current)
			current = nil
		}

		current = append(current, item)
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}

	return groups
}

func summaryCompletion(ctx context.Context, agent *Agent, prov *Provider, transcript string) (string, error) {
	var client openai.Client
	var params openai.ChatCompletionNewParams
	var response *openai.ChatCompletion
	var content string
	var runes []rune
	var limit int

	var err error

	params.Model = agent.Model
	params.Messages = []openai.ChatCompletionMessageParamUnion{openai.UserMessage(transcript)}
	err = contextLimit(agent, params.Messages, nil)
	if err != nil {
		return "", err
	}

	client = chatClient(prov)
	response, err = client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("provider returned no completion choices for the conversation summary")
	}

	content = strings.TrimSpace(response.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("provider returned an empty conversation summary")
	}

	limit = int(contextInputLimit(agent) / 4)
	if limit < 1 {
		limit = 1
	}
	if limit > maxSummaryChars {
		limit = maxSummaryChars
	}
	runes = []rune(content)
	if len(runes) > limit {
		content = string(runes[:limit])
	}

	return content, nil
}

func compactHistory(ctx context.Context, agent *Agent, session *Session, prov *Provider, summary *Summary, history []*Message, pending *Message) (*Summary, error) {
	var dropped []*Message
	var transcript string
	var content string
	var updated Summary
	var groups [][]*Message
	var group []*Message

	var err error

	dropped = compactPrefix(history, pending)
	if len(dropped) == 0 {
		return nil, &ContextLengthError{Limit: contextInputLimit(agent), Tokens: contextInputLimit(agent) + 1}
	}

	groups = summaryGroups(dropped)
	for _, group = range groups {
		transcript, err = summaryTranscript(content, group)
		if err != nil {
			return nil, err
		}
		if content == "" && summary != nil {
			transcript, err = summaryTranscript(summary.Content, group)
			if err != nil {
				return nil, err
			}
		}

		content, err = summaryCompletion(ctx, agent, prov, transcript)
		if err != nil {
			return nil, err
		}
	}

	updated = Summary{SessionId: session.Id, Content: content, ThroughMessageId: dropped[len(dropped)-1].Id}
	err = SummarySave(&updated)
	if err != nil {
		return nil, err
	}

	return &updated, nil
}
