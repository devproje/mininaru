package tui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devproje/mininaru/modules/client"
	"github.com/gorilla/websocket"
)

func TestDimEachLine(t *testing.T) {
	var got string
	var want string

	got = dimEachLine("a\nb")
	want = client.DIM + "a" + client.RESET + "\n" + client.DIM + "b" + client.RESET

	if got != want {
		t.Fatalf("dimEachLine = %q, want %q", got, want)
	}
}

func TestAppendTextEmptyIsNoop(t *testing.T) {
	var m tuiModel

	m.lines = []string{"x"}
	m.appendText("")

	if len(m.lines) != 1 || m.lines[0] != "x" {
		t.Fatalf("lines = %v, want unchanged", m.lines)
	}
}

func TestAppendTextAppendsToLastLine(t *testing.T) {
	var m tuiModel

	m.lines = []string{"hello"}
	m.appendText(" world")

	if len(m.lines) != 1 || m.lines[0] != "hello world" {
		t.Fatalf("lines = %v", m.lines)
	}
}

func TestAppendTextSplitsOnNewline(t *testing.T) {
	var m tuiModel

	m.lines = []string{"a"}
	m.appendText("b\nc\nd")

	if len(m.lines) != 3 {
		t.Fatalf("lines = %v, want 3 entries", m.lines)
	}
	if m.lines[0] != "ab" || m.lines[1] != "c" || m.lines[2] != "d" {
		t.Fatalf("lines = %v", m.lines)
	}
}

func TestAppendTextSeedsEmptyLines(t *testing.T) {
	var m tuiModel

	m.appendText("first")

	if len(m.lines) != 1 || m.lines[0] != "first" {
		t.Fatalf("lines = %v", m.lines)
	}
}

func TestAppendStreamReasoningIsDimmed(t *testing.T) {
	var m tuiModel

	m.appendStream("thinking", "")

	if len(m.lines) != 1 || m.lines[0] != dimEachLine("thinking") {
		t.Fatalf("lines = %v", m.lines)
	}
}

func TestAppendStreamContentGoesThroughMarkdown(t *testing.T) {
	var m tuiModel

	m.appendStream("", "hello\n")

	if len(m.lines) == 0 || !strings.Contains(m.lines[0], "hello") {
		t.Fatalf("lines = %v, want content rendered", m.lines)
	}
}

func TestToolSummarySkipsStarted(t *testing.T) {
	if toolSummary("bash", "started", "") != "" {
		t.Fatal("expected no summary for started status")
	}
}

func TestToolSummaryPlainMessage(t *testing.T) {
	var got string

	got = toolSummary("bash", "finished", "ls -la")

	if !strings.Contains(got, "bash") || !strings.Contains(got, "finished") || !strings.Contains(got, "ls -la") {
		t.Fatalf("summary = %q", got)
	}
}

func TestToolSummaryDiffShowsStats(t *testing.T) {
	var got string

	got = toolSummary("file_edit", "finished", "--- a\n+++ b\n@@ -1,2 +1,1 @@\n+added\n-removed\n-removed2")

	if !strings.Contains(got, "+1") || !strings.Contains(got, "-2") {
		t.Fatalf("summary = %q, want +1/-2 stats", got)
	}
}

func TestToolSummaryPlainMultilineIsNotMistakenForDiff(t *testing.T) {
	var got string

	got = toolSummary("bash_exec", "finished", "total 48\n-rw-r--r-- 1 x x 100 go.mod\ndrwxr-xr-x 2 x x 4096 cmd")

	if strings.Contains(got, "+0") || strings.Contains(got, "-2") {
		t.Fatalf("summary = %q, plain ls -la output should not be read as a diff", got)
	}
}

func dialTestConn(t *testing.T, handle func(server *websocket.Conn)) *websocket.Conn {
	var srv *httptest.Server
	var conn *websocket.Conn

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var up websocket.Upgrader
		var server *websocket.Conn

		var handlerErr error

		server, handlerErr = up.Upgrade(w, r, nil)
		if handlerErr != nil {
			return
		}

		handle(server)
	}))
	t.Cleanup(srv.Close)

	conn, _, err = websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	return conn
}

