package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules/client"
)

func runBatched(cmd tea.Cmd) tea.Msg {
	var msg tea.Msg
	var batch tea.BatchMsg
	var ok bool

	msg = cmd()

	batch, ok = msg.(tea.BatchMsg)
	if !ok {
		return msg
	}

	return batch[len(batch)-1]()
}

func TestTuiCmdSuggestionsFiltersByPrefix(t *testing.T) {
	var got []tuiCmdInfo

	got = tuiCmdSuggestions("/c")

	if len(got) != 2 {
		t.Fatalf("suggestions = %v, want 2 (clear, compact)", got)
	}
}

func TestTuiCmdSuggestionsIgnoresNonSlash(t *testing.T) {
	if tuiCmdSuggestions("hello") != nil {
		t.Fatal("expected nil suggestions for non-slash input")
	}
}

func TestTuiCmdSuggestionsEmptyAfterSlashListsAll(t *testing.T) {
	var got []tuiCmdInfo

	got = tuiCmdSuggestions("/")

	if len(got) != len(tuiCmdList) {
		t.Fatalf("suggestions = %d, want %d", len(got), len(tuiCmdList))
	}
}

func TestTuiCmdExact(t *testing.T) {
	if !tuiCmdExact("/clear") {
		t.Fatal("expected /clear to be exact")
	}
	if !tuiCmdExact("/clear extra args") {
		t.Fatal("expected exact match to ignore trailing args")
	}
	if tuiCmdExact("/cl") {
		t.Fatal("expected partial /cl to not be exact")
	}
}

func TestRunCommandClearResetsTranscript(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var joined string

	m = newTuiModel("a", "s")
	m.lines = []string{"header", "junk1", "junk2"}

	model, _ = m.runCommand("/clear")
	m = model.(tuiModel)

	joined = strings.Join(m.lines, "\n")

	if strings.Contains(joined, "junk1") || strings.Contains(joined, "junk2") {
		t.Fatalf("lines = %v, want prior transcript dropped", m.lines)
	}
}

func TestRunCommandHelpListsCommands(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var joined string

	m = newTuiModel("a", "s")

	model, _ = m.runCommand("/help")
	m = model.(tuiModel)

	joined = strings.Join(m.lines, "\n")

	if !strings.Contains(joined, "/clear") {
		t.Fatalf("help output missing /clear: %q", joined)
	}
}

func TestRunCommandExitQuits(t *testing.T) {
	var m tuiModel
	var cmd tea.Cmd
	var msg tea.Msg
	var ok bool

	m = newTuiModel("a", "s")

	_, cmd = m.runCommand("/exit")
	if cmd == nil {
		t.Fatal("expected a quit command")
	}

	msg = cmd()
	_, ok = msg.(tea.QuitMsg)
	if !ok {
		t.Fatalf("msg = %T, want tea.QuitMsg", msg)
	}
}

func TestRunCommandUnknownReportsError(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var joined string

	m = newTuiModel("a", "s")

	model, _ = m.runCommand("/nope")
	m = model.(tuiModel)

	joined = strings.Join(m.lines, "\n")

	if !strings.Contains(joined, "unknown command") {
		t.Fatalf("output = %q, want an unknown-command error", joined)
	}
}

func TestRunCommandUsageWithoutSessionReportsError(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var joined string

	m = newTuiModel("a", "s")

	model, _ = m.runCommand("/usage")
	m = model.(tuiModel)

	joined = strings.Join(m.lines, "\n")

	if !strings.Contains(joined, "no session connected") {
		t.Fatalf("output = %q, want a no-session error", joined)
	}
}

func TestRunCommandGatewayWithoutConfigReportsError(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var joined string

	m = newTuiModel("a", "s")

	model, _ = m.runCommand("/gateway")
	m = model.(tuiModel)

	joined = strings.Join(m.lines, "\n")

	if !strings.Contains(joined, "no gateways") {
		t.Fatalf("output = %q, want a no-gateways error", joined)
	}
}

