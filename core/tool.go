// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/util"
	"github.com/google/uuid"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
)

type ToolCall struct {
	Id        string `json:"id"`
	CallId    string `json:"call_id"`
	MessageId string `json:"message_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result"`
	Status    string `json:"status"`
	Error     string `json:"error"`
}

type ToolApprovalFunc func(ctx context.Context, def modules.Def, arguments string) (bool, error)

type ToolEvent struct {
	Phase     string `json:"phase"`
	CallId    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result"`
	Status    string `json:"status"`
	Error     string `json:"error"`
}

type ToolEventFunc func(event ToolEvent)

const maxToolRounds = 8

const (
	ToolEventStarted  = "started"
	ToolEventFinished = "finished"
)

func permittedTools(defs []modules.Def) []modules.Def {
	var def modules.Def
	var permitted []modules.Def

	for _, def = range defs {
		if def.Execute == nil || def.Name == "" {
			continue
		}
		permitted = append(permitted, def)
	}

	return permitted
}

func toolParams(defs []modules.Def) []openai.ChatCompletionToolParam {
	var def modules.Def
	var tools []openai.ChatCompletionToolParam

	for _, def = range defs {
		tools = append(tools, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        def.Name,
				Description: param.NewOpt(def.Description),
				Parameters:  shared.FunctionParameters(def.Parameters),
			},
		})
	}

	return tools
}

func findTool(defs []modules.Def, name string) *modules.Def {
	var index int

	for index = range defs {
		if defs[index].Name == name {
			return &defs[index]
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
		CallId: call.ID, MessageId: messageId, Name: call.Function.Name,
		Arguments: call.Function.Arguments, Status: MessagePending,
	}
	if messageId == "" {
		return &record, nil
	}

	record.Id = uuid.NewString()
	_, err = util.DB.Exec(`INSERT INTO tool_calls (id, call_id, message_id, name, arguments, result, status, error)
		VALUES (?, ?, ?, ?, ?, '', ?, '');`, record.Id, record.CallId, record.MessageId, record.Name, record.Arguments, record.Status)
	if err != nil {
		return nil, fmt.Errorf("recording tool call %s failed: %w", call.ID, err)
	}

	return &record, nil
}

func executeTool(ctx context.Context, record *ToolCall, defs []modules.Def, allowDangerous, allowPrivileged bool, approve ToolApprovalFunc) (*ToolCall, error) {
	var def *modules.Def
	var approved bool

	var err error

	if record == nil {
		return nil, fmt.Errorf("tool call record is required")
	}

	def = findTool(defs, record.Name)
	record.Status = MessageCompleted
	if def == nil {
		err = fmt.Errorf("unknown tool %q", record.Name)
	} else if def.Permission == modules.PermissionPrivileged && !allowPrivileged {
		err = fmt.Errorf("privileged tool %q is only available to an interactive front end", def.Name)
	} else if def.Permission == modules.PermissionDangerous && !allowDangerous {
		if approve == nil {
			err = fmt.Errorf("dangerous tool %q requires user approval", def.Name)
		} else {
			approved, err = approve(ctx, *def, record.Arguments)
			if err == nil && !approved {
				err = fmt.Errorf("user denied dangerous tool %q", def.Name)
			}
		}
	}
	if err == nil && def != nil {
		record.Result, err = def.Execute(ctx, record.Arguments)
	}
	if err != nil {
		record.Status = MessageFailed
		record.Error = err.Error()
		record.Result = "error: " + record.Error
	}

	if record.Id == "" {
		return record, nil
	}

	_, err = util.DB.Exec(`UPDATE tool_calls SET result = ?, status = ?, error = ? WHERE id = ?;`,
		record.Result, record.Status, record.Error, record.Id)
	if err != nil {
		return nil, fmt.Errorf("updating tool call %s failed: %w", record.CallId, err)
	}

	return record, nil
}

func toolCallsBySession(sessionId string) (map[string][]*ToolCall, error) {
	var query string
	var rows *sql.Rows
	var calls map[string][]*ToolCall
	var call ToolCall

	var err error

	query = `SELECT t.id, t.call_id, t.message_id, t.name, t.arguments, t.result, t.status, t.error
		FROM tool_calls t JOIN messages m ON m.id = t.message_id
		WHERE m.session_id = ? ORDER BY t.rowid ASC;`

	rows, err = util.DB.Query(query, sessionId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	calls = make(map[string][]*ToolCall)

	for rows.Next() {
		err = rows.Scan(&call.Id, &call.CallId, &call.MessageId, &call.Name, &call.Arguments, &call.Result, &call.Status, &call.Error)
		if err != nil {
			return nil, err
		}

		calls[call.MessageId] = append(calls[call.MessageId], &ToolCall{
			Id: call.Id, CallId: call.CallId, MessageId: call.MessageId, Name: call.Name, Arguments: call.Arguments,
			Result: call.Result, Status: call.Status, Error: call.Error,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return calls, nil
}

func replayableCalls(calls []*ToolCall) bool {
	var call *ToolCall

	if len(calls) == 0 {
		return false
	}

	for _, call = range calls {
		if call.CallId == "" || call.Status == MessagePending {
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

func historyMessages(history []*Message, calls map[string][]*ToolCall) []openai.ChatCompletionMessageParamUnion {
	var cur *Message
	var messages []openai.ChatCompletionMessageParamUnion
	var turn []*ToolCall
	var call *ToolCall

	for _, cur = range history {
		if cur.Role == "assistant" {
			messages = append(messages, openai.AssistantMessage(cur.Content))
			continue
		}

		messages = append(messages, openai.UserMessage(cur.Content))

		turn = calls[cur.Id]
		if !replayableCalls(turn) {
			continue
		}

		messages = append(messages, storedToolCallMessage(turn))

		for _, call = range turn {
			messages = append(messages, openai.ToolMessage(call.Result, call.CallId))
		}
	}

	return messages
}

func ToolCallList(messageId string) ([]*ToolCall, error) {
	var rows *sql.Rows
	var call ToolCall
	var calls []*ToolCall

	var err error

	rows, err = util.DB.Query(`SELECT id, call_id, message_id, name, arguments, result, status, error
		FROM tool_calls WHERE message_id = ? ORDER BY rowid ASC;`, messageId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&call.Id, &call.CallId, &call.MessageId, &call.Name, &call.Arguments, &call.Result, &call.Status, &call.Error)
		if err != nil {
			return nil, err
		}
		calls = append(calls, &ToolCall{
			Id: call.Id, CallId: call.CallId, MessageId: call.MessageId, Name: call.Name, Arguments: call.Arguments,
			Result: call.Result, Status: call.Status, Error: call.Error,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return calls, nil
}
