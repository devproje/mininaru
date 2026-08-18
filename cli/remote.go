// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/devproje/mininaru/cli/tui"
	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules"
	mininarurpc "github.com/devproje/mininaru/rpc"
	mininaruv1 "github.com/devproje/mininaru/rpc/gen/mininaru/v1"
	"google.golang.org/grpc"
)

type remoteBackend struct {
	client    mininaruv1.MininaruServiceClient
	toolCalls map[string][]*core.ToolCall
}

func activeServerAddress() string {
	if serverRef != "" {
		return serverRef
	}

	return config.Client.Server.Address
}

func remoteConnect(ctx context.Context) (*grpc.ClientConn, mininaruv1.MininaruServiceClient, error) {
	var connection *grpc.ClientConn

	var err error

	connection, err = mininarurpc.Dial(ctx, activeServerAddress())
	if err != nil {
		return nil, nil, err
	}

	return connection, mininaruv1.NewMininaruServiceClient(connection), nil
}

func coreMessage(message *mininaruv1.Message) *core.Message {
	if message == nil {
		return nil
	}

	return &core.Message{Id: message.GetId(), SessionId: message.GetSessionId(), Role: message.GetRole(),
		Content: message.GetContent(), Reasoning: message.GetReasoning(), Status: message.GetStatus(), Error: message.GetError()}
}

func coreSession(session *mininaruv1.Session) *core.Session {
	if session == nil {
		return nil
	}

	return &core.Session{Id: session.GetId(), AgentId: session.GetAgentId(), Name: session.GetName()}
}

func coreAgent(agent *mininaruv1.Agent) *core.NaruAgent {
	if agent == nil {
		return nil
	}

	return &core.NaruAgent{Id: agent.GetId(), Name: agent.GetName(), Model: agent.GetModel()}
}

func coreUsage(usage *mininaruv1.Usage) *core.UsageTotals {
	var totals core.UsageTotals
	var line *mininaruv1.UsageLine

	if usage == nil {
		return &totals
	}

	totals = core.UsageTotals{SessionId: usage.GetSessionId(), PromptTokens: usage.GetPromptTokens(),
		CompletionTokens: usage.GetCompletionTokens(), TotalTokens: usage.GetTotalTokens(),
		CachedTokens: usage.GetCachedTokens(), CacheWriteTokens: usage.GetCacheWriteTokens()}
	for _, line = range usage.GetLines() {
		totals.Lines = append(totals.Lines, core.UsageLine{Kind: line.GetKind(), PromptTokens: line.GetPromptTokens(),
			CompletionTokens: line.GetCompletionTokens(), TotalTokens: line.GetTotalTokens(),
			CachedTokens: line.GetCachedTokens(), CacheWriteTokens: line.GetCacheWriteTokens()})
	}

	return &totals
}

func coreToolCall(call *mininaruv1.ToolCall) *core.ToolCall {
	if call == nil {
		return nil
	}

	return &core.ToolCall{Id: call.GetId(), CallId: call.GetCallId(), MessageId: call.GetMessageId(), Name: call.GetName(),
		Arguments: call.GetArguments(), Result: call.GetResult(), Status: call.GetStatus(), Error: call.GetError()}
}

func remoteTool(event *mininaruv1.ToolEvent) core.ToolEvent {
	if event == nil {
		return core.ToolEvent{}
	}

	return core.ToolEvent{Phase: event.GetPhase(), CallId: event.GetCallId(), Name: event.GetName(),
		Arguments: event.GetArguments(), Result: event.GetResult(), Status: event.GetStatus(), Error: event.GetError()}
}

