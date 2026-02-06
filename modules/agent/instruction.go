package agent

type AgentInstruction struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}