func TestSubmitWhileBusyQueuesMessage(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var joined string

	m = newTuiModel("a", "s")
	m.conn = dialTestConn(t, func(server *websocket.Conn) {})
	m.busy = true
	m.input.SetValue("hello")

	model, _ = m.submit()
	m = model.(tuiModel)

	if len(m.queuedMsgs) != 1 || m.queuedMsgs[0] != "hello" {
		t.Fatalf("queuedMsgs = %v, want [hello]", m.queuedMsgs)
	}

	joined = strings.Join(m.lines, "\n")
	if !strings.Contains(joined, "queued") {
		t.Fatalf("lines = %v, want a queued notice", m.lines)
	}
}

func TestFlushQueuedSendsOnceIdle(t *testing.T) {
	var done chan struct{}
	var m tuiModel
	var received client.Frame
	var cmd tea.Cmd

	done = make(chan struct{})

	m = newTuiModel("a", "s")
	m.conn = dialTestConn(t, func(server *websocket.Conn) {
		defer close(done)
		server.ReadJSON(&received)
	})
	m.queuedMsgs = []string{"queued-hello"}
	m.busy = false

	cmd = m.flushQueued()
	if cmd == nil {
		t.Fatal("expected flushQueued to return the spinner-tick command")
	}

	<-done

	if received.Content != "queued-hello" {
		t.Fatalf("sent content = %q, want %q", received.Content, "queued-hello")
	}
	if len(m.queuedMsgs) != 0 {
		t.Fatalf("queuedMsgs = %v, want drained", m.queuedMsgs)
	}
	if !m.busy {
		t.Fatal("busy = false, want true once the queued message is sent")
	}
}

func TestFooterHeightBaseline(t *testing.T) {
	var m tuiModel

	if m.footerHeight() != tuiFooterHeight {
		t.Fatalf("footerHeight = %d, want %d", m.footerHeight(), tuiFooterHeight)
	}
}

func TestFooterHeightBusyAddsRow(t *testing.T) {
	var m tuiModel

	m.busy = true

	if m.footerHeight() != tuiFooterHeight+1 {
		t.Fatalf("footerHeight = %d, want %d", m.footerHeight(), tuiFooterHeight+1)
	}
}

func TestFooterHeightAwaitingAddsOptionRows(t *testing.T) {
	var m tuiModel

	m.busy = true
	m.awaiting = "approval"
	m.awaitOptions = []string{"once", "session", "deny"}

	if m.footerHeight() != tuiFooterHeight+3 {
		t.Fatalf("footerHeight = %d, want %d (busy row suppressed while awaiting)", m.footerHeight(), tuiFooterHeight+3)
	}
}

func TestLayoutPadsChatBodyButNotHeader(t *testing.T) {
	var m tuiModel
	var lines []string
	var headerBorder string
	var bodyLine string

	m = newTuiModel("a", "s")
	m.width = 60
	m.height = 20
	m.appendText("hello")
	m.layout()

	lines = strings.Split(m.viewport.View(), "\n")
	headerBorder = lines[len(lines)-2]
	bodyLine = lines[len(lines)-1]

	if !strings.HasPrefix(headerBorder, "╰") {
		t.Fatalf("header border = %q, want the banner's own border unaffected by body padding", headerBorder)
	}
	if !strings.HasPrefix(bodyLine, " hello") {
		t.Fatalf("body line = %q, want the chat body left-padded", bodyLine)
	}
}

func TestUpdateCtrlVWithoutSessionIsNoop(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var cmd tea.Cmd

	m = newTuiModel("a", "s")

	model, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlV})
	m = model.(tuiModel)

	if cmd != nil {
		t.Fatalf("cmd = %v, want nil when no session is connected", cmd)
	}
}

func TestExtractPastedImagePathAcceptsExistingImageFile(t *testing.T) {
	var f *os.File
	var path string
	var ok bool

	var err error

	f, err = os.CreateTemp("", "pasted-*.png")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	path, ok = extractPastedImagePath(f.Name())

	if !ok {
		t.Fatalf("extractPastedImagePath(%q) = (_, false), want true", f.Name())
	}
	if path != f.Name() {
		t.Fatalf("path = %q, want %q", path, f.Name())
	}
}

func TestExtractPastedImagePathRejectsPlainText(t *testing.T) {
	var ok bool

	_, ok = extractPastedImagePath("just some pasted text")

	if ok {
		t.Fatal("expected plain pasted text to not be treated as an image path")
	}
}