func remoteApproval(ctx context.Context, stream mininaruv1.MininaruService_ChatClient,
	request *mininaruv1.ApprovalRequest, approve core.ToolApprovalFunc) error {
	var allowed bool
	var choice mininaruv1.ApprovalChoice

	var err error

	if approve != nil {
		allowed, err = approve(ctx, modules.Def{Name: request.GetToolName()}, request.GetArguments())
		if err != nil {
			return err
		}
	}

	choice = mininaruv1.ApprovalChoice_APPROVAL_CHOICE_DENY
	if allowed {
		choice = mininaruv1.ApprovalChoice_APPROVAL_CHOICE_ONCE
	}

	return stream.Send(&mininaruv1.ChatClientEvent{Event: &mininaruv1.ChatClientEvent_Approval{Approval: &mininaruv1.ApprovalDecision{
		RequestId: request.GetRequestId(), Choice: choice}}})
}

func (r *remoteBackend) Chat(ctx context.Context, session *core.Session, agent *core.NaruAgent, content string,
	onContent, onReasoning func(string), onTool core.ToolEventFunc, approve core.ToolApprovalFunc) (*core.Message, error) {
	var stream mininaruv1.MininaruService_ChatClient
	var event *mininaruv1.ChatServerEvent
	var failed *mininaruv1.ChatFailed

	var err error

	stream, err = r.client.Chat(ctx)
	if err != nil {
		return nil, err
	}

	err = stream.Send(&mininaruv1.ChatClientEvent{Event: &mininaruv1.ChatClientEvent_Start{Start: &mininaruv1.ChatStart{
		SessionId: session.Id, Content: content, Thinking: config.Client.Thinking.Level}}})
	if err != nil {
		return nil, err
	}

	for {
		event, err = stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("grpc chat ended without a result")
			}
			return nil, err
		}
		if event.GetContent() != nil && onContent != nil {
			onContent(event.GetContent().GetText())
		}
		if event.GetReasoning() != nil && onReasoning != nil {
			onReasoning(event.GetReasoning().GetText())
		}
		if event.GetTool() != nil && onTool != nil {
			onTool(remoteTool(event.GetTool()))
		}
		if event.GetApproval() != nil {
			err = remoteApproval(ctx, stream, event.GetApproval(), approve)
			if err != nil {
				return nil, err
			}
		}
		if event.GetCompleted() != nil {
			return coreMessage(event.GetCompleted().GetMessage()), nil
		}
		failed = event.GetFailed()
		if failed != nil {
			return nil, fmt.Errorf("%s: %s", failed.GetCode(), failed.GetMessage())
		}
	}
}

func (r *remoteBackend) Compact(ctx context.Context, agent *core.NaruAgent, session *core.Session) (bool, error) {
	var response *mininaruv1.CompactSessionResponse

	var err error

	response, err = r.client.CompactSession(ctx, &mininaruv1.CompactSessionRequest{SessionId: session.Id})
	if err != nil {
		return false, err
	}

	return response.GetCompacted(), nil
}

func (r *remoteBackend) Usage(sessionId string) (*core.UsageTotals, error) {
	var usage *mininaruv1.Usage

	var err error

	usage, err = r.client.GetUsage(context.Background(), &mininaruv1.GetUsageRequest{SessionId: sessionId})
	if err != nil {
		return nil, err
	}

	return coreUsage(usage), nil
}

func (r *remoteBackend) Context(sessionId string) (int64, int64, bool, error) {
	var detail *mininaruv1.SessionDetail

	var err error

	detail, err = r.client.GetSession(context.Background(), &mininaruv1.GetSessionRequest{SessionId: sessionId})
	if err != nil {
		return 0, 0, false, err
	}

	return detail.GetContextTokens(), detail.GetContextWindow(), detail.GetContextKnown(), nil
}

func (r *remoteBackend) ToolCalls(messageId string) ([]*core.ToolCall, error) {
	return append([]*core.ToolCall(nil), r.toolCalls[messageId]...), nil
}

func remoteAgent(response *mininaruv1.ListAgentsResponse) (*mininaruv1.Agent, error) {
	var agent *mininaruv1.Agent
	var desired string

	desired = chatAgentRef
	if desired == "" {
		desired = response.GetDefaultAgentId()
	}

	for _, agent = range response.GetAgents() {
		if agent.GetId() == desired || agent.GetName() == desired {
			return agent, nil
		}
	}

	if desired == "" && len(response.GetAgents()) > 0 {
		return response.GetAgents()[0], nil
	}

	return nil, fmt.Errorf("agent %s not found on server", desired)
}

