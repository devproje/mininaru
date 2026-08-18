// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules"
	mininaruv1 "github.com/devproje/mininaru/rpc/gen/mininaru/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mininaruService struct {
	mininaruv1.UnimplementedMininaruServiceServer

	registry *core.Registry
	slots    chan struct{}
}

const defaultSessionNameLayout = "2006-01-02 15:04"

const maxConcurrentChats = 16

const maxChatContentBytes = 1 << 20

func rpcAgent(agent *core.NaruAgent) *mininaruv1.Agent {
	var provider *core.Provider
	var providerName string

	var err error

	if agent == nil {
		return nil
	}

	provider, err = core.ProviderFind(agent.ProviderId)
	if err == nil {
		providerName = provider.Name
	}

	return &mininaruv1.Agent{Id: agent.Id, Name: agent.Name, Model: agent.Model, Provider: providerName}
}

func rpcSkill(skill *modules.Skill, includeBody bool) *mininaruv1.Skill {
	var body string

	if skill == nil {
		return nil
	}
	if includeBody {
		body = skill.Body
	}

	return &mininaruv1.Skill{Name: skill.Name, Description: skill.Description, Scope: skill.Scope, Body: body}
}

func rpcSession(session *core.Session) *mininaruv1.Session {
	if session == nil {
		return nil
	}

	return &mininaruv1.Session{Id: session.Id, AgentId: session.AgentId, Name: session.Name}
}

func rpcMessage(message *core.Message) *mininaruv1.Message {
	if message == nil {
		return nil
	}

	return &mininaruv1.Message{Id: message.Id, SessionId: message.SessionId, Role: message.Role,
		Content: message.Content, Reasoning: message.Reasoning, Status: message.Status, Error: message.Error}
}

func rpcToolCall(call *core.ToolCall) *mininaruv1.ToolCall {
	if call == nil {
		return nil
	}

	return &mininaruv1.ToolCall{Id: call.Id, CallId: call.CallId, MessageId: call.MessageId, Name: call.Name,
		Arguments: call.Arguments, Result: call.Result, Status: call.Status, Error: call.Error}
}

func rpcUsage(totals *core.UsageTotals) *mininaruv1.Usage {
	var usage mininaruv1.Usage
	var line core.UsageLine

	if totals == nil {
		return &usage
	}

	usage = mininaruv1.Usage{SessionId: totals.SessionId, PromptTokens: totals.PromptTokens,
		CompletionTokens: totals.CompletionTokens, TotalTokens: totals.TotalTokens,
		CachedTokens: totals.CachedTokens, CacheWriteTokens: totals.CacheWriteTokens}
	for _, line = range totals.Lines {
		usage.Lines = append(usage.Lines, &mininaruv1.UsageLine{Kind: line.Kind, PromptTokens: line.PromptTokens,
			CompletionTokens: line.CompletionTokens, TotalTokens: line.TotalTokens,
			CachedTokens: line.CachedTokens, CacheWriteTokens: line.CacheWriteTokens})
	}

	return &usage
}

func sessionInstance(registry *core.Registry, sessionId string) (*core.Session, *core.Instance, error) {
	var session *core.Session
	var instance *core.Instance

	var err error

	if sessionId == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "session id is required")
	}

	session, err = core.SessionFind(sessionId)
	if err != nil {
		return nil, nil, status.Error(codes.NotFound, err.Error())
	}

	instance, err = registry.ByAgentId(session.AgentId)
	if err != nil {
		return nil, nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	return session, instance, nil
}

func (s *mininaruService) ListAgents(ctx context.Context, request *mininaruv1.ListAgentsRequest) (*mininaruv1.ListAgentsResponse, error) {
	var response mininaruv1.ListAgentsResponse
	var instance *core.Instance

	for _, instance = range s.registry.List() {
		response.Agents = append(response.Agents, rpcAgent(instance.Agent))
	}
	if core.Global != nil {
		response.DefaultAgentId = core.Global.Id
	}

	return &response, nil
}

func (s *mininaruService) ListSkills(ctx context.Context, request *mininaruv1.ListSkillsRequest) (*mininaruv1.ListSkillsResponse, error) {
	var response mininaruv1.ListSkillsResponse
	var skill modules.Skill

	for _, skill = range modules.SkillAll() {
		response.Skills = append(response.Skills, rpcSkill(&skill, false))
	}

	return &response, nil
}

