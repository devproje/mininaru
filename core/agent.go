// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/devproje/mininaru/util"
	"github.com/google/uuid"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type NaruAgent struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	Role       string `json:"role"`
	Soul       string `json:"soul"`
	Model      string `json:"model"`
	ProviderId string `json:"provider_id"`

	AI *openai.Client `json:"-"`
}

var modelContextWindows sync.Map

func (a *NaruAgent) modelContextCacheKey() string {
	var provider *Provider

	var err error

	if a == nil {
		return ""
	}
	provider, err = ProviderFind(a.ProviderId)
	if err != nil || provider.BaseURL == "" {
		return ""
	}

	return a.ProviderId + "\x00" + provider.BaseURL + "\x00" + a.Model
}

func (a *NaruAgent) CachedModelContextWindow() int64 {
	var cached any
	var cacheKey string
	var ok bool

	cacheKey = a.modelContextCacheKey()
	if cacheKey == "" {
		return 0
	}
	cached, ok = modelContextWindows.Load(cacheKey)
	if !ok {
		return 0
	}

	return cached.(int64)
}

func (a *NaruAgent) ModelContextWindow(ctx context.Context) int64 {
	var provider *Provider
	var requestCtx context.Context
	var cancel context.CancelFunc
	var endpoint string
	var parsed *url.URL
	var query url.Values
	var request *http.Request
	var response *http.Response
	var payload struct {
		DefaultGenerationSettings struct {
			NCtx int64 `json:"n_ctx"`
		} `json:"default_generation_settings"`
	}
	var window int64
	var cached any
	var cacheKey string
	var ok bool

	var err error

	if a == nil {
		return 0
	}
	provider, err = ProviderFind(a.ProviderId)
	if err != nil || provider.BaseURL == "" {
		return 0
	}
	cacheKey = a.modelContextCacheKey()
	cached, ok = modelContextWindows.Load(cacheKey)
	if ok {
		return cached.(int64)
	}
	endpoint = strings.TrimRight(provider.BaseURL, "/")
	endpoint = strings.TrimSuffix(endpoint, "/v1") + "/props"
	parsed, err = url.Parse(endpoint)
	if err != nil {
		return 0
	}
	if a.Model != "" {
		query = parsed.Query()
		query.Set("model", a.Model)
		parsed.RawQuery = query.Encode()
	}
	requestCtx, cancel = context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	request, err = http.NewRequestWithContext(requestCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return 0
	}
	if provider.ApiKey != "" {
		request.Header.Set("Authorization", "Bearer "+provider.ApiKey)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		return 0
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return 0
	}
	err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload)
	if err != nil {
		return 0
	}
	window = payload.DefaultGenerationSettings.NCtx
	if window > 0 {
		modelContextWindows.Store(cacheKey, window)
	}

	return window
}

type AgentConfig struct {
	Global *NaruAgent   `json:"global"`
	Agents []*NaruAgent `json:"agents"`
}

const AGENT_PATH = "agent.json"

var Global *NaruAgent
var Agents []*NaruAgent
var emptyAgentObj AgentConfig = AgentConfig{}

func newClient(prov *Provider) *openai.Client {
	var opts []option.RequestOption
	var ai openai.Client

	if prov == nil {
		return nil
	}

	if prov.ApiKey != "" {
		opts = append(opts, option.WithAPIKey(prov.ApiKey))
	}

	if prov.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(prov.BaseURL))
	}

	ai = openai.NewClient(opts...)

	return &ai
}

func AgentNew(name, role, soul, model string, prov *Provider) *NaruAgent {
	var agent NaruAgent

	if prov == nil || prov.Id == "" {
		return nil
	}

	agent = NaruAgent{
		Id:         uuid.NewString(),
		Name:       name,
		Role:       role,
		Soul:       soul,
		Model:      model,
		ProviderId: prov.Id,

		AI: newClient(prov),
	}

	return &agent
}

func agentProvider(agent *NaruAgent) *Provider {
	var prov *Provider

	var err error

	prov, err = ProviderFind(agent.ProviderId)
	if err != nil {
		return DefaultProvider
	}

	return prov
}

func AgentInit() error {
	var path string
	var buf []byte
	var cfg AgentConfig
	var cur *NaruAgent

	var err error

	path = util.Path(AGENT_PATH)
	buf, err = os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		buf, _ = json.MarshalIndent(emptyAgentObj, "", "    ")

		err = util.WriteFileAtomic(path, buf, 0600)
		if err != nil {
			return err
		}
	}

	err = json.Unmarshal(buf, &cfg)
	if err != nil {
		return err
	}

	Global = cfg.Global
	Agents = cfg.Agents

	if Global != nil {
		Global.AI = newClient(agentProvider(Global))
	}

	for _, cur = range Agents {
		cur.AI = newClient(agentProvider(cur))
	}

	return nil
}

