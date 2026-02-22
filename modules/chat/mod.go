package chat

import (
	"database/sql"
	"fmt"

	"git.wh64.net/naru-studio/mininaru/modules/agent"
	"git.wh64.net/naru-studio/mininaru/modules/database"
)

type ChatModule struct {
	DB    *sql.DB
	Agent *agent.AgentModule
}

func (c *ChatModule) Name() string {
	return "chat-module"
}

func (c *ChatModule) Load() error {
	var err error

	if database.Database == nil {
		err = fmt.Errorf("database module not loaded")
		return err
	}

	if agent.Agent == nil {
		err = fmt.Errorf("agent module not loaded")
		return err
	}

	c.DB = database.Database.DB
	c.Agent = agent.Agent

	return nil
}

func (c *ChatModule) Unload() error {
	if c.Agent != nil {
		c.Agent = nil
	}

	if c.DB != nil {
		c.DB = nil
	}

	return nil
}

var Chat *ChatModule = &ChatModule{}
