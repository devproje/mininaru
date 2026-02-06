package agent

type AgentEngine struct {
	ApiEndpoint string `json:"api_endpoint"`
	ApiKey      string `json:"api_key"`
	Model       string `json:"model"`
}

func (m *AgentModule) CreateEngine(payload *AgentEngine) error {
	return nil
}

func (m *AgentModule) ReadEngine(id string) error {
	return nil
}
