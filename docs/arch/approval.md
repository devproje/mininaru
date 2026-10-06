# Mode, approval, and questions

## Mode — the trust policy behind `approve`

`root` is the **anchor**: for a loopback `/ws` connection it's the client's
reported cwd (the `cwd` field on the chat frame — `modules/client` sends the
process's working directory, captured once at startup); for a non-loopback
connection it's the server process's own `$HOME`, since a remote peer's
claimed cwd can't be trusted. `core.ResolveAnchor` /
`core.IsLoopbackAddr` (`core/mode.go`) make that call from the raw
`RemoteAddr` the request came in on.

Each dangerous tool passes its current session id and root to `approve`, so a
nested `agent_spawn` or `session_send` round cannot inherit a caller's mode or
session approval merely because the caller initiated it. `core.ModeLookup`
(`core/mode.go`) reads `directory.json` and returns
the most specific (deepest) `{root, mode}` entry covering `anchor` by path
segment — not string prefix, so `/home/user/proj` doesn't match
`/home/user/project2` — defaulting to `"default"` when nothing matches. Four
modes: `default` (always ask), `plan` (safe tools run; read-only dangerous
tools — `file_read`, `browser_read`, `browser_screenshot`, via
`core.IsReadOnlyTool` — still ask, everything else dangerous is
auto-rejected without a prompt), `auto_persist` (only `file_read`/
`file_write`/`file_edit` auto-run, via `core.IsIOTool`, and only inside the
anchor — a path that resolves outside it still falls through to asking, via
`core.ModeEscapesAnchor`; `bash_exec`, `browser_*`, `agent_spawn`, and
`session_send` always ask in this mode), `full_auto`
(auto-run everywhere, no anchor check, and `file_read`/`file_write`/
`file_edit` are built with `unrestricted: true` so `util.SafeJoin` lets an
absolute path through instead of rejecting it). `core.ModeUpsert(root, mode)`
rewrites the file; `"default"` is stored as an explicit entry rather than a
deletion, so a subdirectory can be locked back down under a more permissive
ancestor. `POST /api/mode` (`server/controller/mode.go`) is how a client sets
this — plain REST, not a `/ws` frame, since it's a one-off directory
declaration, not part of a live turn. The TUI has no slash command for this —
`Shift+Tab` cycles the four modes (`modules/tui/tui_update.go`), pushing each
change with the same endpoint and seeding the initial value from
`GET /api/mode?cwd=` when a session connects (see [the TUI](tui.md)). The active mode is always visible: the header dot,
the footer badge, and the input box border color all track it
(`client.ModeColor`/`client.ModeColorCode`, `modules/client/style.go`).

## Escaping Plan mode mid-turn — `core/plan_approval.go`

`buildTools` (`core/tools.go`) adds one more `modules.PermissionSafe` tool,
`plan_approval`, only when `core.ModeLookup(root) == core.ModePlan` — it's
the model's way out of Plan's auto-reject for mutating tools, without the
operator having to Shift+Tab mid-conversation. The model calls it with a `plan`
string once it has something concrete to do; it rides the same `AskFunc`
round-trip `ask_user_question` uses, fixed to three options: `Allow once`,
`Allow session (Persist)`, `Deny`. The match on the answer is a
case-insensitive substring check (`strings.Contains` on the lowercased,
trimmed text) rather than an exact match, so a human typing "persist" or
"allow session" instead of picking the listed option still lands correctly.
Picking `Allow once` sets the session's override to `core.ModeDefault`
(every further dangerous call this turn still gets its own prompt — "once"
each); `Allow session (Persist)` sets it to `core.ModeAutoPersist`. Either
way it's `core.SetSessionModeOverride(sessionId, mode)` — a package-level
`sync.Map` in `core/mode.go`, **not** `core.ModeUpsert`, so nothing is
written to `directory.json`: the change is scoped to the rest of this one
`SendChatMessage` call. `approveFunc` (`server/sock/sock.go`) checks
`core.SessionModeOverride(sessionId)` before `core.ModeLookup(root)`, and
`SendChatMessage` calls `core.ClearSessionModeOverride(session.Id)` at its
own entry, so the override can never leak into the next turn — the
directory is back to Plan (or whatever `directory.json` says) as soon as
the next message starts.

The TUI's mode indicator updates live, in the same turn, rather than only on
the next reconnect: `planApprovalTool` takes the same `onTool
func(name, status, message string)` callback `agent_spawn`/`session_send`
already thread through, and calls `onTool(PlanApprovalToolName, "mode",
mode)` right after setting the override. That rides the existing `{type:
"tool", ...}` frame (no new frame type), with `status: "mode"` as a sentinel
the TUI special-cases in `tui_update.go` before the generic
started/finished handling: it just sets `m.mode` and repaints, skipping the
normal tool-result rendering. Since the override is turn-scoped,
`tui_update.go`'s `doneMsg` handler also fires `refreshModeCmd` (a
`GET /api/mode` re-fetch, reusing `fetchMode`/`tuiModeSyncedMsg`) once the
turn ends, so the badge falls back to the real persisted mode (Plan, unless
the operator also hit Shift+Tab) instead of staying on the stale override
color into the next turn.

