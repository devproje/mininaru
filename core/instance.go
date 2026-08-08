package core

import (
	"context"
	"fmt"
	"sync"

	"github.com/devproje/mininaru/modules"
	"github.com/openai/openai-go"
)

type sessionLocks struct {
	gates map[string]chan struct{}
	mu    sync.Mutex
}

type Instance struct {
	Agent *NaruAgent
	Tools []modules.Def

	locks *sessionLocks
}

type Registry struct {
	instances map[string]*Instance
	ordered   []*Instance
	locks     *sessionLocks
	mu        sync.RWMutex
}

func newSessionLocks() *sessionLocks {
	return &sessionLocks{gates: make(map[string]chan struct{})}
}

func (s *sessionLocks) gate(id string) chan struct{} {
	var found chan struct{}
	var ok bool

	s.mu.Lock()
	defer s.mu.Unlock()

	found, ok = s.gates[id]
	if ok {
		return found
	}

	found = make(chan struct{}, 1)
	s.gates[id] = found

	return found
}

func (s *sessionLocks) acquire(ctx context.Context, id string) error {
	var entry chan struct{}

	entry = s.gate(id)

	select {
	case entry <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *sessionLocks) release(id string) {
	<-s.gate(id)
}

func (i *Instance) Complete(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion,
	thinking string, onContent, onReasoning func(string)) (*Completion, error) {
	return Complete(ctx, i.Agent, messages, i.Tools, thinking, onContent, onReasoning)
}

func (i *Instance) Chat(ctx context.Context, session *Session, content string,
	onContent, onReasoning func(string), onTool ToolEventFunc) (*Message, error) {
	var err error

	if session == nil {
		return nil, fmt.Errorf("session is required to chat")
	}

	if session.AgentId != i.Agent.Id {
		return nil, fmt.Errorf("session %s does not belong to agent %s", session.Id, i.Agent.Name)
	}

	err = i.locks.acquire(ctx, session.Id)
	if err != nil {
		return nil, err
	}
	defer i.locks.release(session.Id)

	return chatWithToolPolicy(ctx, session, i.Agent, content, nil, i.Tools, onContent, onReasoning, onTool, nil, false)
}

func (i *Instance) ChatWithTools(ctx context.Context, session *Session, content string, defs []modules.Def,
	onReasoning func(string), onTool ToolEventFunc, approve ToolApprovalFunc) (*Message, error) {
	var err error

	if session == nil {
		return nil, fmt.Errorf("session is required to chat")
	}
	if session.AgentId != i.Agent.Id {
		return nil, fmt.Errorf("session %s does not belong to agent %s", session.Id, i.Agent.Name)
	}
	err = i.locks.acquire(ctx, session.Id)
	if err != nil {
		return nil, err
	}
	defer i.locks.release(session.Id)

	return chatWithToolPolicy(ctx, session, i.Agent, content, nil, defs, nil, onReasoning, onTool, approve, false)
}

func (i *Instance) ChatInput(ctx context.Context, session *Session, content string, parts []openai.ChatCompletionContentPartUnionParam,
	defs []modules.Def, onReasoning func(string), onTool ToolEventFunc, approve ToolApprovalFunc) (*Message, error) {
	var err error

	if session == nil {
		return nil, fmt.Errorf("session is required to chat")
	}
	if session.AgentId != i.Agent.Id {
		return nil, fmt.Errorf("session %s does not belong to agent %s", session.Id, i.Agent.Name)
	}
	err = i.locks.acquire(ctx, session.Id)
	if err != nil {
		return nil, err
	}
	defer i.locks.release(session.Id)

	return chatWithToolPolicy(ctx, session, i.Agent, content, parts, defs, nil, onReasoning, onTool, approve, false)
}

func (i *Instance) Session(name string) (*Session, error) {
	return SessionCreate(i.Agent, name)
}

func (i *Instance) Bind(origin, externalId, name string) (*Session, error) {
	var found *Session

	var err error

	found, err = SessionByExternal(origin, externalId)
	if err != nil {
		return nil, err
	}

	if found != nil && found.AgentId == i.Agent.Id {
		return found, nil
	}

	return SessionAttach(i.Agent, origin, externalId, name)
}

func NewRegistry() *Registry {
	var registry Registry

	registry = Registry{instances: make(map[string]*Instance), locks: newSessionLocks()}

	return &registry
}

func (r *Registry) Reload() error {
	var instances map[string]*Instance
	var agent *NaruAgent
	var taken bool
	var cur *Instance
	var ordered []*Instance

	var err error

	err = ProviderInit()
	if err != nil {
		return err
	}

	err = AgentInit()
	if err != nil {
		return err
	}

	instances = make(map[string]*Instance)

	for _, agent = range AgentAll() {
		if agent.Name == "" {
			continue
		}

		_, taken = instances[agent.Name]
		if taken {
			continue
		}

		cur = &Instance{Agent: agent, Tools: modules.SafeTools(), locks: r.locks}
		instances[agent.Name] = cur
		ordered = append(ordered, cur)
	}

	r.mu.Lock()
	r.instances = instances
	r.ordered = ordered
	r.mu.Unlock()

	return nil
}

func (r *Registry) Get(name string) (*Instance, error) {
	var found *Instance
	var ok bool

	r.mu.RLock()
	found, ok = r.instances[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("agent %s not found", name)
	}

	return found, nil
}

func (r *Registry) ByAgentId(agentId string) (*Instance, error) {
	var cur *Instance

	var err error

	for _, cur = range r.List() {
		if cur.Agent.Id != agentId {
			continue
		}

		return cur, nil
	}

	err = fmt.Errorf("no instance for agent id %s", agentId)

	return nil, err
}

func (r *Registry) Default() (*Instance, error) {
	var listed []*Instance

	listed = r.List()
	if len(listed) == 0 {
		return nil, fmt.Errorf("no agent configured")
	}

	return listed[0], nil
}

func (r *Registry) List() []*Instance {
	var snapshot []*Instance

	r.mu.RLock()
	snapshot = append(snapshot, r.ordered...)
	r.mu.RUnlock()

	return snapshot
}