func AgentSave() error {
	var cfg AgentConfig
	var path string
	var buf []byte

	var err error

	cfg = AgentConfig{
		Global: Global,
		Agents: Agents,
	}

	path = util.Path(AGENT_PATH)
	buf, err = json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}

	err = util.WriteFileAtomic(path, buf, 0600)
	if err != nil {
		return err
	}

	return nil
}

func AgentCreate(name, role, soul, model string, prov *Provider) error {
	var agent *NaruAgent

	var err error

	if Global == nil {
		return fmt.Errorf("orchestration agent not initialized")
	}

	agent = AgentNew(name, role, soul, model, prov)
	if agent == nil {
		return fmt.Errorf("provider is required to create an agent")
	}

	Agents = append(Agents, agent)

	err = AgentSave()
	if err != nil {
		return err
	}

	return nil
}

func AgentFind(id string) *NaruAgent {
	var cur *NaruAgent

	for _, cur = range Agents {
		if cur.Id != id {
			continue
		}

		return cur
	}

	return nil
}

func AgentAll() []*NaruAgent {
	var all []*NaruAgent
	var cur *NaruAgent

	if Global != nil {
		all = append(all, Global)
	}

	for _, cur = range Agents {
		if Global != nil && cur.Id == Global.Id {
			continue
		}

		all = append(all, cur)
	}

	return all
}

func AgentByName(name string) (*NaruAgent, error) {
	var cur *NaruAgent

	var err error

	if name == "" {
		return nil, fmt.Errorf("agent name is required")
	}

	for _, cur = range AgentAll() {
		if cur.Name != name {
			continue
		}

		return cur, nil
	}

	for _, cur = range AgentAll() {
		if cur.Id != name {
			continue
		}

		return cur, nil
	}

	err = fmt.Errorf("agent %s not found", name)

	return nil, err
}

func AgentDefault(ref string) error {
	var target *NaruAgent
	var previous *NaruAgent
	var cur *NaruAgent
	var remaining []*NaruAgent

	var err error

	target, err = AgentByName(ref)
	if err != nil {
		return err
	}

	if Global != nil && Global.Id == target.Id {
		return nil
	}

	previous = Global

	for _, cur = range Agents {
		if cur.Id == target.Id {
			continue
		}

		remaining = append(remaining, cur)
	}

	if previous != nil {
		remaining = append(remaining, previous)
	}

	Global = target
	Agents = remaining

	return AgentSave()
}

func AgentRefreshClient(agent *NaruAgent) error {
	var prov *Provider

	var err error

	if agent == nil {
		return fmt.Errorf("agent is required")
	}

	prov, err = ProviderFind(agent.ProviderId)
	if err != nil {
		return err
	}

	agent.AI = newClient(prov)

	return nil
}

func AgentUpdateFields(id string, name, role, soul, model, providerId *string) error {
	var index int
	var cur *NaruAgent
	var update NaruAgent

	var err error

	for index, cur = range Agents {
		if cur.Id != id {
			continue
		}

		update = *cur

		if name != nil {
			update.Name = *name
		}

		if role != nil {
			update.Role = *role
		}

		if soul != nil {
			update.Soul = *soul
		}

		if model != nil {
			update.Model = *model
		}

		if providerId != nil {
			update.ProviderId = *providerId
			update.AI = newClient(agentProvider(&update))
		}

		Agents[index] = &update
		err = AgentSave()
		if err != nil {
			return err
		}

		return nil
	}

	err = fmt.Errorf("cannot find agent id for %s", id)

	return err
}

func AgentUpdate(id string, payload NaruAgent) error {
	var name, role, soul, model, providerId *string

	if payload.Name != "" {
		name = &payload.Name
	}
	if payload.Role != "" {
		role = &payload.Role
	}
	if payload.Soul != "" {
		soul = &payload.Soul
	}
	if payload.Model != "" {
		model = &payload.Model
	}
	if payload.ProviderId != "" {
		providerId = &payload.ProviderId
	}

	return AgentUpdateFields(id, name, role, soul, model, providerId)
}

func AgentDelete(ref string) error {
	var target *NaruAgent
	var cur *NaruAgent
	var remaining []*NaruAgent

	var err error

	target, err = AgentByName(ref)
	if err != nil {
		return err
	}

	for _, cur = range Agents {
		if cur.Id == target.Id {
			continue
		}

		remaining = append(remaining, cur)
	}

	Agents = remaining

	if Global != nil && Global.Id == target.Id {
		Global = nil

		if len(Agents) > 0 {
			Global = Agents[0]
			Agents = Agents[1:]
		}
	}

	err = SessionDeleteByAgent(target.Id)
	if err != nil {
		return err
	}

	return AgentSave()
}