func remoteSession(ctx context.Context, client mininaruv1.MininaruServiceClient,
	agent *mininaruv1.Agent, args []string) (*mininaruv1.SessionDetail, error) {
	var id string
	var sessions *mininaruv1.ListSessionsResponse
	var created *mininaruv1.Session
	var detail *mininaruv1.SessionDetail

	var err error

	id = sessionIdRef
	if id == "" {
		id = resumeRef
	}
	if id == latestSession && len(args) == 1 {
		id = args[0]
	}

	if id == latestSession {
		sessions, err = client.ListSessions(ctx, &mininaruv1.ListSessionsRequest{Agent: agent.GetName()})
		if err != nil {
			return nil, err
		}
		if len(sessions.GetSessions()) > 0 {
			id = sessions.GetSessions()[len(sessions.GetSessions())-1].GetId()
		} else {
			id = ""
		}
	}

	if id == "" {
		created, err = client.CreateSession(ctx, &mininaruv1.CreateSessionRequest{Agent: agent.GetName(),
			Name: time.Now().Format("2006-01-02 15:04")})
		if err != nil {
			return nil, err
		}
		id = created.GetId()
	}

	detail, err = client.GetSession(ctx, &mininaruv1.GetSessionRequest{SessionId: id})
	if err != nil {
		return nil, err
	}
	if detail.GetAgent().GetId() != agent.GetId() {
		return nil, fmt.Errorf("session %s belongs to agent %s, not %s", id, detail.GetAgent().GetName(), agent.GetName())
	}

	return detail, nil
}

func runRemotePrompt(ctx context.Context, backend *remoteBackend, session *core.Session, agent *core.NaruAgent, content string) error {
	var message *core.Message
	var waiting *progress

	var err error

	waiting = progressStart(ctx, "thinking")
	message, err = backend.Chat(ctx, session, agent, content, nil, func(delta string) {
		waiting.stop()
		if config.Client.Thinking.Show {
			fmt.Fprint(os.Stderr, delta)
		}
	}, func(event core.ToolEvent) {
		waiting.stop()
		promptToolLog(os.Stderr, event)
	}, nil)
	waiting.stop()
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, message.Content)

	return nil
}

func executeRemote(cmdCtx context.Context, args []string, content string) error {
	var connection *grpc.ClientConn
	var client mininaruv1.MininaruServiceClient
	var listed *mininaruv1.ListAgentsResponse
	var selected *mininaruv1.Agent
	var detail *mininaruv1.SessionDetail
	var session *core.Session
	var agent *core.NaruAgent
	var history []*core.Message
	var message *mininaruv1.Message
	var call *mininaruv1.ToolCall
	var backend remoteBackend

	var err error

	connection, err = mininarurpc.Dial(cmdCtx, serverRef)
	if err != nil {
		return err
	}
	defer connection.Close()

	client = mininaruv1.NewMininaruServiceClient(connection)
	listed, err = client.ListAgents(cmdCtx, &mininaruv1.ListAgentsRequest{})
	if err != nil {
		return err
	}
	selected, err = remoteAgent(listed)
	if err != nil {
		return err
	}
	detail, err = remoteSession(cmdCtx, client, selected, args)
	if err != nil {
		return err
	}

	session = coreSession(detail.GetSession())
	agent = coreAgent(detail.GetAgent())
	for _, message = range detail.GetMessages() {
		history = append(history, coreMessage(message))
	}
	backend.client = client
	backend.toolCalls = make(map[string][]*core.ToolCall)
	for _, call = range detail.GetToolCalls() {
		backend.toolCalls[call.GetMessageId()] = append(backend.toolCalls[call.GetMessageId()], coreToolCall(call))
	}

	if content != "" {
		return runRemotePrompt(cmdCtx, &backend, session, agent, content)
	}

	return tui.RunWithBackend(session, agent, history, updateNotice(), &backend)
}

