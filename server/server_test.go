package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
)

func streamChunk(delta, finish string) string {
	return `data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":` +
		delta + `,"finish_reason":` + finish + `}]}` + "\n\n"
}

func containsAll(text string, wants ...string) bool {
	var want string

	for _, want = range wants {
		if strings.Contains(text, want) {
			continue
		}

		return false
	}

	return true
}

func setupAgent(t *testing.T, upstream string) *core.Registry {
	var err error

	t.Helper()

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	core.Providers = nil
	core.DefaultProvider = nil
	core.Agents = nil
	core.Global = nil

	core.ProviderCreate(core.Provider{Name: "local", BaseURL: upstream, ApiKey: "k"})
	core.Global = core.AgentNew("naru", "you are naru", "", "gemma", core.Providers[0])

	return reloadRegistry(t, core.NewRegistry())
}

func reloadRegistry(t *testing.T, reg *core.Registry) *core.Registry {
	var err error

	t.Helper()

	err = core.ProviderSave()
	if err != nil {
		t.Fatal(err)
	}

	err = core.AgentSave()
	if err != nil {
		t.Fatal(err)
	}

	err = reg.Reload()
	if err != nil {
		t.Fatal(err)
	}

	return reg
}

func request(t *testing.T, handler http.Handler, method, path, key, body string) *httptest.ResponseRecorder {
	var recorder *httptest.ResponseRecorder
	var req *http.Request

	t.Helper()

	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(method, path, strings.NewReader(body))
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	handler.ServeHTTP(recorder, req)

	return recorder
}

func TestAuthorizeRequiresMatchingBearerKey(t *testing.T) {
	var reg *core.Registry
	var handler http.Handler
	var recorder *httptest.ResponseRecorder

	reg = setupAgent(t, "http://127.0.0.1")
	handler = routes("secret", reg)

	recorder = request(t, handler, http.MethodGet, pathModels, "", "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("missing key = %d, want 401", recorder.Code)
	}

	recorder = request(t, handler, http.MethodGet, pathModels, "wrong", "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("wrong key = %d, want 401", recorder.Code)
	}

	recorder = request(t, handler, http.MethodGet, pathModels, "secret", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("valid key = %d, want 200", recorder.Code)
	}
}

func TestServeRejectsEmptyApiKey(t *testing.T) {
	var err error

	err = Serve(t.Context(), Config{Host: DefaultHost, Port: 0}, core.NewRegistry())
	if err == nil {
		t.Fatal("serve accepted an empty api key")
	}

	err = Serve(t.Context(), Config{Host: DefaultHost, Port: 0, ApiKey: "k"}, nil)
	if err == nil {
		t.Fatal("serve accepted a nil registry")
	}
}

func TestHTTPServerHasConnectionLimits(t *testing.T) {
	var srv *http.Server

	srv = newHTTPServer(http.NotFoundHandler())
	if srv.ReadHeaderTimeout != readHeaderTimeout {
		t.Fatalf("read header timeout = %s", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != readTimeout {
		t.Fatalf("read timeout = %s", srv.ReadTimeout)
	}
	if srv.IdleTimeout != idleTimeout {
		t.Fatalf("idle timeout = %s", srv.IdleTimeout)
	}
	if srv.MaxHeaderBytes != maxHeaderBytes {
		t.Fatalf("max header bytes = %d", srv.MaxHeaderBytes)
	}
}

func TestConcurrentLimitRejectsExcessRequest(t *testing.T) {
	var entered chan struct{}
	var release chan struct{}
	var handler http.Handler
	var first *httptest.ResponseRecorder
	var wait sync.WaitGroup
	var second *httptest.ResponseRecorder

	entered = make(chan struct{})
	release = make(chan struct{})
	handler = limitConcurrent(1, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))
	first = httptest.NewRecorder()
	wait.Add(1)
	go func() {
		defer wait.Done()
		handler.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/", nil))
	}()
	<-entered

	second = httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("excess request = %d, want 429", second.Code)
	}
	if second.Header().Get("Retry-After") != "1" {
		t.Fatalf("retry-after = %q", second.Header().Get("Retry-After"))
	}

	close(release)
	wait.Wait()
	if first.Code != http.StatusNoContent {
		t.Fatalf("first request = %d, want 204", first.Code)
	}
}