func TestExtractPastedImagePathRejectsMissingFile(t *testing.T) {
	var ok bool

	_, ok = extractPastedImagePath("/tmp/does-not-exist-mininaru.png")

	if ok {
		t.Fatal("expected a nonexistent path to be rejected")
	}
}

func TestUpdatePasteOfImagePathInsertsPlaceholder(t *testing.T) {
	var f *os.File
	var m tuiModel
	var model tea.Model
	var cmd tea.Cmd

	var err error

	f, err = os.CreateTemp("", "dropped-*.png")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	m = newTuiModel("a", "s")
	m.base = "http://example.invalid"

	model, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Paste: true, Runes: []rune(f.Name())})
	m = model.(tuiModel)

	if cmd == nil {
		t.Fatal("expected a background upload command for a pasted image path")
	}
	if m.input.Value() != "[image #1] " {
		t.Fatalf("input = %q, want a placeholder chip inserted", m.input.Value())
	}
	if m.imageCount != 1 {
		t.Fatalf("imageCount = %d, want 1", m.imageCount)
	}
}

func TestSuggestionWindowFitsWithoutScrolling(t *testing.T) {
	var start int
	var end int

	start, end = suggestionWindow(0, 3, maxCmdSuggestionRows)

	if start != 0 || end != 3 {
		t.Fatalf("window = [%d:%d], want [0:3] when everything fits", start, end)
	}
}

func TestSuggestionWindowFollowsCursorPastVisibleRows(t *testing.T) {
	var start int
	var end int

	start, end = suggestionWindow(maxCmdSuggestionRows, 13, maxCmdSuggestionRows)

	if end-start != maxCmdSuggestionRows {
		t.Fatalf("window = [%d:%d], want exactly %d rows", start, end, maxCmdSuggestionRows)
	}
	if maxCmdSuggestionRows < start || maxCmdSuggestionRows >= end {
		t.Fatalf("window = [%d:%d], want the cursor at %d to stay inside it", start, end, maxCmdSuggestionRows)
	}
}

func TestSuggestionWindowClampsAtTheEnd(t *testing.T) {
	var start int
	var end int

	start, end = suggestionWindow(12, 13, maxCmdSuggestionRows)

	if end != 13 {
		t.Fatalf("window = [%d:%d], want it to end at the last suggestion", start, end)
	}
	if end-start != maxCmdSuggestionRows {
		t.Fatalf("window = [%d:%d], want exactly %d rows", start, end, maxCmdSuggestionRows)
	}
}

func TestFooterHeightCapsSuggestionRows(t *testing.T) {
	var m tuiModel

	m = newTuiModel("a", "s")
	m.input.SetValue("/")

	if m.footerHeight() != tuiFooterHeight+maxCmdSuggestionRows {
		t.Fatalf("footerHeight = %d, want %d (capped at %d rows for %d suggestions)",
			m.footerHeight(), tuiFooterHeight+maxCmdSuggestionRows, maxCmdSuggestionRows, len(tuiCmdList))
	}
}

func TestCmdMenuRendersOnlyTheVisibleWindow(t *testing.T) {
	var m tuiModel
	var menu string

	m = newTuiModel("a", "s")
	m.input.SetValue("/")
	m.cmdCursor = len(tuiCmdList) - 1

	menu = m.cmdMenu(0, tuiCmdSuggestions("/"))

	if strings.Count(menu, "\n")+1 != maxCmdSuggestionRows {
		t.Fatalf("menu lines = %d, want %d", strings.Count(menu, "\n")+1, maxCmdSuggestionRows)
	}
	if !strings.Contains(menu, "/"+tuiCmdList[len(tuiCmdList)-1].name) {
		t.Fatalf("menu = %q, want the cursor's entry kept visible", menu)
	}
}

func TestUpdateToolThenContentStartsNewLine(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var last string

	m = newTuiModel("a", "s")

	model, _ = m.Update(tuiToolMsg{name: "ask_user_question", status: "finished", message: ""})
	m = model.(tuiModel)

	model, _ = m.Update(tuiChunkMsg{content: "answer text\n"})
	m = model.(tuiModel)

	last = m.lines[len(m.lines)-2]

	if !strings.Contains(last, "answer text") {
		t.Fatalf("line = %q, want the new chunk on its own line", last)
	}
	if strings.Contains(last, "finished") {
		t.Fatalf("line = %q, tool summary bled into the content line", last)
	}
}