func remoteSessionListExecute(ctx context.Context) error {
	var connection *grpc.ClientConn
	var client mininaruv1.MininaruServiceClient
	var sessions *mininaruv1.ListSessionsResponse
	var session *mininaruv1.Session
	var usage *mininaruv1.Usage
	var rows *uiRows

	var err error

	connection, client, err = remoteConnect(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()

	sessions, err = client.ListSessions(ctx, &mininaruv1.ListSessionsRequest{Agent: sessionAgentIdRef})
	if err != nil {
		return err
	}
	if len(sessions.GetSessions()) == 0 {
		uiEmpty("no sessions on the grpc server yet")
		return nil
	}

	rows = uiTable("ID", "TOKENS", "NAME")
	for _, session = range sessions.GetSessions() {
		usage, err = client.GetUsage(ctx, &mininaruv1.GetUsageRequest{SessionId: session.GetId()})
		if err != nil {
			return err
		}
		rows.row(session.GetId(), tokenCount(usage.GetTotalTokens()), session.GetName())
	}
	rows.flush()

	return nil
}

func remoteUsageSession(ctx context.Context, client mininaruv1.MininaruServiceClient, args []string) (string, error) {
	var sessions *mininaruv1.ListSessionsResponse

	var err error

	if len(args) == 1 {
		return args[0], nil
	}

	sessions, err = client.ListSessions(ctx, &mininaruv1.ListSessionsRequest{Agent: sessionAgentIdRef})
	if err != nil {
		return "", err
	}
	if len(sessions.GetSessions()) == 0 {
		return "", configErrorf("no sessions on the grpc server yet")
	}

	return sessions.GetSessions()[len(sessions.GetSessions())-1].GetId(), nil
}

func remoteSessionUsageExecute(ctx context.Context, args []string) error {
	var connection *grpc.ClientConn
	var client mininaruv1.MininaruServiceClient
	var sessionId string
	var usage *mininaruv1.Usage
	var line *mininaruv1.UsageLine
	var rows *uiRows

	var err error

	connection, client, err = remoteConnect(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()

	sessionId, err = remoteUsageSession(ctx, client, args)
	if err != nil {
		return err
	}
	usage, err = client.GetUsage(ctx, &mininaruv1.GetUsageRequest{SessionId: sessionId})
	if err != nil {
		return err
	}
	if usage.GetTotalTokens() == 0 {
		uiEmpty("no token usage recorded for %s yet", sessionId)
		return nil
	}

	rows = uiTable("KIND", "PROMPT", "CACHE READ", "CACHE WRITE", "COMPLETION", "TOTAL")
	for _, line = range usage.GetLines() {
		rows.row(line.GetKind(), tokenCount(line.GetPromptTokens()), tokenCount(line.GetCachedTokens()),
			tokenCount(line.GetCacheWriteTokens()), tokenCount(line.GetCompletionTokens()), tokenCount(line.GetTotalTokens()))
	}
	rows.row("total", tokenCount(usage.GetPromptTokens()), tokenCount(usage.GetCachedTokens()),
		tokenCount(usage.GetCacheWriteTokens()), tokenCount(usage.GetCompletionTokens()), tokenCount(usage.GetTotalTokens()))
	rows.flush()

	return nil
}

func remoteSessionRemoveExecute(ctx context.Context, sessionId string) error {
	var connection *grpc.ClientConn
	var client mininaruv1.MininaruServiceClient

	var err error

	connection, client, err = remoteConnect(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()

	_, err = client.DeleteSession(ctx, &mininaruv1.DeleteSessionRequest{SessionId: sessionId})

	return err
}

func remoteSessionRenameExecute(ctx context.Context, sessionId, name string) error {
	var connection *grpc.ClientConn
	var client mininaruv1.MininaruServiceClient

	var err error

	connection, client, err = remoteConnect(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()

	_, err = client.RenameSession(ctx, &mininaruv1.RenameSessionRequest{SessionId: sessionId, Name: name})

	return err
}