func TestRunCommandGatewayListsConfigured(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var joined string

	m = newTuiModel("a", "s")
	m.gateways = []client.Gateway{{Name: "prod", Url: "wss://prod.example/ws"}}

	model, _ = m.runCommand("/gateway")
	m = model.(tuiModel)

	joined = strings.Join(m.lines, "\n")

	if !strings.Contains(joined, "prod") {
		t.Fatalf("output = %q, want the configured gateway listed", joined)
	}
}

func TestRunCommandSessionShowsCurrent(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var joined string

	m = newTuiModel("a", "sess-123")

	model, _ = m.runCommand("/session")
	m = model.(tuiModel)

	joined = strings.Join(m.lines, "\n")

	if !strings.Contains(joined, "sess-123") {
		t.Fatalf("output = %q, want current session id", joined)
	}
}

func TestRunCommandModelRejectsBadUsage(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var joined string

	m = newTuiModel("a", "s")
	m.base = "http://example.invalid"

	model, _ = m.runCommand("/model not-a-valid-ref")
	m = model.(tuiModel)

	joined = strings.Join(m.lines, "\n")

	if !strings.Contains(joined, "usage: /model") {
		t.Fatalf("output = %q, want a usage error", joined)
	}
}

func TestRunCommandEffortRejectsBadValue(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var joined string

	m = newTuiModel("a", "s")
	m.base = "http://example.invalid"

	model, _ = m.runCommand("/effort extreme")
	m = model.(tuiModel)

	joined = strings.Join(m.lines, "\n")

	if !strings.Contains(joined, "usage: /effort") {
		t.Fatalf("output = %q, want a usage error", joined)
	}
}

func TestRunCommandYoloRejectsBadMode(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var joined string

	m = newTuiModel("a", "s")
	m.base = "http://example.invalid"

	model, _ = m.runCommand("/yolo maybe")
	m = model.(tuiModel)

	joined = strings.Join(m.lines, "\n")

	if !strings.Contains(joined, "usage: /yolo") {
		t.Fatalf("output = %q, want a usage error", joined)
	}
}

func TestRunCommandBashRequiresArgs(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var joined string

	m = newTuiModel("a", "s")

	model, _ = m.runCommand("/bash")
	m = model.(tuiModel)

	joined = strings.Join(m.lines, "\n")

	if !strings.Contains(joined, "usage: /bash") {
		t.Fatalf("output = %q, want a usage error", joined)
	}
}

func TestRunCommandBashDoesNotBlock(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var cmd tea.Cmd
	var joined string

	m = newTuiModel("a", "s")

	model, cmd = m.runCommand("/!bash sleep 10")
	m = model.(tuiModel)

	if cmd == nil {
		t.Fatal("expected runCommand to return a background command for /bash, not run it inline")
	}

	joined = strings.Join(m.lines, "\n")

	if !strings.Contains(joined, "running") {
		t.Fatalf("lines = %v, want a running placeholder shown immediately", m.lines)
	}
	if m.activeToolLine < 0 {
		t.Fatalf("activeToolLine = %d, want it tracked so the result can replace it in place", m.activeToolLine)
	}
}

func TestUpdateBashResultReplacesRunningLine(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var cmd tea.Cmd
	var runningLine int
	var joined string

	m = newTuiModel("a", "s")

	model, cmd = m.runCommand("/!bash echo hello-from-bash")
	m = model.(tuiModel)
	runningLine = m.activeToolLine

	model, _ = m.Update(runBatched(cmd))
	m = model.(tuiModel)

	joined = strings.Join(m.lines, "\n")

	if !strings.Contains(joined, "hello-from-bash") {
		t.Fatalf("lines = %v, want the actual command output", m.lines)
	}
	if strings.Contains(m.lines[runningLine], "running") {
		t.Fatalf("lines = %v, running placeholder at index %d was not replaced", m.lines, runningLine)
	}
	if m.activeToolLine != -1 {
		t.Fatalf("activeToolLine = %d, want reset after the result lands", m.activeToolLine)
	}
}

