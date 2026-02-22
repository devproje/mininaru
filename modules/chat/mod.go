// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * MiniNaru
 * Copyright (C) 2022-2026 Project_IO
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package chat

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"git.wh64.net/naru-studio/mininaru/modules/agent"
	"git.wh64.net/naru-studio/mininaru/modules/database"
	"github.com/google/uuid"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
)

type ChatModule struct {
	DB    *sql.DB
	Agent *agent.AgentModule
}

type ChatPayload struct {
	AgentId   string  `json:"agent_id"`
	ChannelId *string `json:"channel_id"`
	Message   string  `json:"message"`
}

type ChatMessage struct {
	Id        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ChatResponse struct {
	ChannelId string       `json:"channel_id"`
	Output    *ChatMessage `json:"output"`
}

func (c *ChatModule) Name() string {
	return "chat-module"
}

func (c *ChatModule) Load() error {
	var err error

	if database.Database == nil {
		err = fmt.Errorf("database module not loaded")
		return err
	}

	if agent.Agent == nil {
		err = fmt.Errorf("agent module not loaded")
		return err
	}

	c.DB = database.Database.DB
	c.Agent = agent.Agent

	return nil
}

func (c *ChatModule) Unload() error {
	if c.Agent != nil {
		c.Agent = nil
	}

	if c.DB != nil {
		c.DB = nil
	}

	return nil
}

func (c *ChatModule) CreateChat(channelId string, payload *ChatMessage) (*string, error) {
	var err error
	var id string

	id = uuid.NewString()
	_, err = c.DB.Exec("INSERT INTO chats (id, channel_id, `role`, content) VALUES (?, ?, ?, ?);", id, channelId, payload.Role, payload.Content)
	if err != nil {
		return nil, err
	}

	return &id, nil
}

func (c *ChatModule) ReadChats(channelId string) ([]ChatMessage, error) {
	var err error
	var rows *sql.Rows

	var messages []ChatMessage = make([]ChatMessage, 0)
	var cur ChatMessage

	rows, err = c.DB.Query("SELECT id, `role`, content, created_at, updated_at FROM chats WHERE channel_id = ?;", channelId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&cur.Id, &cur.Role, &cur.Content, &cur.CreatedAt, &cur.UpdatedAt)
		if err != nil {
			return nil, err
		}

		messages = append(messages, cur)
	}

	return messages, nil
}

func (c *ChatModule) Send(payload *ChatPayload) (*ChatResponse, error) {
	var err error
	var cid, mid *string
	var response ChatResponse

	var client openai.Client
	var agent *agent.AgentData
	var resp *openai.ChatCompletion

	var messages []ChatMessage
	var parsed []openai.ChatCompletionMessageParamUnion

	agent, err = c.Agent.Read(payload.AgentId)
	if err != nil {
		return nil, err
	}

	if agent.Engine == nil {
		err = fmt.Errorf("engine not defined from agent '%s'", agent.Id)
		return nil, err
	}

	if payload.ChannelId == nil {
		cid, err = c.CreateChannel(&ChatChannel{
			Name:    "Untitled",
			AgentId: agent.Id,
		})
		if err != nil {
			return nil, err
		}
	} else {
		cid = payload.ChannelId
	}

	messages, err = c.ReadChats(*cid)
	if err != nil {
		return nil, err
	}

	for _, m := range messages {
		if m.Role == "assistant" {
			parsed = append(parsed, openai.ChatCompletionMessageParamUnion{
				OfAssistant: &openai.ChatCompletionAssistantMessageParam{
					Content: openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: openai.String(m.Content),
					},
				},
			})
			continue
		}

		parsed = append(parsed, openai.ChatCompletionMessageParamUnion{
			OfUser: &openai.ChatCompletionUserMessageParam{
				Content: openai.ChatCompletionUserMessageParamContentUnion{
					OfString: openai.String(m.Content),
				},
			},
		})
	}

	parsed = append(parsed, openai.ChatCompletionMessageParamUnion{
		OfUser: &openai.ChatCompletionUserMessageParam{
			Content: openai.ChatCompletionUserMessageParamContentUnion{
				OfString: openai.String(payload.Message),
			},
		},
	})

	_, err = c.CreateChat(*cid, &ChatMessage{Role: "user", Content: payload.Message})
	if err != nil {
		return nil, err
	}

	client = openai.NewClient(option.WithBaseURL(agent.Engine.ApiEndpoint), option.WithAPIKey(agent.Engine.ApiKey))
	resp, err = client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Model:    agent.Engine.Model,
		Messages: parsed,
		Store: param.Opt[bool]{
			Value: false,
		},
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		response = ChatResponse{
			ChannelId: *cid,
			Output:    nil,
		}

		return &response, nil
	}

	mid, err = c.CreateChat(*cid, &ChatMessage{Role: "assistant", Content: resp.Choices[0].Message.Content})
	if err != nil {
		return nil, err
	}

	response = ChatResponse{
		ChannelId: *cid,
		Output: &ChatMessage{
			Id:        *mid,
			Role:      "assistant",
			Content:   resp.Choices[0].Message.Content,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	return &response, nil
}

func (c *ChatModule) SendStream() error {
	return nil
}

var Chat *ChatModule = &ChatModule{}
