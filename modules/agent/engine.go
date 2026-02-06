package agent

type AgentEngine struct {
	Name        string `json:"name"`
	ApiEndpoint string `json:"api_endpoint"`
	ApiKey      string `json:"api_key"`
	Model       string `json:"model"`
}

func (m *AgentModule) CreateEngine(payload *AgentEngine) error {
	var _, err = m.DB.Exec(
		"INSERT INTO agent_engine (`name`, api_endpoint, api_key, model) VALUES (?, ?, ?, ?)",
		payload.Name,
		payload.ApiEndpoint,
		payload.ApiKey,
		payload.Model,
	)
	if err != nil {
		goto handle_err
	}

	return nil

handle_err:
	return err
}

func (m *AgentModule) ReadEngine(id string) (*AgentEngine, error) {
	var engine AgentEngine
	var rows, err = m.DB.Query("SELECT * FROM agent_engine WHERE name = ?", id)
	if err != nil {
		goto handle_err
	}
	defer rows.Close()

	if rows.Next() {
		var name, apiEndpoint, apiKey, model string
		err = rows.Scan(&name, &apiEndpoint, &apiKey, &model)
		if err != nil {
			goto handle_err
		}

		engine.Name = name
		engine.ApiEndpoint = apiEndpoint
		engine.ApiKey = apiKey
		engine.Model = model
	}

	if err = rows.Err(); err != nil {
		goto handle_err
	}

	return &engine, nil

handle_err:
	return nil, err
}
