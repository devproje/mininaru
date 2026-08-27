# Architecture

mininaru is a single Go binary: a stateless-per-request OpenAI-compatible HTTP
API backed by SQLite, a matching websocket for streaming chat, and two CLI
front ends — an admin CLI for providers/agents/sessions, and `mininaru shell`,
an interactive terminal that runs a bash prompt and an agent chat over one
line editor.

This is a rewrite in progress (`refactor/1.0.0-alpha`). An earlier version of
this project had skills, memory, subagent delegation, a Discord front end, a
paired gRPC client, and a full-screen TUI. Most of that still does not exist
in this branch; it was deliberately dropped in favor of starting the server
and CLI over from a small, well-understood core. If you are looking for
the Discord front end, the gRPC client, or the TUI, they are not merely
undocumented — they are not built yet. Four things have come back, all under
a lighter design than the old one: tool calling (bash, file read/write/edit,
browser automation, MCP client), gated by a directory-scoped trust model
("yolo mode") and a human-in-the-loop approval round-trip over `/ws`;
delegation, as the `agent_spawn` and `session_send` tools; persistent
per-agent memory (`modules/memory`); and skills (`modules/skill`), stored
bundles of instructions the model loads on demand and can also author itself.
See "Tool calling" below for all four.

## Packages

```
cli/            cobra root, `serve`, `shell`, and the provider/agent/session admin subcommands
cli/shell/      the `mininaru shell` line editor — bash mode, agent mode, slash commands
core/           Provider, Agent, Session, Message, ToolCall CRUD, the tool-calling chat loop, and yolo trust state
modules/          the Tool/Permission type — a leaf package, imports only util + the MCP SDK
modules/bash/     the bash_exec builtin tool
modules/file/     the file_read/file_write/file_edit builtin tools
modules/browser/  the browser_* computer-use tools (chromedp), the one tool package with cross-call state
modules/mcp/      the MCP client (stdio + streamable-HTTP transports, mcp.json config)
server/           gin HTTP API — OpenAI-compatible /api/v1, REST admin routes under /api, and /ws
util/             data directory layout, SQLite handle + migrations, logging, version/banner
```

Dependencies point one way: `server` and `cli` depend on `core`; `core`
depends on `util` and `modules` (plus `modules/bash`, `modules/file`,
`modules/browser`, `modules/mcp`); those four each depend on `modules` and
`util` but not on each other. Nothing in `core`, `modules`, or `util` imports
its callers.
`cli/shell` reaches a `mininaru serve` instance only through its public
`/api` and `/ws` surface — it is a client like any other, not a special case
with direct database access. The admin subcommands (`provider`, `agent`,
`session`) are the opposite: `cli/main.go` opens the local SQLite file itself
and those commands call `core/*` directly, so they always operate on the
`NARU_PATH` database the CLI process was started against, never on a remote
server the way `serve --url` or `shell --url` can.

## Storage

Everything lives under `.mininaru/`, or `NARU_PATH` if set. `util.InitFS`
creates that directory at mode `0700` and tightens an existing one to `0700`
on every start, since it holds API keys.

SQLite (`modernc.org/sqlite`, no cgo) is opened with WAL, `foreign_keys`, and
a five-second busy timeout (`util.databaseDSN`). Migrations are `.sql` files
embedded into the binary (`util/migrations/*.sql`), tracked one row per
applied version in a `migrations` table, and applied inside one transaction
per file on every `NewDatabase` call — `0001_initial_schema.sql` and
`0002_tool_calls.sql`.

```
providers(id, name, api_key, base_url, active)
  -- a partial unique index on (active) WHERE active = 1 is what makes
  -- ProviderActivate's "deactivate all, activate one" transaction safe
agents(id, name, model, soul, thinking_level, max_context)
  -- thinking_level is CHECKed against off/low/medium/high/max
sessions(id, agent_id REFERENCES agents ON DELETE CASCADE, name, created_at)
messages(id, session_id REFERENCES sessions ON DELETE CASCADE, role, content,
         status, error, created_at)
  -- status is CHECKed against pending/completed/failed/cancelled
tool_calls(id, message_id REFERENCES messages ON DELETE CASCADE, call_id,
           name, arguments, result, status, error, created_at)
  -- status is CHECKed against pending/completed/failed; hangs off the user
  -- message whose turn produced the call, so a session resume can replay it
```

Deleting an agent or a session cascades through the foreign keys; nothing in
Go application code has to clean up sessions or messages by hand.

Yolo trust state is the one exception to "everything is SQLite": it lives in
`NARU_PATH/directory.json` — a plain JSON array of `{root, mode, updated_at}`
entries, rewritten whole via `util.WriteFileAtomic` on every change
(`core/yolo.go`), the same pattern `modules/mcp/config.go` uses for
`mcp.json`. It is a flat trust list, not relational data that needs joins or
cascades, so a JSON file was simpler than a table.

## `core/` — plain CRUD, no ORM

`agent.go`, `provider.go`, `session.go`, `message.go` each hand-build their
SQL with `fmt.Sprintf`, appending an `opts`/`values` pair per non-empty field
so an `Update` call only touches the columns the caller actually set — a
zero-value field on the struct passed to `AgentUpdate`/`ProviderUpdate`/etc.
means "leave this column alone," not "clear it." `ProviderActivate` is the
one write that needs a transaction: it deactivates every provider and
activates the requested one in the same `tx`, which is what the schema's
partial unique index is there to enforce.

`SessionList(agentId)` requires an agent id — that is what the HTTP API's
`GET /api/sessions?agent_id=` needs, since the query param is mandatory
there. `SessionListAll()` is unscoped and exists only for the CLI's
`session list` (which has no such requirement); the HTTP layer has no route
for it.

## `core/chat.go` — completion, streaming only for `/api/v1`