func (s *mininaruService) GetSkill(ctx context.Context, request *mininaruv1.GetSkillRequest) (*mininaruv1.Skill, error) {
	var skill *modules.Skill
	var body string

	var err error

	skill = modules.SkillFind(request.GetName())
	if skill == nil {
		return nil, status.Error(codes.NotFound, "skill not found")
	}
	body, err = modules.SkillResult(skill.Name, "")
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	skill = &modules.Skill{Name: skill.Name, Description: skill.Description, Scope: skill.Scope, Body: body}

	return rpcSkill(skill, true), nil
}

func (s *mininaruService) ListSessions(ctx context.Context, request *mininaruv1.ListSessionsRequest) (*mininaruv1.ListSessionsResponse, error) {
	var instance *core.Instance
	var sessions []*core.Session
	var session *core.Session
	var response mininaruv1.ListSessionsResponse

	var err error

	if request.GetAgent() == "" {
		instance, err = s.registry.Default()
	} else {
		instance, err = s.registry.Get(request.GetAgent())
	}
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	sessions, err = core.SessionList(instance.Agent.Id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	for _, session = range sessions {
		response.Sessions = append(response.Sessions, rpcSession(session))
	}

	return &response, nil
}

func (s *mininaruService) CreateSession(ctx context.Context, request *mininaruv1.CreateSessionRequest) (*mininaruv1.Session, error) {
	var instance *core.Instance
	var name string
	var session *core.Session

	var err error

	if request.GetAgent() == "" {
		instance, err = s.registry.Default()
	} else {
		instance, err = s.registry.Get(request.GetAgent())
	}
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	name = request.GetName()
	if name == "" {
		name = time.Now().Format(defaultSessionNameLayout)
	}

	session, err = instance.Session(name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return rpcSession(session), nil
}

func (s *mininaruService) GetSession(ctx context.Context, request *mininaruv1.GetSessionRequest) (*mininaruv1.SessionDetail, error) {
	var session *core.Session
	var instance *core.Instance
	var messages []*core.Message
	var response mininaruv1.SessionDetail
	var message *core.Message
	var calls []*core.ToolCall
	var call *core.ToolCall
	var tokens int64
	var window int64
	var known bool

	var err error

	session, instance, err = sessionInstance(s.registry, request.GetSessionId())
	if err != nil {
		return nil, err
	}

	messages, err = core.MessageList(session.Id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	response.Session = rpcSession(session)
	response.Agent = rpcAgent(instance.Agent)
	for _, message = range messages {
		response.Messages = append(response.Messages, rpcMessage(message))
		calls, err = core.ToolCallList(message.Id)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		for _, call = range calls {
			response.ToolCalls = append(response.ToolCalls, rpcToolCall(call))
		}
	}

	tokens, window, known, err = core.SessionContextTokens(session.Id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	response.ContextTokens = tokens
	response.ContextWindow = window
	response.ContextKnown = known

	return &response, nil
}

func (s *mininaruService) RenameSession(ctx context.Context, request *mininaruv1.RenameSessionRequest) (*mininaruv1.Session, error) {
	var session *core.Session

	var err error

	if request.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "session name is required")
	}

	session, _, err = sessionInstance(s.registry, request.GetSessionId())
	if err != nil {
		return nil, err
	}

	err = core.SessionUpdate(session.Id, request.GetName())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	session.Name = request.GetName()

	return rpcSession(session), nil
}

func (s *mininaruService) DeleteSession(ctx context.Context, request *mininaruv1.DeleteSessionRequest) (*mininaruv1.Empty, error) {
	var session *core.Session

	var err error

	session, _, err = sessionInstance(s.registry, request.GetSessionId())
	if err != nil {
		return nil, err
	}

	err = core.SessionDelete(session.Id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &mininaruv1.Empty{}, nil
}

func (s *mininaruService) GetUsage(ctx context.Context, request *mininaruv1.GetUsageRequest) (*mininaruv1.Usage, error) {
	var session *core.Session
	var totals *core.UsageTotals

	var err error

	session, _, err = sessionInstance(s.registry, request.GetSessionId())
	if err != nil {
		return nil, err
	}

	totals, err = core.SessionUsage(session.Id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return rpcUsage(totals), nil
}

func (s *mininaruService) CompactSession(ctx context.Context, request *mininaruv1.CompactSessionRequest) (*mininaruv1.CompactSessionResponse, error) {
	var session *core.Session
	var instance *core.Instance
	var compacted bool

	var err error

	session, instance, err = sessionInstance(s.registry, request.GetSessionId())
	if err != nil {
		return nil, err
	}

	compacted, err = core.CompactNow(ctx, instance.Agent, session)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &mininaruv1.CompactSessionResponse{Compacted: compacted}, nil
}

func chatContentEvent(text string) *mininaruv1.ChatServerEvent {
	return &mininaruv1.ChatServerEvent{Event: &mininaruv1.ChatServerEvent_Content{Content: &mininaruv1.TextDelta{Text: text}}}
}

func chatReasoningEvent(text string) *mininaruv1.ChatServerEvent {
	return &mininaruv1.ChatServerEvent{Event: &mininaruv1.ChatServerEvent_Reasoning{Reasoning: &mininaruv1.TextDelta{Text: text}}}
}

func chatToolEvent(event core.ToolEvent) *mininaruv1.ChatServerEvent {
	return &mininaruv1.ChatServerEvent{Event: &mininaruv1.ChatServerEvent_Tool{Tool: &mininaruv1.ToolEvent{
		Phase: event.Phase, CallId: event.CallId, Name: event.Name, Arguments: event.Arguments,
		Result: event.Result, Status: event.Status, Error: event.Error}}}
}

func receiveChat(stream mininaruv1.MininaruService_ChatServer, incoming chan<- *mininaruv1.ChatClientEvent, cancel context.CancelFunc) {
	var event *mininaruv1.ChatClientEvent

	var err error

	for {
		event, err = stream.Recv()
		if err != nil {
			cancel()
			return
		}
		if event.GetCancel() != nil {
			cancel()
			return
		}

		select {
		case incoming <- event:
		case <-stream.Context().Done():
			return
		}
	}
}

func approvalChoice(choice mininaruv1.ApprovalChoice) (bool, bool) {
	if choice == mininaruv1.ApprovalChoice_APPROVAL_CHOICE_SESSION {
		return true, true
	}
	if choice == mininaruv1.ApprovalChoice_APPROVAL_CHOICE_ONCE {
		return true, false
	}

	return false, false
}

func chatApprover(ctx context.Context, stream mininaruv1.MininaruService_ChatServer,
	incoming <-chan *mininaruv1.ChatClientEvent) core.ToolApprovalFunc {
	var allowed map[string]bool
	var sendMu sync.Mutex

	allowed = make(map[string]bool)

	return func(approvalCtx context.Context, def modules.Def, arguments string) (bool, error) {
		var requestId string
		var event *mininaruv1.ChatClientEvent
		var decision *mininaruv1.ApprovalDecision
		var allow bool
		var remember bool

		var err error

		if allowed[def.Name] {
			return true, nil
		}

		requestId = uuid.NewString()
		sendMu.Lock()
		err = stream.Send(&mininaruv1.ChatServerEvent{Event: &mininaruv1.ChatServerEvent_Approval{Approval: &mininaruv1.ApprovalRequest{
			RequestId: requestId, ToolName: def.Name, Arguments: arguments}}})
		sendMu.Unlock()
		if err != nil {
			return false, err
		}

		for {
			select {
			case event = <-incoming:
				decision = event.GetApproval()
				if decision == nil || decision.GetRequestId() != requestId {
					continue
				}
				allow, remember = approvalChoice(decision.GetChoice())
				if remember {
					allowed[def.Name] = true
				}
				return allow, nil
			case <-approvalCtx.Done():
				return false, approvalCtx.Err()
			case <-ctx.Done():
				return false, ctx.Err()
			}
		}
	}
}

func remoteToolExecute(ctx context.Context, stream mininaruv1.MininaruService_ChatServer,
	incoming <-chan *mininaruv1.ChatClientEvent, sendMu *sync.Mutex, name string) func(context.Context, string) (string, error) {
	return func(callCtx context.Context, arguments string) (string, error) {
		var requestId string
		var event *mininaruv1.ChatClientEvent
		var result *mininaruv1.ToolResult

		var err error

		requestId = uuid.NewString()
		sendMu.Lock()
		err = stream.Send(&mininaruv1.ChatServerEvent{Event: &mininaruv1.ChatServerEvent_ToolRequest{ToolRequest: &mininaruv1.ToolRequest{
			RequestId: requestId, ToolName: name, Arguments: arguments}}})
		sendMu.Unlock()
		if err != nil {
			return "", err
		}
		for {
			select {
			case event = <-incoming:
				result = event.GetToolResult()
				if result == nil || result.GetRequestId() != requestId {
					continue
				}
				if result.GetError() != "" {
					return "", errors.New(result.GetError())
				}
				return result.GetResult(), nil
			case <-callCtx.Done():
				return "", callCtx.Err()
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
	}
}

func remoteToolDefs(ctx context.Context, stream mininaruv1.MininaruService_ChatServer,
	incoming <-chan *mininaruv1.ChatClientEvent, sendMu *sync.Mutex, advertised []*mininaruv1.ToolDefinition) ([]modules.Def, error) {
	var item *mininaruv1.ToolDefinition
	var parameters map[string]any
	var defs []modules.Def
	var def modules.Def

	var err error

	for _, item = range advertised {
		parameters = nil
		err = json.Unmarshal([]byte(item.GetParametersJson()), &parameters)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid tool parameters")
		}
		def = modules.Def{Name: item.GetName(), Description: item.GetDescription(), Parameters: parameters,
			Execute: remoteToolExecute(ctx, stream, incoming, sendMu, item.GetName())}
		defs = append(defs, def)
	}

	return defs, nil
}

func errorsIsContext(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func sendChatFailure(stream mininaruv1.MininaruService_ChatServer, err error) error {
	var code string

	code = "chat_failed"
	if errorsIsContext(err) {
		code = "cancelled"
	}

	return stream.Send(&mininaruv1.ChatServerEvent{Event: &mininaruv1.ChatServerEvent_Failed{Failed: &mininaruv1.ChatFailed{
		Code: code, Message: err.Error()}}})
}

func (s *mininaruService) Chat(stream mininaruv1.MininaruService_ChatServer) error {
	var first *mininaruv1.ChatClientEvent
	var start *mininaruv1.ChatStart
	var session *core.Session
	var instance *core.Instance
	var chatCtx context.Context
	var cancel context.CancelFunc
	var incoming chan *mininaruv1.ChatClientEvent
	var defs []modules.Def
	var message *core.Message
	var totals *core.UsageTotals
	var sendMu sync.Mutex
	var streamErr error

	var err error

	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		return status.Error(codes.ResourceExhausted, "too many concurrent chats")
	}

	first, err = stream.Recv()
	if err != nil {
		if err == io.EOF {
			return status.Error(codes.InvalidArgument, "chat start is required")
		}
		return err
	}
	start = first.GetStart()
	if start == nil || start.GetContent() == "" {
		return status.Error(codes.InvalidArgument, "first event must contain a non-empty chat start")
	}
	if len(start.GetContent()) > maxChatContentBytes {
		return status.Error(codes.ResourceExhausted, "chat content exceeds 1 MiB")
	}

	session, instance, err = sessionInstance(s.registry, start.GetSessionId())
	if err != nil {
		return err
	}

	chatCtx, cancel = context.WithCancel(stream.Context())
	defer cancel()
	incoming = make(chan *mininaruv1.ChatClientEvent, 1)
	go receiveChat(stream, incoming, cancel)

	err = stream.Send(&mininaruv1.ChatServerEvent{Event: &mininaruv1.ChatServerEvent_Started{Started: &mininaruv1.ChatStarted{TurnId: uuid.NewString()}}})
	if err != nil {
		return err
	}

	defs, err = remoteToolDefs(chatCtx, stream, incoming, &sendMu, start.GetTools())
	if err != nil {
		return err
	}

	message, err = instance.ChatWithTools(chatCtx, session, start.GetContent(), defs, start.GetThinking(),
		func(text string) {
			sendMu.Lock()
			if streamErr == nil {
				streamErr = stream.Send(chatContentEvent(text))
			}
			sendMu.Unlock()
			if streamErr != nil {
				cancel()
			}
		}, func(text string) {
			sendMu.Lock()
			if streamErr == nil {
				streamErr = stream.Send(chatReasoningEvent(text))
			}
			sendMu.Unlock()
			if streamErr != nil {
				cancel()
			}
		}, func(event core.ToolEvent) {
			sendMu.Lock()
			if streamErr == nil {
				streamErr = stream.Send(chatToolEvent(event))
			}
			sendMu.Unlock()
			if streamErr != nil {
				cancel()
			}
		}, chatApprover(chatCtx, stream, incoming))
	if streamErr != nil {
		return streamErr
	}
	if err != nil {
		return sendChatFailure(stream, err)
	}

	totals, err = core.SessionUsage(session.Id)
	if err != nil {
		return sendChatFailure(stream, fmt.Errorf("read usage: %w", err))
	}

	return stream.Send(&mininaruv1.ChatServerEvent{Event: &mininaruv1.ChatServerEvent_Completed{Completed: &mininaruv1.ChatCompleted{
		Message: rpcMessage(message), Usage: rpcUsage(totals)}}})
}
