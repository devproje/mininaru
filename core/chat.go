package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/util"
	"github.com/google/uuid"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/respjson"
)

type Message struct {
	Id        string `json:"id"`
	SessionId string `json:"session_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Reasoning string `json:"reasoning"`
	Status    string `json:"status"`
	Error     string `json:"error"`
}

const (
	MessagePending   = "pending"
	MessageCompleted = "completed"
	MessageFailed    = "failed"
	MessageCancelled = "cancelled"
)

func MessageList(sessionId string) ([]*Message, error) {
	var rows *sql.Rows
	var cur Message
	var messages []*Message

	var err error

	rows, err = util.DB.Query("SELECT id, session_id, role, content, reasoning, status, error FROM messages WHERE session_id = ? AND status = ? ORDER BY rowid ASC;", sessionId, MessageCompleted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&cur.Id, &cur.SessionId, &cur.Role, &cur.Content, &cur.Reasoning, &cur.Status, &cur.Error)
		if err != nil {
			return nil, err
		}

		messages = append(messages, &Message{
			Id:        cur.Id,
			SessionId: cur.SessionId,
			Role:      cur.Role,
			Content:   cur.Content,
			Reasoning: cur.Reasoning,
			Status:    cur.Status,
			Error:     cur.Error,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}

func MessageSave(sessionId, role, content, reasoning string) (*Message, error) {
	var message Message

	var err error

	message = Message{
		Id:        uuid.NewString(),
		SessionId: sessionId,
		Role:      role,
		Content:   content,
		Reasoning: reasoning,
		Status:    MessageCompleted,
	}

	_, err = util.DB.Exec("INSERT INTO messages (id, session_id, role, content, reasoning, status, error) VALUES (?, ?, ?, ?, ?, ?, ?);",
		message.Id, message.SessionId, message.Role, message.Content, message.Reasoning, message.Status, message.Error)
	if err != nil {
		return nil, err
	}

	return &message, nil
}

func messageStart(sessionId, content string) (*Message, error) {
	var message Message

	var err error

	message = Message{Id: uuid.NewString(), SessionId: sessionId, Role: "user", Content: content, Status: MessagePending}
	_, err = util.DB.Exec("INSERT INTO messages (id, session_id, role, content, reasoning, status, error) VALUES (?, ?, ?, ?, '', ?, '');",
		message.Id, message.SessionId, message.Role, message.Content, message.Status)
	if err != nil {
		return nil, err
	}

	return &message, nil
}

func messageFail(id, status string, chatErr error) error {
	var text string

	var err error

	if chatErr != nil {
		text = chatErr.Error()
	}

	_, err = util.DB.Exec("UPDATE messages SET status = ?, error = ? WHERE id = ?;", status, text, id)
	return err
}

func messageCompleteTurn(userId, sessionId, assistantContent, reasoning string) (*Message, error) {
	var assistant Message
	var tx *sql.Tx

	var err error

	assistant = Message{Id: uuid.NewString(), SessionId: sessionId, Role: "assistant", Content: assistantContent, Reasoning: reasoning, Status: MessageCompleted}

	tx, err = util.DB.Begin()
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec("UPDATE messages SET status = ?, error = '' WHERE id = ? AND status = ?;", MessageCompleted, userId, MessagePending)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	_, err = tx.Exec("INSERT INTO messages (id, session_id, role, content, reasoning, status, error) VALUES (?, ?, ?, ?, ?, ?, '');",
		assistant.Id, assistant.SessionId, assistant.Role, assistant.Content, assistant.Reasoning, assistant.Status)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &assistant, nil
}

func toolCost(calls []*ToolCall) int {
	var call *ToolCall
	var cost int

	for _, call = range calls {
		cost += len(call.Name) + len(call.Arguments) + len(call.Result)
	}

	return cost
}

func trimHistory(history []*Message, calls map[string][]*ToolCall, maxChars, reserved int) []*Message {
	var used int
	var start int
	var index, turnStart, turnCost, cursor int

	if maxChars <= 0 {
		return history
	}

	used = reserved
	start = len(history)
	index = len(history) - 1
	for index >= 0 {
		turnStart = index
		if history[index].Role == "assistant" && index > 0 && history[index-1].Role == "user" {
			turnStart = index - 1
		}

		turnCost = 0
		for cursor = turnStart; cursor <= index; cursor++ {
			turnCost += len(history[cursor].Content) + toolCost(calls[history[cursor].Id])
		}
		if used+turnCost > maxChars {
			break
		}

		used += turnCost
		start = turnStart
		index = turnStart - 1
	}

	return history[start:]
}

func deltaReasoning(delta openai.ChatCompletionChunkChoiceDelta) string {
	var field respjson.Field
	var ok bool
	var text string

	var err error

	field, ok = delta.JSON.ExtraFields["reasoning_content"]
	if !ok {
		field, ok = delta.JSON.ExtraFields["reasoning"]
	}

	if !ok {
		return ""
	}

	if field.Raw() == "" || field.Raw() == respjson.Null {
		return ""
	}

	err = json.Unmarshal([]byte(field.Raw()), &text)
	if err != nil {
		return ""
	}

	return text
}

func Chat(ctx context.Context, session *Session, agent *NaruAgent, content string, onContent, onReasoning func(string)) (*Message, error) {
	var defs []modules.Def

	if config.Client.Tools.Enabled {
		defs = modules.DefaultTools()
	}

	return chatWithTools(ctx, session, agent, content, defs, onContent, onReasoning, nil, nil)
}

func ChatWithApproval(ctx context.Context, session *Session, agent *NaruAgent, content string, onContent, onReasoning func(string), onTool ToolEventFunc, approve ToolApprovalFunc) (*Message, error) {
	var defs []modules.Def

	if config.Client.Tools.Enabled {
		defs = modules.DefaultTools()
	}

	return chatWithTools(ctx, session, agent, content, defs, onContent, onReasoning, onTool, approve)
}

func ChatWithTools(ctx context.Context, session *Session, agent *NaruAgent, content string, defs []modules.Def, onContent, onReasoning func(string)) (*Message, error) {
	return chatWithTools(ctx, session, agent, content, defs, onContent, onReasoning, nil, nil)
}

func chatWithTools(ctx context.Context, session *Session, agent *NaruAgent, content string, defs []modules.Def, onContent, onReasoning func(string), onTool ToolEventFunc, approve ToolApprovalFunc) (*Message, error) {
	return chatWithToolPolicy(ctx, session, agent, content, nil, defs, onContent, onReasoning, onTool, approve, config.AllowDangerousTools)
}

func chatWithToolPolicy(ctx context.Context, session *Session, agent *NaruAgent, content string, parts []openai.ChatCompletionContentPartUnionParam,
	defs []modules.Def, onContent, onReasoning func(string), onTool ToolEventFunc, approve ToolApprovalFunc, allowDangerous bool) (*Message, error) {
	var history []*Message
	var calls map[string][]*ToolCall
	var prompt string
	var messages []openai.ChatCompletionMessageParamUnion
	var params openai.ChatCompletionNewParams
	var pending *Message
	var run completionRun
	var result *Completion
	var status string
	var saveErr error

	var err error

	if session == nil || agent == nil {
		return nil, fmt.Errorf("session and agent are required to chat")
	}
	if agent.AI == nil {
		return nil, fmt.Errorf("agent %s has no available provider client", agent.Id)
	}

	history, err = MessageList(session.Id)
	if err != nil {
		return nil, err
	}

	defs = permittedTools(defs)

	if len(defs) > 0 {
		calls, err = toolCallsBySession(session.Id)
		if err != nil {
			return nil, err
		}
	}

	prompt = systemPrompt(agent, defs)

	messages = append(messages, openai.SystemMessage(prompt))
	history = trimHistory(history, calls, config.Client.Context.MaxChars, len(prompt)+len(content))

	messages = append(messages, historyMessages(history, calls)...)
	if len(parts) > 0 {
		messages = append(messages, openai.UserMessage(parts))
	} else {
		messages = append(messages, openai.UserMessage(content))
	}

	params = openai.ChatCompletionNewParams{
		Model:    agent.Model,
		Messages: messages,
	}
	if len(defs) > 0 {
		params.Tools = toolParams(defs)
	}

	if config.ThinkingEnabled() {
		params.ReasoningEffort = openai.ReasoningEffort(config.Client.Thinking.Level)
	}

	pending, err = messageStart(session.Id, content)
	if err != nil {
		return nil, err
	}

	run = completionRun{
		AI: agent.AI, Params: params, Defs: defs, AllowDangerous: allowDangerous, AllowPrivileged: true, MessageId: pending.Id,
		OnContent: onContent, OnReasoning: onReasoning, OnTool: onTool, Approve: approve,
	}

	result, err = run.execute(ctx)
	if err != nil {
		status = MessageFailed
		if ctx.Err() != nil {
			status = MessageCancelled
		}

		saveErr = messageFail(pending.Id, status, err)
		if saveErr != nil {
			return nil, fmt.Errorf("chat failed: %v; recording failure also failed: %w", err, saveErr)
		}

		return nil, err
	}

	return messageCompleteTurn(pending.Id, session.Id, result.Content, result.Reasoning)
}