func TestUpdateConsecutiveContentChunksDontInsertBlankLines(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var afterFirst int

	m = newTuiModel("a", "s")

	model, _ = m.Update(tuiChunkMsg{content: "hello "})
	m = model.(tuiModel)

	afterFirst = len(m.lines)

	model, _ = m.Update(tuiChunkMsg{content: "world"})
	m = model.(tuiModel)

	if len(m.lines) != afterFirst {
		t.Fatalf("lines grew from %d to %d across two chunks of the same stream", afterFirst, len(m.lines))
	}
	if m.streamMode != "content" {
		t.Fatalf("streamMode = %q, want %q", m.streamMode, "content")
	}
}

func TestUpdateToolStartedThenFinishedReplacesLine(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var before int

	m = newTuiModel("a", "s")

	model, _ = m.Update(tuiToolMsg{name: "ask_user_question", status: "started"})
	m = model.(tuiModel)

	before = len(m.lines)
	if !strings.Contains(m.lines[len(m.lines)-1], "running") {
		t.Fatalf("lines = %v, want a running indicator", m.lines)
	}

	model, _ = m.Update(tuiToolMsg{name: "ask_user_question", status: "finished"})
	m = model.(tuiModel)

	if len(m.lines) != before {
		t.Fatalf("line count changed from %d to %d, want the running line replaced in place", before, len(m.lines))
	}
	if strings.Contains(m.lines[len(m.lines)-1], "running") {
		t.Fatalf("lines = %v, running indicator was not replaced", m.lines)
	}
	if !strings.Contains(m.lines[len(m.lines)-1], "finished") {
		t.Fatalf("lines = %v, want the finished summary in place of the running line", m.lines)
	}
	if m.activeToolLine != -1 || m.activeToolName != "" {
		t.Fatalf("active tool state not cleared: line=%d name=%q", m.activeToolLine, m.activeToolName)
	}
}

func TestUpdateToolFinishedWithoutStartedFallsBackToAppend(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var before int

	m = newTuiModel("a", "s")
	before = len(m.lines)

	model, _ = m.Update(tuiToolMsg{name: "bash_exec", status: "finished", message: "ok"})
	m = model.(tuiModel)

	if len(m.lines) <= before {
		t.Fatalf("lines = %v, want a new line appended when there was no started event", m.lines)
	}
}

func TestUpdateToolDiffShowsBody(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var joined string

	m = newTuiModel("a", "s")

	model, _ = m.Update(tuiToolMsg{
		name:    "file_edit",
		status:  "finished",
		message: "--- a\n+++ b\n@@ -1,2 +1,1 @@\n+added\n-removed\n-removed2",
	})
	m = model.(tuiModel)

	joined = strings.Join(m.lines, "\n")

	if !strings.Contains(joined, "+added") || !strings.Contains(joined, "-removed") {
		t.Fatalf("lines = %v, want the diff body shown below the summary", m.lines)
	}
}

func TestFormatToolCallExtractsPrimaryArg(t *testing.T) {
	var got string

	got = formatToolCall("bash_exec", `{"command": "rm a.txt"}`)

	if got != "bash_exec(rm a.txt)" {
		t.Fatalf("got = %q", got)
	}
}

func TestFormatToolCallFallsBackToRawArguments(t *testing.T) {
	var got string

	got = formatToolCall("some_mcp_tool", `{"x":1}`)

	if got != `some_mcp_tool({"x":1})` {
		t.Fatalf("got = %q", got)
	}
}

func TestUpdateApprovalPromptUsesFormattedCall(t *testing.T) {
	var m tuiModel
	var model tea.Model
	var joined string

	m = newTuiModel("a", "s")

	model, _ = m.Update(tuiPromptMsg{
		kind:      "approval",
		name:      "bash_exec",
		arguments: `{"command": "rm a.txt && ls a.txt 2>&1"}`,
		cwd:       "/home/devproje/Workspace/mininaru",
	})
	m = model.(tuiModel)

	joined = strings.Join(m.lines, "\n")

	if !strings.Contains(joined, "bash_exec(rm a.txt && ls a.txt 2>&1)") {
		t.Fatalf("lines = %v, want the formatted tool call", m.lines)
	}
	if !strings.Contains(joined, "Do you want to proceed?") {
		t.Fatalf("lines = %v, want the claude-code style question", m.lines)
	}
	if len(m.awaitLabels) != 3 || m.awaitLabels[0] != "Yes" {
		t.Fatalf("awaitLabels = %v", m.awaitLabels)
	}
}

