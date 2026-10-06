# `modules/tui/` — the full-screen TUI

`mininaru` (no `-p`) is a [bubbletea](https://github.com/charmbracelet/bubbletea)
`Model`/`Update`/`View` program, not a hand-rolled line editor: a scrollable
chat log (`bubbles/viewport`) on top, a one-line compose box
(`bubbles/textarea`) at the bottom. It imports `modules/client` for the wire
protocol and adds nothing to the dependency graph below it — nothing outside
`cli/` imports `modules/tui`.

- `tui.go` — the `tuiModel` struct, its construction (`newTuiModel`,
  `newTuiSessionModel`), the banner header block, and `Init()`.
- `tui_update.go` — `Update()`, the single dispatch point for every
  keystroke, websocket event, and async-command result.
- `tui_view.go` — `View()` and everything it composes: `layout()` (resizes
  the viewport, repaints the header, pads the chat body), `footer()`, the
  approval/question menu, and the `/command` suggestion popup.
- `tui_actions.go` — the mutating helpers `Update()` calls: appending to the
  transcript, `submit()`, `sendMessage()`, the queued-message flush, and the
  generic async-command scaffolding (`beginAsync`/`landAsyncResult`).
- `tui_command.go` — the slash-command registry and `runCommand`.
- `tui_session.go` — `RunTuiSession` (resolve agent/session, dial, attach,
  start the model and the reply pump) and the message types + `tea.Cmd`
  constructors for session/gateway switching.
- `tui_clipboard.go` — clipboard and pasted/dropped-path image detection.

## One input mode, and who owns the socket

Every submitted line either dispatches a `/command` or is sent to the agent
as a turn — there is no shell mode, no toggle between the two, same as
before. (This is a different "mode" from the four-value approval mode —
Default/Plan/Auto Persist/Full Auto, cycled with Shift+Tab — described
under [Mode](approval.md); the TUI has exactly one way of routing what you type,
regardless of which approval mode the current directory is in.)
`RunTuiSession` resolves the agent (`GET /api/agents`) and session
(`GET /api/sessions/:id` for `--session`, else `POST /api/sessions`), dials
`/ws`, sends an `attach` frame, builds the `tuiModel`, and hands it to
`tea.NewProgram(..., tea.WithAltScreen())`.

One goroutine owns the `/ws` read for the connection's lifetime:
`pumpTuiReplies` (`tui_session.go`) loops `client.Pump(conn)`'s channel and
calls `p.Send(...)` for every frame, translating each `Reply` into the
matching bubbletea message (`tuiChunkMsg`, `tuiToolMsg`, `tuiMessageMsg`,
`tuiPromptMsg`, `tuiDoneMsg`) and routing `approval_request`/
`question_request` through the same `answers` channel `Update()` writes to
when a prompt is confirmed. Because this goroutine runs continuously —
not only "during a turn" the way the old REPL's `Receive` did — a
`session_send` injection from another agent reaches the screen immediately
as a `←`-prefixed message line, live, instead of waiting for the viewer's
next turn. A closed channel sends one `tuiDoneMsg{errText:
client.ErrGone.Error()}` and the goroutine returns; **there is no automatic reconnect** — unlike the
old REPL's `sh.reconnect()`, a dropped connection surfaces the error and
stays dropped until the TUI is restarted. The one exception is `/gateway`,
whose `switchGatewayCmd` dials the new endpoint itself and calls
`restartPump` to start a fresh `pumpTuiReplies` goroutine on the new
connection.

## Key handling and focus

`Esc` toggles focus between the compose box and the chat viewport — bubbles'
`textarea.Update` is a no-op while blurred, so a blurred `Up`/`Down` falls
through to the viewport's own key handling and scrolls the log instead of
walking input history. While focused, `Up`/`Down` first check for an open
approval/question prompt (move the selection), then a `/command` suggestion
popup (move the highlighted entry, capped at `maxCmdSuggestionRows` and
scrolled with `suggestionWindow` to keep the cursor visible), then input
history (`m.history`, in-memory for the session only — not written to disk,
gone on restart). `Tab` completes the highlighted suggestion. `Enter` sends
the message, runs the command, or confirms the selected prompt option —
`ta.KeyMap.InsertNewline` is disabled, so there is no multi-line compose and
no Shift+Enter behavior to speak of. Everything else (`Ctrl+A/E` line
start/end, `Ctrl+K`/`Ctrl+U` kill to end/start, `Ctrl+W` kill word back, left
/right, home/end) falls through to `bubbles/textarea`'s own default keymap
unmodified. `Ctrl+C` interrupts the agent's turn if one is running
(`{type: "interrupt"}` over `/ws`) or exits; `Ctrl+D` always exits.

## Async commands and queued messages

Every network-touching slash command (`/usage`, `/compact`, `/session`,
`/gateway`, `/model`, `/effort`, `/bash`, `/!bash`) runs through
`beginAsync(label)`/`landAsyncResult(label, lines)`: a dim "running
/label..." line is inserted and the work runs in its own `tea.Cmd` goroutine
(never inside `Update()`, which must not block), with the footer's
`BarFrame` spinner ticking for as long as `m.busy` stays true. When the
result lands, `landAsyncResult` replaces that same line in place rather than
appending a new one. Sending a chat message while an async command or the
agent's own turn is still in flight doesn't drop it: `submit()` appends it to
`m.queuedMsgs` and shows a "queued — sends once the current turn finishes"
line; `flushQueued()` sends the oldest queued message automatically the next
time the connection goes idle (`tuiDoneMsg`, `tuiCmdResultMsg`, and the
session/gateway/image result handlers all call it on their way out).

## Attaching an image

There is no `/img` command. `tui_clipboard.go` wires two paths into the same
upload flow (`queueImage`, which inserts a `[image #N]` placeholder into the
compose box and fires the upload in the background, silent on success and
reporting a chat-log line only on failure):

- **`Ctrl+V`** (`resolveClipboardImageCmd`) — tries a copied file reference
  first (`text/uri-list` via `wl-paste`/`xclip`), then raw clipboard image
  bytes (`image/png` via `wl-paste`/`xclip`/`pngpaste`, saved to a temp file
  that's removed after the upload).
- **Paste or drag-and-drop** of a path to an existing image file
  (`.png`/`.jpg`/`.jpeg`/`.gif`/`.webp`/`.bmp`) — caught in `Update()`'s
  bracketed-paste handling (`kmsg.Paste`) via `extractPastedImagePath`,
  before the pasted text ever reaches the textarea.

## `/bash` and `/!bash`

`runLocalBash` (`tui_command.go`) runs one `$SHELL -c <args>` in the
session's `cwd` and captures combined output with a plain `exec.Command`;
unlike the old REPL there is no process-group setup and no kill path — the
child is not interruptible from the TUI once started, and `Ctrl+C` while one
is running only sends the websocket interrupt frame, which has no effect on
it. `/bash` POSTs the command, exit status, and captured output (capped at
`bashShareLimit`, 8000 bytes, via `bashTranscript`) to
`POST /api/sessions/:id/messages` as a `user` message once it finishes, so
the agent reads it on its next turn; `/!bash` skips that POST.

## Slash commands

A name-keyed slice (`tuiCmdList`, `tui_command.go`); `runCommand` splits
`/<name> <args>` and switches on the name. `/help`, `/clear`, `/exit`,
`/bash`/`/!bash` (above), `/usage` and `/compact` (refresh/summarize the
context window, async), `/session [id-or-name]` (show, or switch —
`FindSession` tries `GET /api/sessions/:ref` then a name match against the
agent's sessions, then re-`attach`es), `/gateway [name]` (no args: lists the
saved gateways; a name: reconnects to it via `switchGatewayCmd`, starting a
new session there), `/model [provider:model]` (no args: `GET
/api/providers/models` live-fetches every provider's own `/v1/models`
catalog server-side and lists it; an explicit `provider:model` arg sets it
directly via `patchAgent`), and `/effort <off|low|medium|high|max>` (same
`patchAgent` path against `/api/agents/:id`). There is no `/mode` command —
`Shift+Tab` cycles the four modes instead (see [Mode](approval.md)). There is no
`/agent` (agent switching happens with `mininaru agent primary`, outside the
TUI) and no `/reset`.