Anything that doesn't match "once" or "persist"/"session" is treated as
`Deny`, and `Deny` is a real interrupt, not a soft denial: the tool calls
the same cancellation path a `{type:
"interrupt"}` frame does. `core.SetSessionCanceler` (`core/session_router.go`,
the same func-var-hook pattern `SetSessionRouter`/`SetLiveSessionsLister`
use for other things only `server/sock` owns) is registered in
`server/sock/session.go`'s `init()` to a closure over `interruptSession`,
which looks up the session's `context.CancelFunc` in a package-level
`runningTurns sync.Map` (hoisted out of `SockHandler` so it's reachable by
session id regardless of which connection's goroutine is driving the turn)
and calls it. The tool calls that hook, then returns an error; the
`ctx.Err()` check immediately after every tool call in `SendChatMessage`
(`core/chat.go`) catches the now-canceled context and unwinds exactly like a
user-initiated Ctrl+C — no separate "hard abort" path was added anywhere.

`GET /api/sessions/:id/usage` (`SessionContextUsage`, `core/context.go`)
returns `session.last_prompt_tokens` (a session column set from the
provider's own `usage.prompt_tokens + usage.completion_tokens` after each
completed round, alongside `last_cached_tokens` from
`usage.prompt_tokens_details.cached_tokens` when the provider reports it) as
`used`, plus `limit` (80% of `max_context`) and `max_context` itself. Before
a session has any completed round, it falls back to rebuilding the session's
next prompt — stored summary, memory, skills, tool schemas — and returning a
conservative tiktoken estimate instead. `/compact` (`SessionCompact`) resets
both stored counters to 0 so the display falls back to the estimate until
the next real round lands. The TUI only fetches this on demand (`/usage`,
and again after `/compact`) rather than keeping it visible continuously; the
label is `ctx:used/limit (percent)`, plus a `cache:percent` segment when
`cached` is nonzero (`client.ContextLabel`, `modules/client/style.go`).

## The HIL round-trip

When the current mode says "ask," `server/sock`'s `approveFunc` closure
(`server/sock/sock.go`) registers a per-session response channel before it
sends `{type: "approval_request", session_id, cwd, name, arguments}` over the
same `/ws` connection, then blocks on that channel
(`server/sock/session.go`'s `approvalRouter`) until the client answers
`{type: "approval", session_id, decision: "once"|"session"|"deny"}`.
`"session"` also flips an in-memory, session-id-keyed flag
(`sessionAutoApprove`, a package-level `sync.Map`) so the rest of that
session's dangerous calls skip the prompt — it's not written to
`directory.json` and is gone on restart. The client displays the execution
session and directory and echoes the request's session id in its response,
which matters when a nested round belongs to a different session.

`SockHandler`'s read loop can't call `handleFrame` synchronously — that
would deadlock waiting for an approval frame it can only read from the same
loop. It reads continuously in one goroutine, handling `type: "approval"`
(routed to the `approvalRouter`) and `type: "interrupt"` (calls
`interruptSession`, which cancels that session's stored `context.CancelFunc`
— see below) inline, and dispatching everything else to `handleFrame` in its
own goroutine (`go handleFrame(...)`), so an approval or interrupt can
arrive while a turn is mid-stream. Every `conn.WriteJSON` goes through a
`safeConn`'s mutex, since gorilla/websocket doesn't allow concurrent
writers, and a `pingPeriod`/`pongWait` keepalive goroutine drops a dead
connection. A blocked approval wait is tied to a context canceled the moment
the read loop exits (client disconnect), so it resolves to `"deny"` instead
of leaking a goroutine.

`handleFrame` derives a per-turn `context.WithCancel` from the handler
context and stores its `cancel` in a `running sync.Map` keyed by session id
(deleted on return). An `interrupt` frame calls that `cancel`, which
propagates into `SendChatMessage`'s stream and tool calls; the round returns
a context-canceled error, `handleFrame` sends it as an `"error"` frame, and
the client (`render.go`'s `frame`) prints a plain `interrupted` line rather
than an error when the message contains `context canceled`.

`Receive`'s `keys` parameter (`render.go`) exists for a caller that wants to
feed it a background `chan byte` so `renderer.watch` can route bytes to the
approval `y`/`a`/`n` reader (`decide`) while one is pending, and treat a
`0x03` (Ctrl+C) or `0x1b` (Esc) elsewhere as an `{type: "interrupt"}` frame.
`cli/prompt.go` (`-p`) always passes `nil`, so `watch` never actually runs
today; `readByte` falls back to a direct blocking `rawByte()` read of stdin
instead, which is how `-p --format string` answers an approval or question
prompt interactively when one comes up. The TUI has its own key handling
entirely (`modules/tui/tui_update.go`) and never calls into `render.go` at
all. `{type: "tool", ...}` frames are handled by `renderer.tool` — a spinner
while a call is open, a settled
`●` line (with a diff block for a multi-line message) once it finishes.

## `ask_user_question` — the same round-trip, for an actual question

`modules/ask_question` is a single `PermissionSafe` tool — it never goes
through the approval gate above; that gate exists for dangerous side
effects, and asking a question has none. It reuses the HIL round-trip's
plumbing rather than adding a second one: `server/sock/sock.go`'s `askFunc`
registers on the same `approvalRouter` (`server/sock/session.go`), keyed by
session id exactly like `approveFunc`, and sends
`{type: "question_request", session_id, question, options}` instead of an
`approval_request`. The client answers with
`{type: "question", session_id, answer}`; the router's `wait` doesn't know
or care which kind of request is pending, so the one pending-channel map
serves both — safe because `core.SessionLock` already limits a session to
one in-flight tool call at a time. `wait` takes the value to return on
`ctx.Done()` as a parameter now (`"deny"` for approval, `""` for a
question) rather than hardcoding it, since an unanswered question isn't a
"deny."

The tool itself never talks to `server/sock` directly — `core/tools.go`'s
`buildTools` takes an `ask AskFunc` (`core/tool_loop.go`) from
`SendChatMessage` and binds it to the current session id, producing a
`modules.AskFunc` closure that's all `modules/ask_question` ever sees
(mirrors how `modules/web_search`/`modules/web_fetch` take a bound
`func() (*modules.WebBackend, error)` lookup instead of importing `core`, since `core` imports them
and a direct import would cycle). Two distinct fallbacks keep a call from
blocking forever with nobody to answer it: if `ask` itself is `nil` (no
caller wired one — the one production case is `core/context.go`'s
`buildTools` call, made only to size a context window, never to run a
turn), the tool returns an error immediately without ever reaching
`server/sock`. `-p --format json|xml` runs through the real `/ws`/`askFunc`
path like any other turn, so `ask` there is never `nil` — instead
`renderer.collect` (`modules/client/render.go`), the same as it does for
`approval_request`, auto-replies `{type: "question", answer: ""}` the
moment a `question_request` arrives, so the call still returns promptly
with an empty answer rather than an error.

`renderer.ask` (`render.go`) prints the question and, if given options,
a numbered list; it reads one line via the new `renderer.readLine` (Enter
submits, Backspace edits, Ctrl+C aborts to `""`) built on the same
`readByte`/`rawByte` primitive `key()` already used for the single-keystroke
approval prompt. A reply that parses as a valid option number returns that
option's text; anything else is taken verbatim — so a multiple-choice
question still accepts a free-typed answer instead of one of the options.
