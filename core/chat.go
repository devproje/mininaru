// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/shared"
)

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
	var params openai.ChatCompletionNewParams
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

	params.Model = agent.Model
	params.Messages = union

	switch ThinkingLevel(agent.ThinkingLevel) {
	case Low:
		params.ReasoningEffort = shared.ReasoningEffortLow
	case Medium:
		params.ReasoningEffort = shared.ReasoningEffortMedium
	case High, Max:
		params.ReasoningEffort = shared.ReasoningEffortHigh
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

func SendChatMessage(ctx context.Context, agent *Agent, session *Session, onChunk func(openai.ChatCompletionChunk)) error {
	var history []*Message
	var messages []ChatMessage
	var item *Message
	var pending *Message
	var builder strings.Builder
	var assistant Message
	var updateErr error

	var err error

	history, err = MessageList(session.Id)
	if err != nil {
		return err
	}

	if agent.Soul != "" {
		messages = append(messages, ChatMessage{Role: "system", Content: agent.Soul})
	}

	for _, item = range history {
		messages = append(messages, ChatMessage{Role: item.Role, Content: item.Content})

		if item.Role == "user" && item.Status == "pending" {
			pending = item
		}
	}

	if pending == nil {
		err = fmt.Errorf("session %s has no pending user message", session.Id)
		return err
	}

	err = ChatCompletionStream(ctx, agent, messages, func(chunk openai.ChatCompletionChunk) error {
		if len(chunk.Choices) > 0 {
			builder.WriteString(chunk.Choices[0].Delta.Content)
		}

		onChunk(chunk)

		return nil
	})
	if err != nil {
		updateErr = MessageUpdate(pending.Id, &Message{Status: "failed", Error: err.Error()})
		if updateErr != nil {
			return updateErr
		}

		return err
	}

	err = MessageUpdate(pending.Id, &Message{Status: "completed"})
	if err != nil {
		return err
	}

	assistant = Message{Id: uuid.NewString(), SessionId: session.Id, Role: "assistant", Content: builder.String(), Status: "completed"}

	return MessageCreate(&assistant)
}
