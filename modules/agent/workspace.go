package agent

type AgentWorkspace struct {
	Name      string `json:"name"`
	Content   string `json:"content"`
	CreatedAt uint64 `json:"created_at"`
	UpdatedAt uint64 `json:"updated_at"`
}