func TestInputHistoryUpRecallsPreviousMessages(t *testing.T) {
	var m tuiModel
	var model tea.Model

	m = newTuiModel("a", "s")
	m.pushHistory("first message")
	m.pushHistory("second message")

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = model.(tuiModel)

	if m.input.Value() != "second message" {
		t.Fatalf("input = %q, want the most recent history entry", m.input.Value())
	}

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = model.(tuiModel)

	if m.input.Value() != "first message" {
		t.Fatalf("input = %q, want to keep walking back through history", m.input.Value())
	}

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = model.(tuiModel)

	if m.input.Value() != "first message" {
		t.Fatalf("input = %q, want to stop at the oldest entry", m.input.Value())
	}
}

func TestInputHistoryDownRestoresDraft(t *testing.T) {
	var m tuiModel
	var model tea.Model

	m = newTuiModel("a", "s")
	m.pushHistory("earlier message")
	m.input.SetValue("draft in progress")

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = model.(tuiModel)

	if m.input.Value() != "earlier message" {
		t.Fatalf("input = %q, want the recalled entry", m.input.Value())
	}

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(tuiModel)

	if m.input.Value() != "draft in progress" {
		t.Fatalf("input = %q, want the original draft restored", m.input.Value())
	}
	if m.historyPos != -1 {
		t.Fatalf("historyPos = %d, want -1 after returning to the draft", m.historyPos)
	}
}

func TestInputHistoryResetsWhenTyping(t *testing.T) {
	var m tuiModel
	var model tea.Model

	m = newTuiModel("a", "s")
	m.pushHistory("recalled entry")

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = model.(tuiModel)

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = model.(tuiModel)

	if m.historyPos != -1 {
		t.Fatalf("historyPos = %d, want reset to -1 after editing", m.historyPos)
	}
}

func TestEscBlursThenRefocusesInput(t *testing.T) {
	var m tuiModel
	var model tea.Model

	m = newTuiModel("a", "s")

	if !m.input.Focused() {
		t.Fatal("expected input to start focused")
	}

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(tuiModel)

	if m.input.Focused() {
		t.Fatal("expected esc to blur the input")
	}

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(tuiModel)

	if !m.input.Focused() {
		t.Fatal("expected a second esc to refocus the input")
	}
}

func TestUpDownScrollsViewportWhenBlurred(t *testing.T) {
	var m tuiModel
	var i int
	var before int
	var model tea.Model

	m = newTuiModel("a", "s")
	m.pushHistory("should not be recalled")
	m.width = 40
	m.height = 10
	m.lines = make([]string, 0, 40)
	for i = 0; i < 40; i++ {
		m.lines = append(m.lines, "line")
	}
	m.layout()
	m.input.Blur()

	before = m.viewport.YOffset

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = model.(tuiModel)

	if m.viewport.YOffset >= before {
		t.Fatalf("YOffset = %d, want it to scroll up from %d while blurred", m.viewport.YOffset, before)
	}
	if m.input.Value() != "" {
		t.Fatalf("input = %q, want history untouched while blurred", m.input.Value())
	}
	if m.input.Focused() {
		t.Fatal("expected up/down to not refocus the input")
	}
}

func TestTypingWhileBlurredRefocusesInput(t *testing.T) {
	var m tuiModel
	var model tea.Model

	m = newTuiModel("a", "s")
	m.input.Blur()

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m = model.(tuiModel)

	if !m.input.Focused() {
		t.Fatal("expected typing to refocus the input")
	}
	if m.input.Value() != "h" {
		t.Fatalf("input = %q, want the typed character inserted", m.input.Value())
	}
}

func TestInputHistorySkipsConsecutiveDuplicates(t *testing.T) {
	var m tuiModel

	m = newTuiModel("a", "s")
	m.pushHistory("same")
	m.pushHistory("same")

	if len(m.history) != 1 {
		t.Fatalf("history = %v, want consecutive duplicates collapsed", m.history)
	}
}
