package agent

type AgentModule struct{}

func (m *AgentModule) Name() string {
	return "agent-module"
}

func (m *AgentModule) Load() error {
	return nil
}

func (m *AgentModule) Unload() error {
	return nil
}

var Agent *AgentModule = &AgentModule{}