func TestRunLocalBashCapturesOutput(t *testing.T) {
	var out string

	var err error

	out, err = runLocalBash(".", "echo hi")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out, "hi") {
		t.Fatalf("out = %q", out)
	}
}

func TestRunCommandUsageDoesNotBlock(t *testing.T) {
	var srv *httptest.Server
	var m tuiModel
	var model tea.Model
	var cmd tea.Cmd
	var joined string

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(core.ContextUsage{Used: 10, Limit: 100, MaxContext: 100})
	}))
	defer srv.Close()

	m = newTuiModel("a", "s")
	m.base = srv.URL

	model, cmd = m.runCommand("/usage")
	m = model.(tuiModel)

	if cmd == nil {
		t.Fatal("expected a background command for /usage, not a direct network call")
	}

	joined = strings.Join(m.lines, "\n")
	if !strings.Contains(joined, "running") {
		t.Fatalf("lines = %v, want a running placeholder shown immediately", m.lines)
	}

	model, _ = m.Update(runBatched(cmd))
	m = model.(tuiModel)

	joined = strings.Join(m.lines, "\n")
	if strings.Contains(joined, "running /usage") {
		t.Fatalf("lines = %v, running placeholder was not replaced", m.lines)
	}
	if m.activeToolLine != -1 {
		t.Fatalf("activeToolLine = %d, want reset after the result lands", m.activeToolLine)
	}
}

func TestRunCommandCompactShowsResult(t *testing.T) {
	var srv *httptest.Server
	var m tuiModel
	var model tea.Model
	var cmd tea.Cmd
	var joined string

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(core.ContextUsage{Used: 10, Limit: 100, MaxContext: 100})
	}))
	defer srv.Close()

	m = newTuiModel("a", "s")
	m.base = srv.URL

	model, cmd = m.runCommand("/compact")
	m = model.(tuiModel)

	if cmd == nil {
		t.Fatal("expected a background command for /compact")
	}

	model, _ = m.Update(runBatched(cmd))
	m = model.(tuiModel)

	joined = strings.Join(m.lines, "\n")
	if !strings.Contains(joined, "compacted") {
		t.Fatalf("lines = %v, want the compact result shown", m.lines)
	}
}

func TestRunCommandModelSetDoesNotBlock(t *testing.T) {
	var srv *httptest.Server
	var m tuiModel
	var model tea.Model
	var cmd tea.Cmd
	var joined string

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(core.Agent{Model: "openai:gpt"})
	}))
	defer srv.Close()

	m = newTuiModel("a", "s")
	m.base = srv.URL

	model, cmd = m.runCommand("/model openai:gpt")
	m = model.(tuiModel)

	if cmd == nil {
		t.Fatal("expected a background command for /model")
	}

	model, _ = m.Update(runBatched(cmd))
	m = model.(tuiModel)

	joined = strings.Join(m.lines, "\n")
	if !strings.Contains(joined, "openai:gpt") {
		t.Fatalf("lines = %v, want the new model shown", m.lines)
	}
}

func TestRunCommandGatewaySwitchDoesNotBlock(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var cmd tea.Cmd
	var joined string

	m = newTuiModel("a", "s")
	m.gateways = []client.Gateway{{Name: "prod", Url: "wss://prod.example.invalid/ws"}}

	model, cmd = m.runCommand("/gateway prod")
	m = model.(tuiModel)

	if cmd == nil {
		t.Fatal("expected a background command for /gateway, not a direct dial")
	}

	joined = strings.Join(m.lines, "\n")
	if !strings.Contains(joined, "running") {
		t.Fatalf("lines = %v, want a running placeholder shown immediately", m.lines)
	}
}

func TestBashTranscriptTruncatesLongOutput(t *testing.T) {
	var long string
	var got string

	long = strings.Repeat("x", bashShareLimit+100)
	got = bashTranscript("cmd", long, nil)

	if !strings.Contains(got, "truncated") {
		t.Fatalf("transcript = %q, want truncation marker", got[:200])
	}
}
