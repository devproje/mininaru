package core

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devproje/mininaru/util"
)

const OriginTest = "test"

func registrySetup(t *testing.T, upstream string) *Registry {
	var registry *Registry

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

	ProviderCreate(Provider{Name: "local", BaseURL: upstream, ApiKey: "k"})
	Global = AgentNew("naru", "", "", "m", Providers[0])
	Agents = []*NaruAgent{AgentNew("coder", "", "", "q", Providers[0])}

	err = ProviderSave()
	if err != nil {
		t.Fatal(err)
	}

	err = AgentSave()
	if err != nil {
		t.Fatal(err)
	}

	registry = NewRegistry()

	err = registry.Reload()
	if err != nil {
		t.Fatal(err)
	}

	return registry
}

func TestRegistryResolvesEveryAgentInOrder(t *testing.T) {
	var registry *Registry
	var listed []*Instance
	var found *Instance

	var err error

	registry = registrySetup(t, "http://127.0.0.1")

	listed = registry.List()
	if len(listed) != 2 || listed[0].Agent.Name != "naru" || listed[1].Agent.Name != "coder" {
		t.Fatalf("instances = %#v, want the global agent first", listed)
	}

	found, err = registry.Get("coder")
	if err != nil || found.Agent.Model != "q" {
		t.Fatalf("lookup = %#v err=%v", found, err)
	}

	if len(found.Tools) == 0 {
		t.Fatal("instance exposes no tools")
	}

	_, err = registry.Get("ghost")
	if err == nil {
		t.Fatal("registry resolved an unknown agent")
	}
}

func TestRegistryReloadPicksUpNewAgentsAndKeepsOldInstancesUsable(t *testing.T) {
	var registry *Registry
	var before *Instance
	var after *Instance

	var err error

	registry = registrySetup(t, "http://127.0.0.1")

	before, err = registry.Get("naru")
	if err != nil {
		t.Fatal(err)
	}

	Agents = append(Agents, AgentNew("writer", "", "", "llama", Providers[0]))

	err = AgentSave()
	if err != nil {
		t.Fatal(err)
	}

	err = registry.Reload()
	if err != nil {
		t.Fatal(err)
	}

	if len(registry.List()) != 3 {
		t.Fatalf("after reload = %d instances, want 3", len(registry.List()))
	}

	after, err = registry.Get("naru")
	if err != nil {
		t.Fatal(err)
	}

	if after == before {
		t.Fatal("reload reused the old instance, so a changed provider would not take effect")
	}

	if before.Agent == nil || before.locks != after.locks {
		t.Fatal("session locks must survive a reload or concurrent turns could interleave")
	}
}

func TestInstanceSerializesTurnsOnTheSameSession(t *testing.T) {
	var srv *httptest.Server
	var inflight atomic.Int32
	var overlap atomic.Bool
	var registry *Registry
	var target *Instance
	var session *Session
	var round int
	var group sync.WaitGroup

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if inflight.Add(1) > 1 {
			overlap.Store(true)
		}

		time.Sleep(40 * time.Millisecond)
		inflight.Add(-1)

		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, toolChunk("x", `{"role":"assistant","content":"ok"}`, `"stop"`))
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	registry = registrySetup(t, srv.URL)

	target, err = registry.Get("naru")
	if err != nil {
		t.Fatal(err)
	}

	session, err = target.Session("shared")
	if err != nil {
		t.Fatal(err)
	}

	for round = 0; round < 4; round++ {
		group.Add(1)

		go func() {
			defer group.Done()

			target.Chat(context.Background(), session, "hi", nil, nil, nil)
		}()
	}

	group.Wait()

	if overlap.Load() {
		t.Fatal("two turns on the same session reached the model at once")
	}
}

