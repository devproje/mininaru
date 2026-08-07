package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/devproje/mininaru/util"
	"github.com/openai/openai-go"
)

const completionBody = `{"id":"1","object":"chat.completion","created":1,"model":"m",
"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`

func fakeProvider(hits *int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hits++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(completionBody))
	}))
}

func ask(t *testing.T, agent *NaruAgent) {
	var err error

	t.Helper()

	if agent.AI == nil {
		t.Fatalf("agent %q has no client", agent.Name)
	}

	_, err = agent.AI.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model:    "m",
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func setup(t *testing.T, hitA, hitB *int) (*Provider, *Provider) {
	var srvA, srvB *httptest.Server

	var err error

	t.Helper()

	srvA = fakeProvider(hitA)
	srvB = fakeProvider(hitB)
	t.Cleanup(srvA.Close)
	t.Cleanup(srvB.Close)

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	Providers = []*Provider{}
	Global = nil
	Agents = nil

	ProviderCreate(Provider{Name: "alpha", BaseURL: srvA.URL, ApiKey: "k"})
	ProviderCreate(Provider{Name: "beta", BaseURL: srvB.URL, ApiKey: "k"})

	err = ProviderSave()
	if err != nil {
		t.Fatal(err)
	}

	DefaultProvider = Providers[0]

	return Providers[0], Providers[1]
}

func reload(t *testing.T) {
	var err error

	t.Helper()

	Global = nil
	Agents = nil
	Providers = nil

	err = ProviderInit()
	if err != nil {
		t.Fatal(err)
	}

	err = AgentInit()
	if err != nil {
		t.Fatal(err)
	}
}

func TestAgentKeepsOwnProviderAcrossReload(t *testing.T) {
	var alpha, beta *Provider
	var hitA, hitB int
	var err error

	alpha, beta = setup(t, &hitA, &hitB)

	Global = AgentNew("global", "", "", "m", alpha)
	err = AgentCreate("sub", "", "", "m", beta)
	if err != nil {
		t.Fatal(err)
	}

	reload(t)

	if Global == nil || len(Agents) != 1 {
		t.Fatalf("reload lost agents: global=%v agents=%d", Global, len(Agents))
	}

	ask(t, Global)
	if hitA != 1 || hitB != 0 {
		t.Fatalf("global agent should hit alpha: alpha=%d beta=%d", hitA, hitB)
	}

	ask(t, Agents[0])
	if hitB != 1 {
		t.Fatalf("sub agent should hit its own provider beta, but beta=%d (alpha=%d)", hitB, hitA)
	}
}

func TestAgentWithoutProviderIdFallsBackToDefault(t *testing.T) {
	var alpha *Provider
	var hitA, hitB int
	var err error

	alpha, _ = setup(t, &hitA, &hitB)

	Global = AgentNew("global", "", "", "m", alpha)
	err = AgentCreate("legacy", "", "", "m", alpha)
	if err != nil {
		t.Fatal(err)
	}

	Agents[0].ProviderId = ""

	err = AgentSave()
	if err != nil {
		t.Fatal(err)
	}

	reload(t)

	ask(t, Agents[0])
	if hitA != 1 {
		t.Fatalf("agent without provider_id should fall back to the default provider: alpha=%d beta=%d", hitA, hitB)
	}
}

func TestAgentByNameFindsGlobalAndNamedAgents(t *testing.T) {
	var found *NaruAgent
	var prov *Provider

	var err error

	Providers = nil
	DefaultProvider = nil
	Agents = nil
	Global = nil

	ProviderCreate(Provider{Name: "p", BaseURL: "http://127.0.0.1", ApiKey: "k"})
	prov = Providers[0]

	Global = AgentNew("naru", "", "", "m", prov)
	Agents = []*NaruAgent{AgentNew("helper", "", "", "q", prov)}

	found, err = AgentByName("naru")
	if err != nil || found != Global {
		t.Fatalf("global lookup = %#v err=%v", found, err)
	}

	found, err = AgentByName("helper")
	if err != nil || found.Model != "q" {
		t.Fatalf("named lookup = %#v err=%v", found, err)
	}

	found, err = AgentByName(Global.Id)
	if err != nil || found != Global {
		t.Fatalf("id fallback = %#v err=%v", found, err)
	}

	_, err = AgentByName("ghost")
	if err == nil {
		t.Fatal("unknown agent name resolved")
	}

	if len(AgentAll()) != 2 {
		t.Fatalf("AgentAll = %d agents, want 2", len(AgentAll()))
	}
}

func agentLifecycleSetup(t *testing.T) *Provider {
	var err error

	t.Helper()

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	util.DB, err = util.InitDatabase(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}

	Providers = nil
	DefaultProvider = nil
	Agents = nil
	Global = nil

	ProviderCreate(Provider{Name: "p", BaseURL: "http://127.0.0.1", ApiKey: "k"})

	return Providers[0]
}

func TestAgentDefaultPromotesAndDemotes(t *testing.T) {
	var prov *Provider
	var previous *NaruAgent

	var err error

	prov = agentLifecycleSetup(t)
	Global = AgentNew("naru", "", "", "m", prov)
	Agents = []*NaruAgent{AgentNew("coder", "", "", "q", prov)}
	previous = Global

	err = AgentDefault("coder")
	if err != nil {
		t.Fatal(err)
	}

	if Global.Name != "coder" {
		t.Fatalf("global = %q, want coder", Global.Name)
	}

	if len(Agents) != 1 || Agents[0] != previous {
		t.Fatalf("demoted agents = %#v, want the previous global", Agents)
	}

	err = AgentDefault("coder")
	if err != nil || Global.Name != "coder" || len(Agents) != 1 {
		t.Fatalf("promoting the current global changed state: global=%q agents=%d err=%v", Global.Name, len(Agents), err)
	}

	err = AgentDefault("ghost")
	if err == nil {
		t.Fatal("promoted an unknown agent")
	}
}

func TestAgentDeleteGlobalPromotesSuccessorAndDropsSessions(t *testing.T) {
	var prov *Provider
	var removed *NaruAgent
	var sessions []*Session
	var count int

	var err error

	prov = agentLifecycleSetup(t)
	Global = AgentNew("naru", "", "", "m", prov)
	Agents = []*NaruAgent{AgentNew("coder", "", "", "q", prov)}
	removed = Global

	_, err = SessionCreate(removed, "doomed")
	if err != nil {
		t.Fatal(err)
	}

	_, err = SessionCreate(Agents[0], "kept")
	if err != nil {
		t.Fatal(err)
	}

	err = AgentDelete("naru")
	if err != nil {
		t.Fatal(err)
	}

	if Global == nil || Global.Name != "coder" {
		t.Fatalf("global after removing the global agent = %#v, want coder promoted", Global)
	}

	if len(Agents) != 0 {
		t.Fatalf("agents = %#v, want the promoted agent removed from the slice", Agents)
	}

	err = util.DB.QueryRow("SELECT count(*) FROM sessions WHERE agent_id = ?;", removed.Id).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("removed agent left %d orphan sessions", count)
	}

	sessions, err = SessionList(Global.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("surviving agent has %d sessions, want 1", len(sessions))
	}
}

func TestAgentDeleteLastAgentLeavesNoGlobal(t *testing.T) {
	var prov *Provider

	var err error

	prov = agentLifecycleSetup(t)
	Global = AgentNew("naru", "", "", "m", prov)

	err = AgentDelete("naru")
	if err != nil {
		t.Fatal(err)
	}

	if Global != nil || len(Agents) != 0 {
		t.Fatalf("global=%#v agents=%#v, want everything cleared", Global, Agents)
	}
}
