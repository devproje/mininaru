package agent

import (
	"database/sql"

	"git.wh64.net/naru-studio/mininaru/modules/database"
)

type AgentModule struct {
	DB *sql.DB
}

type AgentData struct {
	Id           string             `json:"id"`
	Name         string             `json:"name"`
	Engine       *AgentEngine       `json:"engine"`
	Default      bool               `json:"default"`
	Workspaces   []AgentWorkspace   `json:"workspaces"`
	Instructions []AgentInstruction `json:"instructions"`
}

func (m *AgentModule) Name() string {
	return "agent-module"
}

func (m *AgentModule) Load() error {
	if m.DB != nil {
		return nil
	}

	m.DB = database.Database.DB
	return nil
}

func (m *AgentModule) Unload() error {
	if m.DB == nil {
		return nil
	}

	m.DB = nil
	return nil
}

func (m *AgentModule) Create(engineId string, payload *AgentData) error {
	return nil
}

func (m *AgentModule) Read(id string) (*AgentData, error) {
	return nil, nil
}

func (m *AgentModule) GetDefault() (*AgentData, error) {
	return nil, nil
}

func (m *AgentModule) SetName(id string, newname string) error {
	return nil
}

func (m *AgentModule) SetEngine(id string, engineId string) error {
	return nil
}

func (m *AgentModule) SetDefault(id string) error {
	return nil
}

func (m *AgentModule) Delete(id string) error {
	return nil
}

var Agent *AgentModule = &AgentModule{}