func TestInstanceRunsDifferentSessionsConcurrently(t *testing.T) {
	var srv *httptest.Server
	var inflight atomic.Int32
	var peakMu sync.Mutex
	var peak int32
	var registry *Registry
	var target *Instance
	var first, second *Session
	var group sync.WaitGroup

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var current int32

		current = inflight.Add(1)

		peakMu.Lock()
		if current > peak {
			peak = current
		}
		peakMu.Unlock()

		time.Sleep(60 * time.Millisecond)
		inflight.Add(-1)

		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, toolChunk("x", `{"role":"assistant","content":"ok"}`, `"stop"`))
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	registry = registrySetup(t, srv.URL)

	target, err = registry.Get("naru")
	if err != nil {
		t.Fatal(err)
	}

	first, err = target.Session("one")
	if err != nil {
		t.Fatal(err)
	}

	second, err = target.Session("two")
	if err != nil {
		t.Fatal(err)
	}

	group.Add(2)

	go func() {
		defer group.Done()

		target.Chat(context.Background(), first, "hi", nil, nil, nil)
	}()

	go func() {
		defer group.Done()

		target.Chat(context.Background(), second, "hi", nil, nil, nil)
	}()

	group.Wait()

	peakMu.Lock()
	defer peakMu.Unlock()

	if peak < 2 {
		t.Fatalf("peak concurrency = %d, want separate sessions to run in parallel", peak)
	}
}

func TestInstanceRejectsForeignSession(t *testing.T) {
	var registry *Registry
	var owner *Instance
	var other *Instance
	var session *Session

	var err error

	registry = registrySetup(t, "http://127.0.0.1")

	owner, err = registry.Get("naru")
	if err != nil {
		t.Fatal(err)
	}

	other, err = registry.Get("coder")
	if err != nil {
		t.Fatal(err)
	}

	session, err = owner.Session("owned")
	if err != nil {
		t.Fatal(err)
	}

	_, err = other.Chat(context.Background(), session, "hi", nil, nil, nil)
	if err == nil {
		t.Fatal("an instance accepted another agent's session")
	}
}

func TestBindKeepsOneLiveSessionPerChannel(t *testing.T) {
	var registry *Registry
	var naru, coder *Instance
	var first, second, switched, found *Session
	var messages []*Message

	var err error

	registry = registrySetup(t, "http://127.0.0.1")

	naru, err = registry.Get("naru")
	if err != nil {
		t.Fatal(err)
	}

	coder, err = registry.Get("coder")
	if err != nil {
		t.Fatal(err)
	}

	first, err = naru.Bind(OriginTest, "channel-1", "chan")
	if err != nil {
		t.Fatal(err)
	}

	second, err = naru.Bind(OriginTest, "channel-1", "chan")
	if err != nil {
		t.Fatal(err)
	}

	if first.Id != second.Id {
		t.Fatal("binding the same channel twice created a second session")
	}

	_, err = MessageSave(first.Id, "user", "remember me", "")
	if err != nil {
		t.Fatal(err)
	}

	switched, err = coder.Bind(OriginTest, "channel-1", "chan")
	if err != nil {
		t.Fatal(err)
	}

	if switched.Id == first.Id || switched.AgentId != coder.Agent.Id {
		t.Fatalf("switching agents reused the old session: %#v", switched)
	}

	found, err = SessionByExternal(OriginTest, "channel-1")
	if err != nil {
		t.Fatal(err)
	}

	if found.Id != switched.Id {
		t.Fatalf("channel resolves to %s, want the newest session %s", found.Id, switched.Id)
	}

	messages, err = MessageList(first.Id)
	if err != nil {
		t.Fatal(err)
	}

	if len(messages) != 1 {
		t.Fatalf("detached session lost its history: %d messages", len(messages))
	}
}

func TestSessionByExternalReturnsNothingForUnknownChannel(t *testing.T) {
	var found *Session

	var err error

	registrySetup(t, "http://127.0.0.1")

	found, err = SessionByExternal(OriginTest, "never-seen")
	if err != nil || found != nil {
		t.Fatalf("unknown channel = %#v err=%v", found, err)
	}

	_, err = SessionByExternal("", "")
	if err == nil {
		t.Fatal("blank origin accepted")
	}
}

func TestLocalSessionsDoNotCollideOnEmptyExternalId(t *testing.T) {
	var registry *Registry
	var naru *Instance

	var err error

	registry = registrySetup(t, "http://127.0.0.1")

	naru, err = registry.Get("naru")
	if err != nil {
		t.Fatal(err)
	}

	_, err = naru.Session("local one")
	if err != nil {
		t.Fatal(err)
	}

	_, err = naru.Session("local two")
	if err != nil {
		t.Fatal(err)
	}
}