`chatClient` builds a fresh `openai.Client` from `ProviderActive()` on every
call rather than caching one on the agent, so changing which provider is
active takes effect on the very next message. `chatParams`/`chatParamsUnion`
map an agent's stored `ThinkingLevel` to the SDK's `ReasoningEffort`
(`low`/`medium` map directly, `high` and `max` both become `high`, and `off`
— or anything unrecognized — sets nothing, which is the SDK's own default).
`ChatCompletion`/`ChatCompletionStream` (the stateless functions the
`/api/v1/chat/completions` controller calls) never pass tools — that surface
stays completion-only, mirroring how it takes the caller's entire message
history with no server-side session.

`chatStreamRound` (the session-backed path's per-round streaming call) guards
against a provider that stops sending data mid-stream without closing the
connection — a `time.AfterFunc` idle timer (`streamIdleTimeout`, 2 minutes)
resets on every chunk and cancels a context derived from the caller's if it
ever fires, turning what would otherwise be an indefinite "thinking…" hang
into a bounded failure. It only trips on true silence: an actively streaming
response (even one made of nothing but reasoning filler, or a long tool
turn) keeps resetting the timer and is never cut off.

## Tool calling — session-backed only

`SendChatMessage` (`core/chat.go`, `core/toolloop.go`), the entry point the
`/ws` handler calls, is a round loop (`maxToolRounds = 50`): it rebuilds the
session's message history via `historyUnion` — replaying each earlier turn's
recorded `tool_calls` back as an assistant tool-call message plus the
matching `openai.ToolMessage` results, so a resumed session doesn't have to
re-run anything — streams a completion, and if the model's response carries
tool calls, executes each one via `executeTool` and loops. Turns with no
`call_id` recorded yet or a `tool_calls` row still `pending` (a turn that was
cut off mid-flight, e.g. by a server restart) are not replayed.

`buildTools(root, sessionId, caller, depth, onTool, approve)` (`core/tools.go`)
assembles the tool list every round: `bash_exec` and the three file tools
from `modules/bash`/`modules/file` rooted at `root`, the six
`modules/browser` tools scoped to `sessionId` (see below), whatever
`modules/mcp.Tools()` currently exposes from `mcp.json`-configured MCP
servers, the three `modules/memory` tools scoped to `caller.Id` (see below),
the `session_list`/`agent_list` discovery pair, and — only while
`depth` hasn't hit its cap — `agent_spawn` and `session_send` (see
"Delegation" below). Every `modules.Tool` carries a
`Permission` (`Safe`/`Dangerous`) — builtins are always `Dangerous`, MCP
tools infer it from `ToolAnnotations.ReadOnlyHint` unless a server or
per-tool override in `mcp.json` says otherwise. `executeTool` only consults
`Permission` and the caller-supplied `ApproveFunc`: a `Safe` tool always runs
unconditionally; a `Dangerous` one calls `approve(ctx, name, arguments)` and
runs only if the decision isn't `"deny"`. `core` itself has no opinion on
*when* to ask — that policy lives one layer up, in `server/sock`.

### Persistent memory — `modules/memory`

mininaru has no project/git-repo concept to scope memory to the way
Claude Code scopes its own auto-memory to a repository — the only durable
identity in the system is `Agent` (`core/agent.go`), a named persona
already injected into every turn via `agent.Soul`. Memory is scoped to
`agent.Id` for that reason: `root`/`anchor` (`core.ResolveAnchor`,
`core/yolo.go`) was considered and rejected, since it's recomputed from
the client-reported cwd on every inbound message rather than persisted on
`Session`, so keying storage on it would drift if a client's cwd changed
mid-session.

Storage lives under the existing global `.mininaru/` data dir
(`util.RootDir`/`util.Path`, `util/narufs.go`), same tree `directory.json`
(yolo) and `mcp.json` already use:

```
.mininaru/memory/<agent_id>/
├── MEMORY.md       # index, auto-injected into every chat turn for that agent
└── <slug>.md       # topic files: YAML frontmatter (name/description/metadata.type/modified) + markdown body
```

Three `modules.PermissionSafe` tools (`memory_save`, `memory_read`,
`memory_forget`) — safe because they're confined to a validated slug under
a managed directory, never an arbitrary path, the same trust level already
given to `mcp` tools. Frontmatter keeps Claude Code's four-way
`type` taxonomy (`user`/`feedback`/`project`/`reference`), enforced via a
JSON Schema `enum` on `memory_save`'s `type` argument.

One deliberate deviation from Claude Code: there, the model freely edits
`MEMORY.md` itself with a generic file-edit tool, so the index can drift
out of sync with the topic files it lists. mininaru's memory tools are
structured (`memory_save` takes `name`/`description` as separate fields,
not raw markdown), so `modules/memory` upserts the matching `MEMORY.md`
line server-side on every save/forget instead — the model never edits the
index directly, which removes that whole failure mode. `LoadIndex(agentId)`
caps what actually gets read at session start to 200 lines / 25KB (mirroring
Claude Code's own limit), returns `""` if there's nothing saved yet, and
`SendChatMessage` (`core/chat.go`) prepends its result as a `SystemMessage`
right after `agent.Soul` and the skill catalog (see "Skills" below) — topic
files themselves are never preloaded, only fetched on demand via
`memory_read`, same as Claude Code only loading topic files when the model
actually reads them.

### Skills — `modules/skill`

A skill is a folder of instructions the model loads on demand instead of
carrying in every prompt — the same idea as Claude Code's own skills. A
bundle is a directory containing `SKILL.md` (YAML frontmatter `name`/
`description` + a markdown body) and, optionally, companion files (scripts,
references) the model can read or run once it has loaded the bundle.

```
.mininaru/skills/<name>/SKILL.md       # project scope
~/.mininaru/skills/<name>/SKILL.md     # user scope
```

Two `modules.PermissionSafe` tools: `skill` reads a bundle — with no `path`
argument it returns the full `SKILL.md` body plus a listing of companion
files; with `path` it returns one companion file's content, each path
segment validated with `util.SafeSegment` against traversal and hidden
files. `skill_create` writes (or, with `overwrite: true`, replaces) a bundle
from model-supplied `name`/`description`/`body`/`scope` — full-body
replacement only, no incremental append, matching the trust level already
given to `memory_save`.

Unlike `.bak`'s prior implementation, `modules/skill` keeps no in-memory
cache: `Catalog()`, `Find()`, and `All()` each do a fresh directory scan on
every call, the same choice `modules/memory` already made for `MEMORY.md`.
With at most 64 small bundles this costs nothing per turn and removes an
entire subsystem (`Init`/`Reload`/reload-on-SIGHUP) a stateful cache would
otherwise need.

`skill.Catalog()` returns `""` when no skills exist, otherwise a header of
rule text followed by one `name: description` line per skill (capped at
4096 characters), and `SendChatMessage` (`core/chat.go`) prepends it as a
`SystemMessage` right after `agent.Soul`. The rule text is also where the
self-improvement loop lives: rather than a separate background job, the
catalog itself tells the model that after finishing real work it should
call `skill_create` when it used or discovered a reusable multi-step
technique — the skill-side counterpart to `memory_save`'s "feedback" type
for facts and preferences — entirely at the model's own judgment during
normal conversation.

Every `skill` tool call is recorded in the `skill_uses` table
(`core/skilluse.go`, hooked into `core/chat.go`'s tool-call loop) —
`skill, scope, path, rel, session_id, call_id, created_at` — queryable via
`SkillUseStats` / `mininaru skill uses`.

### Computer use — `modules/browser`

`browser_navigate`/`browser_click`/`browser_type`/`browser_read`/
`browser_screenshot`/`browser_close` drive a headless Chrome/Chromium tab via
`github.com/chromedp/chromedp` (pure Go, talks CDP directly over a
websocket — no separate driver process, unlike Playwright). This is the one
tool package with cross-call state: `modules/browser/manager.go` keeps a
`map[sessionId]*session` (a live chromedp context + cancel func), so
`navigate` then `click` then `screenshot` in the same mininaru session act on
the same tab. A lazily-started reaper goroutine (`sync.Once`-gated, so it
never runs if browser tools are never called) closes sessions idle for more
than 5 minutes; `browser_close` lets the model end one early. The Chrome
binary is found via `MININARU_CHROME` (mirroring `MININARU_SHELL` in
`modules/bash`) or `$PATH` (checking `headless-shell`/`chromium-headless-shell`
ahead of the full-browser names — a headless-only build works fine, chromedp
always launches with `--headless` regardless), falling back to chromedp's own
default search; `browser.Available()` is the same check used to skip
`modules/browser`'s integration tests when no Chrome/Chromium is installed.
Browser sessions are in-memory only — they don't survive a server restart,
and a resumed mininaru session just opens a fresh tab on its next
`browser_navigate`.

`newSession()`'s initial `chromedp.Run(ctx)` call — the one that actually
launches the browser and binds the target to `ctx` — runs in a goroutine
bounded by `select`/`time.After(callTimeout)` rather than a
`context.WithTimeout` wrapped around the call itself: chromedp binds a
target to whichever context first runs a successful action on it, so
canceling a context derived for just that one call (even after it
succeeds) poisons the session for every later call sharing its parent. On
timeout only the root `ctx` (and the session) is abandoned, so a stalled
Chrome launch fails fast instead of holding the package-level session
mutex forever and hanging every other session's browser tools with it.

Screenshots hit a real constraint: the OpenAI Chat Completions API's `tool`
message can only carry text (`ChatCompletionToolMessageParam.Content` is
`string | []ChatCompletionContentPartTextParam` — no image parts), while a
`user` message can (`openai.UserMessage([]ChatCompletionContentPartUnionParam{
openai.ImageContentPart(...)})`). So `browser_screenshot` returns the PNG as
a `data:image/png;base64,...` string, and `core/chat.go`'s round loop
(`isScreenshotResult`, `core/toolloop.go`) special-cases any tool result with
that prefix: the `tool_calls` row and the `ToolMessage` both get a short
`"screenshot captured"` placeholder instead of the raw data, and a synthetic
`UserMessage` carrying the image is appended right after — so the model sees
it as an attached image on its next round. The image itself is never
persisted to SQLite (avoids blob bloat); a resumed session replays the
placeholder text only, not the picture.

### Delegation — `core/agentspawn.go`, `core/sessionconnect.go`

`agent_spawn` is the one built-in tool that lives in `core` rather than
`modules/*` — it needs `AgentByName`/`SessionCreate`/`MessageCreate`/
`SendChatMessage` directly, which a leaf package can't import without a
cycle. It's also the only tool `core` builds by hand instead of pulling in
from a `modules` subpackage. Calling it creates a real `Session` (named
`"spawn: <prompt preview>"`) and a `Message` for the target agent, then
recurses into `SendChatMessage` for that session with the same `anchor` and
`approve` the caller has — a dangerous tool call inside the delegate prompts
for approval exactly like one at the top level, routed through the same
`/ws` connection since `approve` is a closure already bound to the parent
session id. The delegate starts with no memory of the calling conversation;
the prompt has to carry everything it needs. The tool's result is the
delegate's last assistant message, read back with `MessageList` once
`SendChatMessage` returns.

Depth is capped at one level via a `depth int` threaded through
`SendChatMessage` and `buildTools` (`core/tools.go`): `buildTools` only
appends `agent_spawn` to the tool list when `depth < maxSpawnDepth`, so a
delegate's own tool list never includes it — not a permission check the
delegate could route around, the tool simply isn't there. Same shape as
this session's own Explore/Plan subagents not carrying an `Agent` tool.

Because the delegate's own streamed content never reaches the caller
(`onChunk` is a no-op in the recursive call — only the final answer comes
back), `agentSpawnTool` sends a few extra `onTool` events by hand so the
delegation doesn't look like a silent multi-round pause: a synthetic
`{name: target.Name, status: "started", message: "spawned by ..., running
independently — <prompt>"}` right before the recursive call, one more
`"finished"`/`"failed"` after it returns, and the delegate's *own* tool
calls forwarded as `{name: target.Name + "/" + toolName, ...}` via a wrapped
`onTool` closure.

`cli/shell/client.go`'s `renderFrame` tracks these as a stack
(`renderState.toolStack`), not just a log: `"started"` pushes a name and
(re)starts a `spinner()` for whatever is now on top, `"finished"`/`"failed"`
pop their name, print a settled `✔`/`✖` line, and restart the spinner for
whatever's left — so nested activity
(`agent_spawn` → `worker` → `worker/bash_exec`) shows as one
line spinning at a time, innermost first, with each level's settled line
staying in scrollback once it completes. `"started"`'s `Message` (the
delegation blurb) is printed once, plainly, right before its spinner starts.

`session_send` is `agent_spawn`'s sibling: instead of creating a fresh
session for a fresh agent, it injects a message into a session that already
exists, gated to sessions owned by the *same* agent as the caller
(`target.AgentId != caller.Id` is refused, as is targeting the caller's own
session — that would deadlock on its own session lock below). It reuses
`agentSpawnTool`'s `lastAssistantMessage` helper and the same
`depth < maxSpawnDepth` gate in `buildTools`, and — like `agent_spawn` —
runs the nested `SendChatMessage` with the caller's own `anchor`/`approve`
rather than building a second approval path.

Because the target session may have a person watching it live over another
`/ws` connection, two extra pieces exist purely to serve that case:

- **`core.SessionLock(sessionId string) func()`** (`core/sessionlock.go`) —
  a `sync.Map` of per-session `*sync.Mutex`, `Load`-or-`Store`d by id. Every
  place that reads a session's history, appends a new pending message, and
  runs a `SendChatMessage` round now holds this lock for the duration:
  `session_send`'s `Execute`, and `server/sock/sock.go`'s `handleFrame` (the
  normal per-frame path). Without it, `session_send` writing into a session
  that a person is concurrently typing into — or two fast frames on the same
  `/ws` connection — could interleave two `historyUnion` reads against the
  same "one pending message" invariant and corrupt the session.
- **The live-connection registry** (`server/sock/session.go`) — a
  `sessionId -> *safeConn` `sync.Map` (`liveConns`, alongside the existing
  `sessionAutoApprove` map in the same file), populated the moment a session
  is resolved and cleared for a connection's sessions when `SockHandler`'s
  loop exits. `core` can't import `server/sock` (cycle), so the wiring runs
  the other way: `core/sessionrouter.go` exposes
  `SetSessionRouter(messageFn, chunkFn, toolFn, doneFn)`, and
  `server/sock/session.go`'s `init()` calls it once with closures that look a
  session up in `liveConns` and, if present, `writeFrame` the same frame
  shapes `handleFrame` already sends. `session_send` calls these hooks
  unconditionally and they are no-ops when nobody's watching: `messageFn`
  right after the injected `Message` is persisted (so the viewer's transcript
  order matches the stored order), `chunkFn`/`toolFn` from the nested round's
  callbacks, and `doneFn` once the round settles — a `"done"` frame on
  success, an `"error"` frame carrying the failure otherwise. The `"message"`
  frame reuses the existing `Name` field for the *origin* session id and
  `Message` for the injected content.

  Registration used to happen lazily, only inside `handleFrame` once a real
  chat frame for that session was processed — so a shell that had connected
  but never sent a message wasn't "live" yet, even though its socket was
  open and idle. `connect()` (`cli/shell/client.go`) now writes a
  `{"type":"attach","session_id":...}` frame right after dialing, and
  `SockHandler` dispatches `"attach"` to `handleAttach` (`server/sock/sock.go`)
  — a synchronous, no-round path that just validates the session exists
  (`core.SessionRead`) and calls the same `registerLiveConn`/`seen.Store`
  pair `handleFrame` uses, so a session counts as live from the moment the
  shell connects.

A mirrored round only reaches a person if their shell is actually reading the
socket, and a shell sitting at its prompt used to read nothing until the next
time it sent something — mirrored frames piled up in the buffer and then
rendered as if they answered whatever the person typed next. Two changes in
`cli/shell` close that:

- The socket is drained by one reader goroutine per connection
  (`readFrames`, started lazily by `ensureReader`) feeding a buffered
  `chan inbound`. Gorilla makes read errors sticky, so a read deadline cannot
  be used to poll a connection that must survive the poll. All *rendering*
  still happens on the main thread — `renderFrame` and the `renderState` it
  mutates are never touched from the reader goroutine.
- `readLine` polls stdin with the existing `pollStdin` on a 100ms
  `idlePollInterval` instead of blocking. Each idle tick calls `drainMirror`,
  which renders whatever frames have queued and then `redraw`s the prompt and
  the half-typed line underneath them. `sendAgent` calls `awaitMirror` first,
  which blocks until any open mirrored round has rendered its terminal frame,
  so a local round can never start inside someone else's.

`session_list` and `agent_list` (`core/sessiontools.go`) exist purely so a
model can pick a valid target for the two tools above without being told one
in its prompt — `agent_list` is `AgentList()` unfiltered, and `session_list`
is `SessionList(caller.Id)` intersected with the same `liveConns` registry
`session_send`'s mirroring uses, via a second getter set alongside
`SetSessionRouter`: `core.SetLiveSessionsLister(fn func() []string)`, called
from the same `server/sock/session.go` `init()`. Both are
`modules.PermissionSafe` — pure reads with no side effects, unlike
`bash_exec`/`file_*`/`browser_*`, which stay `PermissionDangerous` because
they touch the filesystem or network.

### Yolo mode — the trust policy behind `approve`

`root` is the **anchor**: for a loopback `/ws` connection it's the client's
reported cwd (the `cwd` field on the chat frame — `mininaru shell` already
tracks `state.cwd` for bash execution and just forwards it); for a
non-loopback connection it's the server process's own `$HOME`, since a
remote peer's claimed cwd can't be trusted. `core.ResolveAnchor` /
`core.IsLoopbackAddr` (`core/yolo.go`) make that call from the raw
`RemoteAddr` the request came in on.

`core.YoloLookup(anchor)` (`core/yolo.go`) reads `directory.json` and returns
the most specific (deepest) `{root, mode}` entry covering `anchor` by path
segment — not string prefix, so `/home/user/proj` doesn't match
`/home/user/project2` — defaulting to `"off"` when nothing matches. Three
modes: `off` (always ask), `persist` (auto-run — since tools are rooted at
the anchor, every call is "inside" it by construction), `on` (auto-run
everywhere, no directory check). `core.YoloUpsert(root, mode)` rewrites the
file; `"off"` is stored as an explicit entry rather than a deletion, so a
subdirectory can be locked back down under a more permissive ancestor.
`POST /api/yolo` (`server/controller/yolo.go`) is how a client sets this —
plain REST, not a `/ws` frame, since it's a one-off directory declaration,
not part of a live turn. `mininaru shell`'s `/yolo <off|persist|on>` command
calls it; switching to `on` asks for a local confirmation first
(`confirmPrompt` in `cli/shell/client.go`). `GET /api/yolo?cwd=` reads it back
without changing anything — `mininaru shell` polls this (`refreshYoloMode` in
`cli/shell/client.go`, called on connect and on `cd`, not on every prompt
redraw) and colors the path segment of its prompt by the result
(`pathColor` in `cli/shell/style.go`): yellow for `persist`, red for `on`,
dim (the default) for `off`. The prompt also shows a `git:(branch)` segment
and, in bash mode, a `✗ <code>` segment after a non-zero exit — both read
from cached `state` fields (`gitBranch`, `lastExitCode`) refreshed only when
`cwd` actually changes or a command finishes, never from `prompt()` itself,
since `prompt()` runs on nearly every keystroke redraw and shelling out to
`git` or `stat`-walking on every one would be a stutter. `cli/shell/git.go`
resolves the branch by reading `.git/HEAD` directly (following a `.git`
*file*'s `gitdir:` pointer for worktrees/submodules) rather than running
`git`; it reports the branch name, or the first 7 hex characters of the
commit hash when `HEAD` is detached. There is deliberately no dirty/staged
indicator — that needs `git status`, a subprocess call, which the
per-keystroke redraw budget can't afford.

### The HIL round-trip

When yolo mode says "ask," `server/sock`'s `approveFunc` closure
(`server/sock/sock.go`) sends `{type: "approval_request", name, arguments}`
over the same `/ws` connection and blocks on a per-session channel
(`server/sock/session.go`'s `approvalRouter`) until the client answers
`{type: "approval", session_id, decision: "once"|"session"|"deny"}`.
`"session"` also flips an in-memory, session-id-keyed flag
(`sessionAutoApprove`, a package-level `sync.Map`) so the rest of that
session's dangerous calls skip the prompt — it's not written to
`directory.json` and is gone on restart.

This forced a concurrency change in `SockHandler`: the read loop used to call
`handleFrame` synchronously, which would have deadlocked waiting for an
approval frame it can only read from the same loop. It now reads continuously
in one goroutine, routing `type: "approval"` frames straight to the
`approvalRouter` and dispatching everything else to `handleFrame` in its own
goroutine (`go handleFrame(...)`), so an approval reply can arrive while a
turn is mid-stream. Every `conn.WriteJSON` goes through a `safeConn`'s
mutex, since gorilla/websocket doesn't allow concurrent writers. A blocked
approval wait is tied to a context that's canceled the moment the read loop
exits (client disconnect), so it resolves to `"deny"` instead of leaking a
goroutine.

`cli/shell` mirrors this: `renderFrame`'s `"approval_request"` case pauses
the ESC-to-interrupt watcher (`interruptWatch.pause()`, `cli/shell/client.go`)
before reading a synchronous y/a/n keypress — two goroutines can't safely
read raw stdin at once — prompts, then writes the decision frame back. The
watcher is not restarted afterward, so ESC-to-interrupt is unavailable for
the rest of that turn once a prompt has fired; nothing typed is lost, since
unread bytes just stay buffered in the terminal until the next `readLine()`.

`/ws` also sends a `{type: "tool", name, status: "started"|"finished"|
"failed"}` frame around each call purely for progress display, unrelated to
approval; `cli/shell` turns it into the spinner-and-settled-line stack
described under "Delegation" above — a spinner while a call is open, a `✔`
or `✖ ... failed` line once it settles.

## `server/` — three route groups, one gin engine

`server.NewAppServer(host, port)` builds one `*gin.Engine` and wires:

- **`/api`** (`server/api.go`) — REST admin CRUD for agents, providers,
  sessions, and messages, one controller file per resource
  (`server/controller/*.go`), each doing bind → validate → `core` call → JSON.
  `ProviderList`/`ProviderRead`/etc. never return the raw API key; every
  response goes through `toProviderResponse`, which masks it to
  `sk-t...efgh` before it leaves the process. `POST /api/yolo` is the one
  extra route here that isn't resource CRUD — it upserts a yolo trust entry
  (see "Tool calling" below).
- **`/api/v1`** (`server/openai.go`) — the OpenAI-compatible surface:
  `POST /chat/completions` (streaming SSE or a single JSON body) and
  `GET /models`. The `model` field of a chat request names a **mininaru
  agent** by its `Name`, resolved with `core.AgentByName` — not an upstream
  model string — so `GET /models` lists configured agents.
- **`/ws`** (`server/sock/sock.go`) — one generic websocket that multiplexes
  every session over a single connection type. Inbound frames are dispatched
  by their `type`: a chat frame carries no type at all and is
  `{session_id, content, cwd}` (`cwd` feeds the yolo anchor), `{type:
  "approval", session_id, decision}` answers a pending prompt, and `{type:
  "attach", session_id}` registers the connection as that session's live
  viewer without running a round. Because an absent field survives a
  `json.Unmarshal` into a reused struct, `SockHandler` zeroes its
  `inboundFrame` on every iteration — otherwise an `attach` would leave its
  type behind and swallow the next chat frame. Outbound frames are
  `{type: "message"|"chunk"|"tool"|"approval_request"|"done"|"error", ...}`
  — `message` echoes a `session_send` injection (`name` is the *origin*
  session id), `chunk` carries a completion delta, `tool` reports a call's
  `name`/`status`, `approval_request` carries `name`/`arguments` and blocks
  the turn until an approval frame answers it (see "Tool calling" below).
  Reasoning deltas are pulled out of the chunk's raw JSON (`chunkReasoning`),
  because `openai.ChatCompletionChunk` has no typed field for
  `reasoning`/`reasoning_content` and different providers use either key.

All three require `Authorization: Bearer <key>` (`server/auth.go`). The key
is a random 32-byte value generated on first use and stored at
`NARU_PATH/mininaru.key`, mode `0600` (`util.APIKey`, `util/apikey.go`) —
there is no setup step and no separate command to reveal it again later;
reading the file is the only way. `cli/serve.go` calls `util.APIKey()` at
startup and passes it into `NewAppServer`; `cli/shell` resolves the key it
sends per connection as `--api-key` flag > `MININARU_API_KEY` env var >
(only when the target host is loopback) reading that same local file —
a shell pointed at a remote `--url` never reads the local key file, so a
locally-generated key cannot leak to whatever host `--url` happens to name.

## `cli/` — the admin surface

`cli/main.go` reads `NARU_PATH` (default `.mininaru`), calls `util.InitFS`,
opens the database itself, registers every subcommand, and calls
`root.Execute()`. `root.SilenceUsage = true` plus `os.Exit(1)` on a command
error is deliberate: earlier this called `panic(err)`, so an ordinary failure
like "session not found" printed a Go stack trace instead of one clean
`Error: ...` line.

`provider`, `agent`, and `session` each follow the same shape
(`cli/provider.go`, `cli/agent.go`, `cli/session.go`): `add`, `list`, `show`,
`set`, `remove`, plus `provider activate`. Every subcommand that takes a
positional id resolves it through a small `resolveX(idOrName)` helper that
tries reading by id first and falls back to a name match, so `agent show
naru` and `agent show <uuid>` both work. `session list` defaults to every
session (`core.SessionListAll`) and narrows to one agent with `--agent`.

`cli/serve.go` starts the HTTP server and blocks on `ListenAndServe`.
`cli/shell.go` is a thin wrapper that builds `shell.Options` from flags and
calls `shell.Run`.

### `cli/update.go` — self-update

`mininaru update` fetches a release from `.github/workflows/release.yml`'s
output (`mininaru_<tag>_<os>_<arch>.tar.gz`/`.zip` + `SHA256SUMS` +
attestation, unchanged by this rewrite), verifies the checksum, and replaces
the running executable. It intentionally does **not** call GitHub's
`/releases/latest` — that endpoint excludes prereleases, and every
`1.0.0-alpha.x` tag is one (`release.yml` marks any tag containing `-` as
`--prerelease`). `updateLatestRelease` hits `GET /repos/{repo}/releases`
(the list endpoint) and takes the newest entry instead, so "latest" doesn't
fall through to an old, incompatible `0.x` release once one exists again.

The download is staged to a temp file next to the target executable, hashed
while downloading, and only extracted after the checksum matches
(`updateDownloadArchive` → `updateExtractTarGz`/`updateExtractZip`, chosen
by `updateAssetExt()`'s `runtime.GOOS` check — the two extractors themselves
take the binary's expected filename as a parameter rather than reading
`runtime.GOOS` internally, so tests can exercise the zip path on a Linux
runner). Replacing the file is also OS-dependent, split into two directly
testable functions rather than one branch: `updateReplaceUnix` is a plain
`os.Rename` over the running binary, which POSIX allows; `updateReplaceWindows`
renames the running `.exe` aside to `<name>.exe.old` first (Windows refuses to
overwrite an open file, but allows renaming one), moves the staged build into
place, then best-effort removes the `.old` file.

`util/update.go` holds the half both `cli` (writes `update.json`) and
`cli/shell` (only reads it, to print a notice in `banner()`) need — `cli` is
`package main` and can't be imported by `cli/shell`. `updateCheckStart`,
wired into `root.PersistentPreRunE` in `cli/main.go`, runs a TTL-gated
(`util.UpdateCacheTTL`, 24h) background check on every command except
`update` and `serve` itself, so the notice in `showVersion()` and the shell
banner are usually a command or two behind rather than triggering a network
call on every invocation.

## `cli/shell/` — the interactive shell

`mininaru shell` is a line editor and terminal front end built from scratch
for this rewrite — unrelated to the earlier project's bubbletea TUI. It talks
to a `mininaru serve` instance only over `/api` and `/ws`; nothing in
`cli/shell` imports `server` or touches SQLite directly.

### One editor, two modes

`readLine()` (`input.go`) is the single byte-at-a-time raw-terminal reader
both modes share; `state.mode` only changes which prompt badge is drawn and
what a submitted line dispatches to. Shift+Tab toggles it. Switching into
agent mode with no live connection connects lazily and falls back to
"still offline" on failure, so the shell works in bash-only mode with no
server reachable at all — the initial connect at startup is best-effort in
the same way.

### Platform split — `tty_unix.go` / `tty_windows.go`

Four functions (`pollStdin`, `setForeground`, `runForeground`, `enableAnsi`)
are the only OS-specific surface in `cli/shell` — everything else is
already cross-platform Go stdlib or a dependency that handles both itself
(`golang.org/x/term.MakeRaw` has its own `term_windows.go`, setting
`ENABLE_VIRTUAL_TERMINAL_INPUT` so arrow-key escapes arrive the same way a
unix pty sends them). Split with `//go:build unix` / `//go:build windows`,
same shape `modules/bash/bashproc_unix.go`/`bashproc_other.go` already uses
for process-group kill vs. plain `Process.Kill()`. Unix's `pollStdin` is a
`poll(2)` on `POLLIN`; Windows' is `windows.WaitForSingleObject` on the
stdin handle (safe to treat any signal as "a key is ready" because
`ENABLE_MOUSE_INPUT`/`ENABLE_WINDOW_INPUT` are off by default and
`MakeRaw` never turns them on). `setForeground`/`runForeground` are unix's
POSIX foreground-process-group handoff (`SysProcAttr{Setpgid: true}` plus
ignoring `SIGTTOU`/`SIGTTIN` around the transfer) versus a plain
`cmd.Start()`/`cmd.Wait()` on Windows, where a child inheriting the console
reads keyboard input directly with no such concept. `enableAnsi` is a
Windows-only addition — `MakeRaw` only sets *input* mode, so rendering the
ANSI color/cursor codes `style.go` writes needs
`ENABLE_VIRTUAL_TERMINAL_PROCESSING` set on the *output* handle separately;
`Run()` (`shell.go`) calls it once, right after `MakeRaw`, and it is a
no-op on unix.

`exec.go`'s `bashPath`/`switchUser` don't need build tags — no syscalls,
just `runtime.GOOS`. `bashPath` dispatches to `bashPathUnix`
(`$SHELL`/`/bin/bash`) or `bashPathWindows` (`$COMSPEC`/`cmd.exe`);
`shellInvokeFlag` picks `-c` or `/C` to match. `switchUser` (the `su`/`sudo`
parser behind `escalate`) returns unmatched on Windows unconditionally —
there is no POSIX-style user-switch binary there, and Windows' UAC
elevation is a different enough mechanism that it was left out rather than
half-ported.

### Reconnecting

Neither of those failures is final: whenever there is no connection, the
shell keeps dialing on its own. A dial costs up to `DIAL_TIMEOUT` plus
`openSession`'s HTTP round-trips, so running it inline would freeze typing on
every attempt — instead `startDial` (`client.go`) hands a snapshot of the
fields a dial needs (`dialConfig`) to a goroutine and takes the answer back
as a `dialResult` on a channel. Nothing in `state` is touched off the main
thread; `openSession`, `pickAgent`, and `seedAgent` were changed to take that
snapshot and *return* the agent name rather than assigning `state.name`,
which is what made them safe to call from the dial goroutine at all.

`readLine`'s idle tick — the same 100ms `pollStdin` beat that drains mirrored
frames — calls `retryConnect`, which adopts a finished dial if one has landed
and starts the next one once `state.retryAt` has passed. `armRetry` doubles
`state.retryDelay` from `RETRY_MIN` to a `RETRY_MAX` ceiling on every
failure, and failed attempts print nothing at all: a notice per attempt would
bury the prompt within a minute. A successful adopt resets the backoff,
re-sends the `attach` frame for the **existing** session id (so the
conversation survives a server restart), re-reads yolo mode, and prints one
`reconnected` line.

`disconnect` records whether the drop happened in agent mode
(`state.wasAgent`) before dropping to bash mode, and `adoptDial` restores
that mode on success — a `mininaru serve` restart reads as a pause rather
than as being kicked back to a bash prompt. An explicit Shift+Tab while a
background dial is in flight waits for that dial instead of starting a
second one.

### Line editing

- Left/Right move the cursor by one character; Ctrl+Left/Right (`ESC [
  1;5 D/C`) and Home/End (`ESC [ H/F`, or the VT220-style `ESC [ 1~`/`ESC [
  4~`) move by word or to the start/end of the line. Typing and backspace
  act at the cursor position, not just at the end of the line.
- Ctrl+A/E jump to the start/end of the line. Ctrl+K kills from the cursor
  to end-of-line, Ctrl+U kills from start-of-line to the cursor (readline
  semantics — it used to unconditionally clear the whole line regardless of
  cursor position; that changed), Ctrl+W kills the word behind the cursor,
  and Ctrl+Y yanks whatever the last kill put in `state.killBuffer` back in
  at the cursor. Ctrl+L clears the screen and reprints the current line.
  `wordBoundaryLeft`/`wordBoundaryRight` (`input.go`) share the same
  whitespace-boundary logic between word-movement and Ctrl+W.
- Up/Down recall history, and recall is **kept separate per mode** —
  `state.history` for bash, `state.agentHistory` for agent
  (`historyFor(sh)` picks the right one) — so bash commands and chat lines
  never show up in each other's recall.
- `history` (`history.go`) is a GNU-bash-compatible builtin: a bare listing,
  `history N`, `-c` (clear), `-d offset` (delete; a negative offset counts
  from the end), `-w`/`-r` (write/read `HISTFILE`, default
  `.mininaru/shell_history`), honoring `HISTSIZE`/`HISTFILESIZE`/`HISTFILE`.
  Only bash history persists to disk; agent history lives only for the
  current process.
- Every full redraw (backspace, Ctrl+U, history recall, cursor movement)
  goes through `redraw()` (`redraw.go`), which strips ANSI escapes to measure
  the *visible* width of `prompt+line`, works out how many terminal rows that
  occupied, moves up that many rows, and clears with `\x1b[0J` before
  reprinting. A plain `\r\x1b[2K` (clear only the current row) leaves stale
  wrapped content on screen once a line is longer than the terminal width —
  that was a real, reported bug before this existed.
- Tab completion (`complete.go`): bash-mode, word-start position offers the
  builtins (`cd`, `exit`, `quit`, `history`) plus everything executable on
  `$PATH`; agent-mode completion, when the word starts with `/`, offers the
  registered slash-command names. Completing the **second** word in bash
  mode, when the first word is a key in `subcommandSets` (a small hardcoded
  map — `git`, `go`, `npm`, `docker`, `cargo`), offers that program's known
  subcommands instead of falling into path completion — e.g. `git ` + Tab
  lists `add`/`commit`/`push`/etc. This is deliberately shallow: first-level
  subcommand names only, no flags, no argument-aware completion like
  branch names for `git checkout`; anything past that falls through to
  ordinary path completion. Everything else containing `/`, or not at
  word-start, is path completion. Multi-candidate columns are sized with
  `displayWidth`, which is East-Asian-width aware, not raw byte/rune count.

### Multiline input

Two independent triggers feed the same continuation loop in `Run()`
(`shell.go`), both signaled from `readLine` as a sentinel error rather than a
normal return:

- **Automatic, bash mode only** — after Enter, `continueLine()`
  (`multiline.go`) shells out to `bash -n -c <accumulated text>` and checks
  stderr for `unexpected EOF`/`unexpected end of file`, or checks for an
  unescaped trailing backslash; either one means "not done yet," and it keeps
  reading more lines under a `> ` prompt until the accumulated text parses.
- **Manual, either mode** — Ctrl+J (a literal `\n` byte, distinct from the
  `\r` a physical Enter key sends) ends the current line without submitting.
  `Run()` accumulates it into `composing` and keeps reading under the same
  `> ` prompt until a real Enter. Shift+Enter, when a terminal encodes it as
  the Kitty-protocol CSI-u sequence `13;2u`, is recognized as an alias for
  the same path, but mininaru never activates that protocol itself —
  turning it on broke Ctrl+C/Ctrl+D/Ctrl+U on terminals with partial
  support for it, so Ctrl+J is the one path guaranteed to work everywhere.

Ctrl+C during either kind of continuation (`state.continuation == true`)
aborts the whole in-progress multi-line entry, not just the current segment.

### Running bash

`runBash`/`runNested` (`exec.go`) run `bash -c <line>` — or, for `su`/`sudo`,
re-exec `mininaru shell` itself as the target user, carrying `--session` and
`--agent` over so a privilege switch does not lose the conversation — through
`runForeground()` (`tty.go`). The child is placed in its own process group
and handed the terminal's foreground process group via `TIOCSPGRP`
(`SIGTTOU`/`SIGTTIN` ignored around the handoff, standard job-control
practice), then the shell reclaims the foreground group when the child
exits. Without this, a `Ctrl+C` meant for a running child would land on the
whole process group and kill the mininaru shell along with it, since both
would otherwise share one group.

`runBash` records the child's exit code into `state.lastExitCode`
(`exitCode()`, reading `cmd.ProcessState.ExitCode()` after `Wait` returns)
for the prompt's `✗ <code>` segment, and only prints its own red error
notice when the failure is **not** a plain non-zero exit
(`errors.As(err, &exitErr)` against `*exec.ExitError`) — a command that
simply returned non-zero (`false`, a no-match `grep`) is silent, the way a
real shell is; only a genuine failure to run the command at all (bash
itself couldn't start, for instance) gets the notice.

### Agent mode

`sendAgent()` (`client.go`) posts `{session_id, content, cwd}` over the
websocket and streams the reply back, rendering `reply.Reasoning` — dimmed,
under a "thinking" heading — ahead of the answer text. `isReasoningFiller`
drops any reasoning delta that's nothing but dots and whitespace before
rendering it — some providers stream literal `.` characters as a heartbeat
while a reasoning summary is still being generated, instead of holding the
delta back until there's real content; the header only appears once real
text arrives. While a turn is in flight, a
background goroutine (`watchInterrupt`) polls stdin so pressing Esc cancels
the wait: it closes the websocket, then reconnects **reusing the existing
session id** (`connect()` only calls `openSession()` — which creates a new
session — when `state.session` is still empty), and hands control back to
the prompt. Polling is used instead of `SetReadDeadline` because raw-mode
stdin does not support read deadlines on this platform; the poll goes
through `golang.org/x/sys/unix.Poll` on the raw fd instead.

Any other byte typed while that watcher is running — i.e., while a response
is streaming — is captured rather than discarded, and replayed into the next
`readLine()` call through `state.pendingInput` once the turn ends. An earlier
version of this simply read and dropped those bytes, which made ordinary
keys — including Ctrl+C and Ctrl+U — appear to silently stop working
whenever they landed during, or immediately after, a streaming response.

`watchInterrupt` and `sendAgent` are wrapped in `interruptWatch`
(`client.go`), which exists for one reason: a tool-approval prompt
(`approvalPrompt`) also reads raw stdin synchronously, and two goroutines
can't safely read the same fd at once. `interruptWatch.pause()` stops the
watcher (idempotent — safe to call from both the approval case and
`sendAgent`'s own cleanup) before that read happens, and it is deliberately
not restarted, so ESC stops interrupting for the rest of that turn once an
approval prompt has fired in it.

### Slash commands

`/help`, `/reset` (start a fresh session against the same agent, or the one
set with `/agent` if any), `/session` (show the current session id, agent,
and creation time), `/agent <global|current> <id-or-name>` (set
`state.agent` — the default agent `/reset` and a freshly opened connection
pick; `resolveAgentByIdOrName` in `client.go` tries `GET /agents/<id>`
first, then falls back to a name match against `GET /agents`, the same list
`pickAgent` already used. Both scopes set `state.agent` for the running
process; `global` additionally persists the choice via
`cli/shell/preferences.go` to `.mininaru/shell.json`, so `Run()` loads it
back into `state.agent` on the next `mininaru shell` launch whenever
`--agent` wasn't passed explicitly, while `current` leaves that file alone
so the change doesn't outlive this shell), `/clear`,
`/bash` (back to bash mode), `/exit` (quit the shell — `quitShellCommand`
returns `commandResult{Quit: true}`, and `dispatchCommand` turns that into
`io.EOF`, the same sentinel bash-mode `exit`/`quit` return to break
`Run()`'s loop), `/yolo <off|persist|on>` (set the dangerous-tool trust mode
for the shell's current directory — see "Tool calling" above) — a small
name-keyed registry (`command.go`), dispatched only in agent mode when a line
starts with `/`.
Handlers take `*state` directly and read/write it in place; there is no
adapter interface here, because there used to be — this lived in a separate
`cli/cmd` package for a short time and was folded back into `cli/shell` once
the indirection was not paying for itself.

## Development

```sh
make build      # -> out/mininaru
make fmt        # gofmt -l, fails on unformatted files
make vet        # go vet ./...
make test       # fmt + vet + go test ./... -v
make test-race  # the same suite under the race detector
make test-cover # race + coverage, writes out/coverage.out
make dist GOOS=linux GOARCH=arm64   # cross-compile a release layout into dist/
```

`make test-race` is what CI (`.github/workflows/ci.yml`) runs on every push
and pull request, alongside a plain `make build` and a cross-compile check
for `linux/amd64`, `linux/arm64`, and `darwin/arm64`. `.github/workflows/release.yml`
runs `make dist` for six `GOOS/GOARCH` pairs on a pushed `v*` tag, archives
each, writes `SHA256SUMS`, and attests build provenance.

Follow [CONVENTION.md](CONVENTION.md) for code style — it is enforced by
`make fmt`/`make test` where it can be, and by review where it can't.
