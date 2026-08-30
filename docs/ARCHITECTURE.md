# Architecture

mininaru is an LLM harness in a single Go binary. `core` holds the agent
runtime — a tool-calling round loop with bash, file, headless-browser, and
MCP tools, per-agent memory, skills, and one-level delegation — and two front
ends put a user in front of it:

- **`mininaru`** (`modules/client/`) — a terminal REPL where every line goes
  straight to the agent; `/bash` and `/!bash` run one shell command on
  request, other slash commands manage the session. `mininaru -p "<prompt>"`
  is the same client wire protocol without the REPL: attach, one message,
  print the reply, exit.
- **`mininaru serve`** (`server/`) — a stateless-per-request
  OpenAI-compatible HTTP API backed by SQLite, plus a `/ws` websocket for
  streaming chat, the approval round-trip, and interrupts.

An admin CLI (`mininaru provider`/`agent`/`session`/`mcp`/`skill`) manages
the SQLite-backed config directly, and `mininaru daemon` runs `serve` as a
per-user background service (`cli/daemon.go`).

Dangerous tools are gated by a directory-scoped trust model ("yolo mode")
with a human-in-the-loop approval round-trip over `/ws`. Delegation is the
`agent_spawn` and `session_send` tools. Memory (`modules/memory`) is a
per-agent markdown store; skills (`modules/skill`) are instruction bundles
the model loads on demand and can also author itself. See "Tool calling"
below.

This is a from-scratch rewrite, currently in the `1.0.0-alpha` series. An
earlier version had a Discord front end, a paired gRPC client, a full-screen
TUI, and a dual-mode `mininaru shell` / `narush` terminal; none of those
exist here.

## Packages

```
cli/            cobra root, `serve`, `daemon`, and the provider/agent/session/mcp/skill admin subcommands
modules/client/ the `mininaru` REPL — line editor, websocket turn loop, streamed-reply renderer, slash commands
core/           Provider, Agent, Session, Message, ToolCall CRUD, the tool-calling chat loop, and yolo trust state
modules/          the Tool/Permission type — a pure leaf package, imports only the standard library
modules/bash/     the bash_exec builtin tool
modules/file/     the file_read/file_write/file_edit builtin tools
modules/browser/  the browser_* computer-use tools (chromedp), the one tool package with cross-call state
modules/mcp/      the MCP client (stdio + streamable-HTTP transports, mcp.json config, `cli/mcp.go` admin CLI)
modules/memory/   the memory_save/memory_read/memory_forget tools over a per-agent markdown store
modules/skill/    skill discovery plus the skill/skill_create tools
server/           gin HTTP API — OpenAI-compatible /api/v1, REST admin routes under /api, and /ws
util/             data directory layout, SQLite handle + migrations, logging, version/banner
```

Dependencies point one way: `server` and `cli` depend on `core`; `core`
depends on `util` and every `modules/*` tool package (`bash`, `file`,
`browser`, `mcp`, `memory`, `skill`); each of those depends on `modules` and
`util` but not on each other. Nothing in `core`, `modules`, or `util` imports
its callers.

`modules/client` is the odd one under `modules/` — not a tool package but a
front end. It imports `core` for its `Agent`/`Session` types and `util`, and
reaches a `mininaru serve` instance only through its public `/api` and `/ws`
surface; it never touches SQLite and nothing in `core` imports it. The admin
subcommands (`provider`, `agent`, `session`) default to the opposite:
`cli/main.go` opens the local SQLite file itself and those commands call
`core/*` directly against the `NARU_PATH` database. Passing `--gateway <name>`
(a saved endpoint from `cli/gateway.go`, `.mininaru/gateways.json`) or a bare
`--url`/`--api-key` flips the read paths (`list`, `show`) and the
`session remove` / `provider remove` / `provider activate` paths to the
remote's `/api` instead, via `cli/remote.go`'s helpers over `client.Api`.
`add` and `set` are always local; `mcp` and `skill` have no `/api` and are
always local.

## Storage

Everything lives under `.mininaru/`, or `NARU_PATH` if set. `util.InitFS`
creates that directory at mode `0700` and tightens an existing one to `0700`
on every start, since it holds API keys.

SQLite (`modernc.org/sqlite`, no cgo) is opened with WAL, `foreign_keys`, and
a five-second busy timeout (`util.databaseDSN`). Migrations are `.sql` files
embedded into the binary (`util/migrations/*.sql`), tracked one row per
applied version in a `migrations` table, and applied inside one transaction
per file on every `NewDatabase` call — `0001_initial_schema.sql`,
`0002_tool_calls.sql`, `0003_skill_uses.sql`.

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
skill_uses(id, skill, scope, path, rel, session_id, call_id, created_at)
  -- one row per `skill` tool call (core/skilluse.go); session_id is indexed
  -- but not a foreign key, so it outlives the session it was logged in
```

Deleting an agent or a session cascades through the foreign keys; nothing in
Go application code has to clean up sessions or messages by hand.

Yolo trust state is the one exception to "everything is SQLite": it lives in
`NARU_PATH/directory.json` — a plain JSON array of `{root, mode, updated_at}`
entries, rewritten whole via `util.WriteFileAtomic` on every change
(`core/yolo.go`), the same pattern `modules/mcp/config.go` uses for
`mcp.json`. It is a flat trust list, not relational data that needs joins or
cascades, so a JSON file is simpler than a table.

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
servers, the three `modules/memory` tools scoped to `caller.Id`, the two
`modules/skill` tools, the `session_list`/`agent_list` discovery pair, and —
only while `depth` hasn't hit its cap — `agent_spawn` and `session_send`
(see "Delegation" below). Every `modules.Tool` carries a `Permission`
(`Safe`/`Dangerous`): `bash_exec`, the file tools, the `browser_*` tools,
`agent_spawn`, and `session_send` are `Dangerous`; `memory_*`,
`skill`/`skill_create`, and `session_list`/`agent_list` are `Safe` (pure
reads, or writes confined to a validated slug under a managed directory).
MCP tools infer it from
`ToolAnnotations.ReadOnlyHint` unless a server or per-tool override in
`mcp.json` says otherwise. `executeTool` only consults `Permission` and the
caller-supplied `ApproveFunc`: a `Safe` tool always runs
unconditionally; a `Dangerous` one calls `approve(ctx, name, arguments)` and
runs only if the decision isn't `"deny"`. `core` itself has no opinion on
*when* to ask — that policy lives one layer up, in `server/sock`.

### MCP servers — `modules/mcp`

`mcp.Init(ctx)`/`mcp.Reload(ctx)` (`client.go`, `Init` is a plain alias for
`Reload`) dial every enabled server in `mcp.json` and populate the in-memory
`shared` manager that `mcp.Tools()` reads from. `cli/serve.go`'s
`serveExecute` calls `mcp.Init` once at startup (a failure is logged as a
warning, not fatal — one broken server shouldn't take the HTTP API down),
and a `watchReload` goroutine alongside it calls `mcp.Reload(ctx)` on
`SIGHUP` (`signal.Notify(syscall.SIGHUP)`), so a running server picks up
config changes without restarting. `Reload` reuses unchanged sessions: a
live session is kept as-is when `fingerprint(&existing.entry) ==
fingerprint(&reloaded.entry)` (a JSON marshal of the whole `Server`
struct), only genuinely-changed or newly-added servers get redialed, and
servers dropped from the config or disabled have their client closed.

`util.Log` (`util/logging.go`) is the shared `log/slog` logger; `cli/main.go`
calls `util.NewLog(util.LogOptions{})` once at startup (default level `info`,
text-or-JSON auto-picked by whether stderr is a terminal). The few
`util.Log.*` calls in `modules/client` are `Debug`-level, below that
threshold, so they stay out of the REPL's interactive output.

`mcp.StatusAll() []Status` (`client.go`, next to `Tools()`) reports, per
*configured* server (`Loaded.Servers`, not just the currently-live ones — so
`cli/mcp.go show`/`list` can display a disabled or never-successfully-dialed
server too): `Enabled`, `Connected`, `Tools` (count), and `Error` (the live
session's dial error, if any). `cli/mcp.go`'s `list`/`show` call
`mcp.Init(ctx)` themselves (so a one-off `mininaru mcp list` invocation — a
separate process from any running `serve` — dials fresh rather than showing
a stale cached state it has no way to see) and `defer mcp.Close()`
afterward, since an MCP CLI subcommand process exiting mid-connection
without a clean `client.Close()` produces a `write EPIPE` on the child
stdio process otherwise.

`cli/mcp.go` mirrors `cli/agent.go`/`cli/provider.go`'s admin-command shape
(`add`/`list`/`show`/`remove`/`enable`/`disable`) but resolves servers by
**name only** (`mcpFind`, a linear scan over `mcp.Loaded.Servers`) since
`Server` has no id, unlike the SQLite-backed `Agent`/`Provider`. `add`'s
`--tool-permission <tool>=safe|dangerous` (repeatable) exposes
`Server.ToolPermission` (`config.go`); a per-tool override always wins over
the server-wide `--permission`.

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

The memory tools are structured — `memory_save` takes `name`/`description`
as separate fields, not raw markdown — so `modules/memory` upserts the
matching `MEMORY.md` line itself on every save/forget. The model never
edits the index directly, so it can't drift out of sync with the topic
files it lists. `LoadIndex(agentId)` caps what's read at session start to
200 lines / 25KB, returns `""` if nothing is saved yet, and
`SendChatMessage` (`core/chat.go`) prepends the result as a `SystemMessage`
right after `agent.Soul` and the skill catalog (see "Skills" below). Topic
files are never preloaded — only `memory_read` fetches one, on demand.

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
replacement only, no incremental append, the same trust level as
`memory_save`.

`modules/skill` keeps no in-memory
cache: `Catalog()`, `Find()`, and `All()` each do a fresh directory scan on
every call, the same choice `modules/memory` makes for `MEMORY.md`. With at
most 64 small bundles this costs nothing per turn and needs no
`Init`/`Reload`/reload-on-SIGHUP subsystem the way a stateful cache would.

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

`agent_spawn` lives in `core` rather than `modules/*` — it needs
`AgentByName`/`SessionCreate`/`MessageCreate`/`SendChatMessage` directly,
which a leaf package can't import without a cycle. `core` builds it, plus
`session_send`, `session_list`, and `agent_list`, by hand rather than
pulling them from a `modules` subpackage. Calling it creates a real
`Session` (named
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
delegate could route around, the tool simply isn't there.

Because the delegate's own streamed content never reaches the caller
(`onChunk` is a no-op in the recursive call — only the final answer comes
back), `agentSpawnTool` sends a few extra `onTool` events by hand so the
delegation doesn't look like a silent multi-round pause: a synthetic
`{name: target.Name, status: "started", message: "spawned by ..., running
independently — <prompt>"}` right before the recursive call, one more
`"finished"`/`"failed"` after it returns, and the delegate's *own* tool
calls forwarded as `{name: target.Name + "/" + toolName, ...}` via a wrapped
`onTool` closure.

`modules/client`'s renderer (`render.go`) turns each `tool` frame into a
one-line status: `"started"` starts a `spinner()` labelled with the tool
name, `"finished"`/`"failed"` stop it and print a settled `●` line (green or
red), and a multi-line `message` (a file diff) is printed under a
`+n -n` header with per-line numbers (`writeDiff`). Nested delegate activity
arrives already namespaced (`worker`, `worker/bash_exec`) from the wrapped
`onTool` closure, so it reads as a flat stream rather than a managed stack.

`session_send` is `agent_spawn`'s sibling: instead of creating a fresh
session for a fresh agent, it injects a message into a session that already
exists — **any** session, including one owned by a different agent than the
caller. The only refusal is the caller's own session, which would deadlock
on its own session lock (below). It reuses `agentSpawnTool`'s
`lastAssistantMessage` helper and the same `depth < maxSpawnDepth` gate in
`buildTools`, and — like `agent_spawn` — runs the nested `SendChatMessage`
with the caller's own `anchor`/`approve` rather than building a second
approval path.

Because a cross-agent injection would otherwise look, from the receiving
session's own history, indistinguishable from that agent's own user typing
a message, `markSenderAgent` (`sessionconnect.go`) prefixes the content
with `[message from agent "<caller>" via session_send]` whenever
`target.AgentId != caller.Id` — a same-agent send (still the common case)
is left untouched, byte for byte. The mirrored copy a live viewer sees
(`mirrorMessage`, below) carries the same marked text the target agent
actually received, not the raw un-marked input, so what a person watching
sees matches what was delivered.

Its `session` argument accepts an id **or a name** — `resolveSessionRef`
tries `SessionRead` first and, only on `sql.ErrNoRows`, falls back to a
name match against `SessionListAll()` (every session, matching
`session_list`'s output). `session_list` shows the model each session's
`Name` alongside its id, and every session has a random `adjective-noun`
name (`core/sessionname.go`), so a model shown `quiet-otter` can pass that
back verbatim. A miss on either path reports `no session %q — check
session_list`, not a raw `sql: no rows in result set`.

Because the target session may have a person watching it live over another
`/ws` connection, two extra pieces exist purely to serve that case:

- **`core.SessionLock(sessionId string) func()`** (`core/sessionlock.go`) —
  a `sync.Map` of per-session `*sync.Mutex`, `Load`-or-`Store`d by id. Every
  place that reads a session's history, appends a new pending message, and
  runs a `SendChatMessage` round holds this lock for the duration:
  `session_send`'s `Execute`, and `server/sock/sock.go`'s `handleFrame` (the
  normal per-frame path). Without it, `session_send` writing into a session
  that a person is concurrently typing into — or two fast frames on the same
  `/ws` connection — could interleave two `historyUnion` reads against the
  same "one pending message" invariant and corrupt the session.
- **The live-connection registry** (`server/sock/session.go`) — a
  `sessionId -> *safeConn` `sync.Map` (`liveConns`, alongside the
  `sessionAutoApprove` map in the same file), populated the moment a session
  is resolved and cleared for a connection's sessions when `SockHandler`'s
  loop exits. `core` can't import `server/sock` (cycle), so the wiring runs
  the other way: `core/sessionrouter.go` exposes
  `SetSessionRouter(messageFn, chunkFn, toolFn, doneFn)`, and
  `server/sock/session.go`'s `init()` calls it once with closures that look a
  session up in `liveConns` and, if present, `writeFrame` the same frame
  shapes `handleFrame` sends. `session_send` calls these hooks
  unconditionally and they are no-ops when nobody's watching: `messageFn`
  right after the injected `Message` is persisted (so the viewer's transcript
  order matches the stored order), `chunkFn`/`toolFn` from the nested round's
  callbacks, and `doneFn` once the round settles — a `"done"` frame on
  success, an `"error"` frame carrying the failure otherwise. The `"message"`
  frame reuses the `Name` field for the *origin* session id and `Message` for
  the injected content.

  A session counts as live from the moment the client connects, not just
  once it sends a message: `Run()` (`modules/client/repl.go`) writes a
  `{"type":"attach","session_id":...}` frame right after dialing, and
  `SockHandler` dispatches `"attach"` to `handleAttach` (`server/sock/sock.go`)
  — a synchronous, no-round path that validates the session exists
  (`core.SessionRead`) and calls the same `registerLiveConn`/`seen.Store`
  pair `handleFrame` uses.

The current `modules/client` REPL reads the socket only inside `Receive`
during an active turn — it does not drain mirrored frames while sitting at
the prompt — so a message injected into the session by `session_send` while
its owner is idle is delivered and persisted, but only rendered on that
person's screen the next time they take a turn (the stored transcript is
correct either way).

`session_list` and `agent_list` (`core/sessiontools.go`) exist so a model
can pick a valid target for the two tools above without being told one in
its prompt: `agent_list` is `AgentList()` unfiltered, and `session_list` is
`SessionListAll()` (**every** session, not just the caller's own agent's)
intersected with the same `liveConns` registry `session_send`'s mirroring
uses, via a second getter set alongside `SetSessionRouter`:
`core.SetLiveSessionsLister(fn func() []string)`, called from the same
`server/sock/session.go` `init()`. Each entry carries an `agent` field (the
owning agent's name, from a one-shot `AgentList()` id→name map built per
call) and a `current` bool (`item.Id == callerSessionId`). Every live
session here is a valid `session_send` target except `current`. Both tools
are `modules.PermissionSafe` — pure reads with no side effects, unlike
`bash_exec`/`file_*`/`browser_*`, which are `PermissionDangerous` because
they touch the filesystem or network.

### Yolo mode — the trust policy behind `approve`

`root` is the **anchor**: for a loopback `/ws` connection it's the client's
reported cwd (the `cwd` field on the chat frame — `modules/client` sends the
process's working directory, captured once at startup); for a non-loopback
connection it's the server process's own `$HOME`, since a remote peer's
claimed cwd can't be trusted. `core.ResolveAnchor` /
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
not part of a live turn. `modules/client`'s `/yolo [off|persist|on]` command
(`cmdYolo`, `command.go`) calls it, and with no argument reads it back with
`GET /api/yolo?cwd=`; `Run()` also calls `cmdYolo(&sh, "")` once at startup
to seed `sh.yolo`. The prompt colours the path segment by that value
(`pathColor`, `style.go`): yellow for `persist`, red for `on`, dim for
`off`. The prompt (`sh.prompt()`, `repl.go`) is two lines — `agent-name
[effort] session-name git:(branch)` then `path ❯` — one string with an
embedded `\n`; `write()` turns `\n` into `\r\n`, and `rowsFor` (`input.go`)
splits on `\n` and sums wrapped-row counts per line so `redraw()`'s
up-then-clear cursor math lands with a multi-row prompt. The `git:(branch)`
segment reads from `sh.cwd` (fixed for the process); `git.go` resolves the
branch by reading `.git/HEAD` directly (following a `.git` *file*'s
`gitdir:` pointer for worktrees/submodules) rather than running `git`, and
reports the branch name or the first 7 hex chars of a detached `HEAD`.
There is deliberately no dirty/staged indicator.

Line one carries the connected agent's `Name` and `ThinkingLevel` (from
`sh.agent`, a `*core.Agent` fetched once via `GET /api/agents` at startup
and replaced whole by `/model`/`/effort`/`/agent`), coloured by level
(`effortColor`, `style.go`: dim `off`, blue `low`, gray `medium`, yellow
`high`, red `max`).

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

`modules/client` reads a single raw stdin stream (`keys`, a
`chan byte` filled by one goroutine in `input.go`). During a turn,
`renderer.watch` (`render.go`) consumes that channel: while
`renderer.awaiting` is set (an approval prompt is up) each byte goes to the
`answers` channel `decide()` reads a `y`/`a`/`n` from; otherwise a `0x03`
(Ctrl+C) or `0x1b` (Esc) sends an `{type: "interrupt"}` frame. Outside a
turn the same channel feeds the line editor. `{type: "tool", ...}` frames
are handled by `renderer.tool` — a spinner while a call is open, a settled
`●` line (with a diff block for a multi-line message) once it finishes.

## `server/` — three route groups, one gin engine

`server.NewAppServer(host string, port uint16, apiKey string)` builds one
`*gin.Engine` and wires:

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
  "approval", session_id, decision}` answers a pending prompt, `{type:
  "interrupt", session_id}` cancels that session's in-flight round, and
  `{type: "attach", session_id}` registers the connection as that session's
  live viewer without running a round. Because an absent field survives a
  `json.Unmarshal` into a reused struct, `SockHandler` zeroes its
  `inboundFrame` on every iteration — otherwise an `attach` would leave its
  type behind and swallow the next chat frame. Outbound frames are
  `{type: "message"|"chunk"|"tool"|"approval_request"|"done"|"error", ...}`
  — `message` echoes a `session_send` injection (`name` is the *origin*
  session id), `chunk` carries a completion delta plus a `reasoning` string,
  `tool` reports a call's `name`/`status`/`message`, `approval_request`
  carries `name`/`arguments` and blocks the turn until an approval frame
  answers it (see "Tool calling" below). Reasoning deltas are pulled out of
  the chunk's raw JSON (`chunkReasoning`), because
  `openai.ChatCompletionChunk` has no typed field for
  `reasoning`/`reasoning_content` and different providers use either key.

All three require `Authorization: Bearer <key>` (`server/auth.go`). The key
is a random 32-byte value generated on first use and stored at
`NARU_PATH/mininaru.key`, mode `0600` (`util.APIKey`, `util/apikey.go`) —
there is no setup step and no separate command to reveal it again later;
reading the file is the only way. `cli/serve.go` calls `util.APIKey()` at
startup and passes it into `NewAppServer`; `modules/client` resolves the key
it sends as `--api-key` flag > `MININARU_API_KEY` env var > (only when the
target `--url` host is loopback) reading that same local file
(`ResolveApiKey`, `client.go`) — a client pointed at a remote `--url` never
reads the local key file, so a locally-generated key cannot leak to whatever
host `--url` happens to name.

## `cli/` — the admin surface

`cli/main.go` reads `NARU_PATH` (default `.mininaru`), calls `util.InitFS`,
opens the database itself, registers every subcommand, and calls
`root.Execute()`. `root.SilenceUsage = true` plus `os.Exit(1)` on a command
error means an ordinary failure like "session not found" prints one clean
`Error: …` line, not a Go stack trace.

`provider` and `agent` (`cli/provider.go`, `cli/agent.go`) each have
`add`/`list`/`show`/`set`/`remove`, plus `provider activate`. `session`
(`cli/session.go`) is read/cleanup only — `list`, `show <id>`, `remove
<id>`. Every subcommand that takes a positional id resolves it through a
small `resolveX(idOrName)` helper that tries reading by id first and falls
back to a name match, so `agent show naru` and `agent show <uuid>` both
work. `session list` defaults to every session (`core.SessionListAll`) and
narrows to one agent with `--agent`. Against a `--gateway`/`--url` remote
these read paths scan the `/api` list instead (the `/api/sessions` route
needs an `agent_id`, so a remote `session list` sweeps every agent). `mcp`
and `skill` are the other two
admin groups — `mcp` in its own section above, `skill` with `list`/`show
<name>`/`uses`.

`cli/serve.go` starts the HTTP server and blocks on `ListenAndServe`.
`cli/daemon.go` is `mininaru daemon install`/`restart`/`uninstall` — it
switches on `runtime.GOOS` to write a `systemd --user` unit (Linux), a
`launchd` agent (macOS), or a per-logon Scheduled Task (Windows) that runs
`mininaru serve` with `NARU_PATH` pinned to the current data directory, and
also pins `export NARU_PATH` into the user's shell rc (`pinNaruPath`) so an
interactive `mininaru` shares that directory. The bare `mininaru` command
(no subcommand) is the REPL: `cli/main.go`'s `execute` calls `shortPrompt`
(`cli/prompt.go`) when `-p` is set, otherwise `client.Run`
(`cli/client.go`).

### `cli/update.go` — self-update

`mininaru update` fetches a release from `.github/workflows/release.yml`'s
output (`mininaru_<tag>_<os>_<arch>.tar.gz`/`.zip` + `SHA256SUMS` +
attestation), verifies the checksum, and replaces the running executable. It
does **not** call GitHub's `/releases/latest` — that endpoint excludes
prereleases, and every `1.0.0-alpha.x` tag is one (`release.yml` marks any
tag containing `-` as `--prerelease`). `updateLatestRelease` hits
`GET /repos/{repo}/releases` (the list endpoint) and takes the newest entry
instead, so "latest" doesn't fall through to an old, incompatible `0.x`
release. The `scripts/install.sh` / `install.ps1` installers additionally
refuse a resolved `0.x` tag unless `--tag` names it explicitly — a guard the
in-binary updater does not have.

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
`modules/client` (only reads it, to print a notice in `banner()`) need —
`cli` is `package main` and can't be imported. `updateCheckStart`, wired
into `root.PersistentPreRunE` in `cli/main.go`, runs a TTL-gated
(`util.UpdateCacheTTL`, 24h) background check on every command except
`update` and `serve` itself, so the notice in `showVersion()` and the REPL
banner are usually a command or two behind rather than triggering a network
call on every invocation.

### `scripts/` — install helpers

Not built or imported by anything; hand-run, and `make install`/`uninstall`
shell out to `install-binary.sh`. Each has a `.sh` (POSIX, for
Linux/macOS) and a `.ps1` sibling.

- `install.sh` / `install.ps1` — resolve the newest release the same way
  `cli/update.go` does (`GET /repos/devproje/mininaru/releases`, first
  entry), download `mininaru_<tag>_<os>_<arch>.{tar.gz,zip}` + `SHA256SUMS`,
  verify, and unpack into `~/.local/bin` (`$BINDIR`/`$PREFIX` to change).
  When `mininaru` is already on `PATH` they hand off to `mininaru update`
  instead. A tag that resolves to `0.x` is refused unless `--tag` names it
  — the one guard the in-binary updater does **not** have. On an interactive
  terminal they offer to run `mininaru daemon install` (the background
  service, which is also what pins `NARU_PATH`); `.ps1` still pins
  `NARU_PATH` as a `User` env var directly since Windows has no shell rc.
- `install-binary.sh` / `.ps1` — install the local `out/` build; the
  target of `make install`. Does not touch `NARU_PATH`.

Registering the background service moved from a `register-daemon` script to
the `mininaru daemon` subcommand (`cli/daemon.go`); the shell-rc pin it
writes uses the `# >>> mininaru env >>>` sentinel block, shared with
`install.sh` so a pin from either side is idempotent.

`ci.yml`'s `scripts` job shellchecks the `.sh` files and parse-checks the
`.ps1` files.

## `modules/client/` — the REPL

`mininaru` is a hand-rolled raw-terminal line editor and streamed-reply
renderer — no TUI framework. It talks to a `mininaru serve` instance only
over `/api` and `/ws`; it imports `core` for `Agent`/`Session` types and
`util`, never `server`, and never touches SQLite.

### One mode

Every submitted line either dispatches a `/command` or is sent to the agent
as a turn — there is no shell mode, no mode toggle. `Run()` (`repl.go`)
resolves the agent (`GET /api/agents`) and session (`GET /api/sessions/:id`
for `--session`, else `POST /api/sessions`), dials `/ws`, sends an `attach`
frame, prints the banner, seeds yolo mode, puts the terminal in raw mode
(`golang.org/x/term`), and runs `loop()`: read a line, record it in
history, and either `dispatch` it or `sh.turn(line)` it. `sh.turn` writes
the chat frame and calls `Receive` to stream the reply; on a write error it
reconnects once and retries the send.

`/gateway` (`gateway.go`) is the one command that re-points the whole shell:
`selectFrom` (`selector.go`, an arrow-key list picker over `sh.keys`) chooses a
saved `Gateway` (passed in via `Options.Gateways`), then a session on it
(sweeping every agent, since `/api/sessions` needs an `agent_id`) or a fresh
one, then `switchGateway` sets `sh.url`/`base`/`apiKey`/`session`/`agent` and
calls `sh.reconnect()` — which already re-dials `sh.url`, re-attaches, and
starts a new `Pump`.

One goroutine owns every `/ws` read for the session's lifetime: `Pump`
(`client.go`) loops `conn.ReadJSON` into a buffered `<-chan Reply` and closes
it when the socket drops. Both `Receive` (during a turn) and `editor.readLine`
(at the prompt) consume that one channel, so a frame is never lost to whichever
side isn't looking — `session_send` from another agent, which the session lock
serialises to only arrive while your session is idle, reaches the editor
instead of sitting in the socket buffer until your next turn. `readLine`
`select`s the frame channel against the key channel; a `session_send` round is
folded into an `ambient` (`ambient.go`) — name, injected prompt, reply deltas,
tool names — and printed as one blue-`┆`-guttered block above a redrawn prompt
when the round's `done`/`error` lands. A closed channel surfaces as `errGone`
from either consumer; `loop()` reconnects and carries on, `turn()` reports it
and the user re-sends.

Non-TTY stdin is refused with a pointer to `-p`. `mininaru -p "<prompt>"`
(`cli/prompt.go`) runs the same wire protocol without the editor: resolve a
session (deleting it on exit unless `--session` was given), dial, `attach`,
send one frame, and `Receive` (fed by its own `Pump`) with a `nil` key stream
so there is no interrupt watcher.

`--format` / `-f` (`string` default, or `json`/`xml`) is checked by
`client.ValidFormat` and threaded into `Receive`. For `string` the read loop
drives `renderer.frame` exactly as the REPL does; for `json`/`xml` it drives
`renderer.collect` instead, which writes nothing to stdout — it accumulates the
content deltas and terminal `tool` statuses, auto-denies `approval_request`
(no TTY to ask), and on `done`/`error` marshals one `client.Result`
(`format.go`) via `marshalResult` and prints it. An `error` frame still prints
the object and returns the error, so the process exits non-zero.

### Platform split — `exec_unix.go` / `exec_windows.go`

Two functions, `setGroup` and `killGroup`, are the only OS-specific surface,
used by `/bash`: unix puts the child in its own process group
(`Setpgid`) and signals the group with `SIGINT`; Windows leaves the group
alone and calls `Process.Kill()`. Split with `//go:build unix` /
`//go:build windows`, the same shape
`modules/bash/bashproc_unix.go`/`bashproc_other.go` uses. Everything else is
cross-platform Go stdlib or `golang.org/x/term`, which has its own
`term_windows.go` (setting `ENABLE_VIRTUAL_TERMINAL_INPUT` so arrow-key
escapes arrive the same way a unix pty sends them).

### Reconnecting

There is no background dialer. `sh.reconnect()` (`repl.go`) dials a fresh
`/ws`, closes the old conn, re-sends the `attach` frame for the **existing**
session id, and starts a new `Pump`. `sh.turn` calls it once on a failed
`WriteJSON` and retries the send; `Receive` and `readLine` call it when the
frame channel closes mid-stream. A `mininaru serve` restart between turns is
invisible; a drop *during* a streamed reply loses that turn and the user
re-sends.

### Line editing

`editor.readLine()` (`input.go`) is a byte-at-a-time raw-terminal reader that
`select`s the shared `keys` channel against the `Pump` frame channel (see "One
mode"). There is no Tab completion and no `@file` expansion — both were
shell-era features.

- Left/Right by character; Ctrl+Left/Right (`ESC [ 1;5 D/C`) and Home/End
  (`ESC [ H/F` or VT220 `ESC [ 1~`/`4~`) by word / line end. Typing and
  backspace act at the cursor.
- Ctrl+A/E line start/end; Ctrl+K kill to end, Ctrl+U kill to start, Ctrl+W
  kill word behind (readline semantics — `wordBoundaryLeft`/`Right` share the
  boundary logic), Ctrl+Y yank the last kill. Ctrl+L clears the screen.
- Ctrl+C returns `errInterrupted` (the loop just re-prompts), Ctrl+D returns
  `io.EOF` (quit).
- `Run` (`repl.go`) writes `ESC [ > 1 u` after `MakeRaw` to push the Kitty
  keyboard protocol (and `ESC [ < u` to pop it on exit). With it on, Shift+Enter
  arrives as CSI-u `13;2u`; `csiUCode` parses any `<code>;<mods>u`, a modified
  Enter (`code == 13`) inserts a literal newline instead of submitting, and a
  Ctrl+letter event — which the protocol also reports as CSI-u — is turned back
  into its control byte and re-fed through `synth`/`haveSynth` so Ctrl+C/D and
  the kill/yank keys keep working. Ctrl+J is the fallback on terminals that
  ignore the push.
- Up/Down recall `sh.history` — one list, since there is only one mode.
  `history.go` loads/saves it to `NARU_PATH/history` (`NARU_HISTFILE` to
  override), trimmed to `HISTSIZE`/`HISTFILESIZE` (default 500);
  `recordHistory` drops consecutive duplicates. There is no `history`
  builtin.
- `redraw()` (`input.go`) strips ANSI to measure the *visible* width of
  `prompt+line` (`displayWidth` is East-Asian-width aware), computes the row
  count it wrapped to, moves up that many rows and clears with `\x1b[0J`
  before reprinting — a plain `\r\x1b[2K` would leave stale wrapped rows once
  a line exceeds the terminal width. `rowsFor` splits the two-line prompt on
  `\n` so the cursor math survives a multi-row prompt.

### Streaming a reply

`Receive` (`render.go`) reads `/ws` frames until a `"done"` or `"error"` and
drives a `renderer`. Content and reasoning deltas go through `renderer.text`,
which switches "mode" between `reasoning` (dimmed, under a `● thinking`
heading) and `content`. On a TTY (`r.rich`), content runs through
`mdRenderer` (`markdown.go`), a dependency-free, line-buffered markdown→ANSI
pass: a line is styled and emitted only once its newline arrives (the
trailing partial is held until the next delta or a `flush()` at close), so
output streams per line, not per token. It handles ATX headings,
`-`/`*`/`+`/`1.`/`1)` list markers (normalised to `•`), blockquotes,
thematic breaks, fenced code blocks (a gutter, no inline processing inside),
inline `` `code` `` (red text) / `**bold**` / `*em*` /
`[text](url)`, and GFM pipe tables. A table can't stream row-by-row — column
widths need every row — so `line()` buffers consecutive `|…|` rows into
`mdRenderer.table` and `drainTable()` renders the block (bold header, a `─`
rule, alignment from the `:---`/`---:`/`:---:` separator) when the table ends
or at `flush()`; a `|…|` run with no separator row is emitted verbatim. Nested
lists are still out of scope. Piped (non-TTY) output skips markdown and prints
raw text.

Interrupt and approval are covered under "The HIL round-trip" above:
`renderer.watch` consumes the shared `keys` channel during a turn, routing
each byte either to the approval `y`/`a`/`n` reader (`decide`) or, on Ctrl+C
/ Esc, to an `{type: "interrupt"}` frame. A `tool` frame with a multi-line
message (a file diff) is printed under a `+n -n` header with per-line numbers
by `writeDiff`.

### `/bash` and `/!bash`

`cmdBash` / `cmdBashQuiet` (`command.go`) run one `$SHELL -c <args>` in
`sh.cwd`, output tee'd to the terminal and to an `strings.Builder`
(`crlfWriter` rewrites `\n`→`\r\n` for raw mode). The child is put in its own
process group (`setGroup`); a `0x03` byte on the shared `keys` channel while
it runs calls `killGroup` (`feedChild`). `/bash` then POSTs the command, exit
status, and captured output (capped at `bashShareLimit`, 8000 bytes) to
`POST /api/sessions/:id/messages` as a `user` message, so the agent reads it
on its next turn; `/!bash` skips that POST. `/help` prints a standing warning
that `/bash` output is recorded in the session.

### Slash commands

A name-keyed registry (`commands`, `command.go`); `dispatch` (`repl.go`)
splits `/<name> <args>` and runs the handler, which takes `*Shell` directly.
`/help`, `/clear`, `/exit` (sets `sh.quit`), `/bash`, `/!bash` (above),
`/session [id-or-name]` (show, or switch — `findSession` tries
`GET /api/sessions/:ref` then a name match against the agent's sessions, then
re-`attach`es), `/agent <id-or-name>` (resolve via `GET /api/agents`,
`POST /api/sessions` for a fresh session on that agent, re-`attach`),
`/model <model>` and `/effort <off|low|medium|high|max>` (PATCH
`/api/agents/:id` via `sh.patchAgent`, which replaces `sh.agent` whole with
the response — against `AgentUpdate`'s "only touch non-empty fields"
semantics, so `{"model": …}` leaves the rest alone), and
`/yolo [off|persist|on]` (see "Yolo mode" above). There is no `/reset`
(`/agent <same>` or a fresh launch covers it) and no persisted client
preferences file.

## Development

```sh
make build      # -> out/mininaru
make fmt        # gofmt -l, fails on unformatted files
make vet        # go vet ./...
make test       # fmt + vet + go test ./... -v
make test-race  # the same suite under the race detector
make test-cover # race + coverage, writes out/coverage.out
make dist GOOS=linux GOARCH=arm64   # cross-compile a release layout into dist/
make install    # out/mininaru -> $BINDIR (default ~/.local/bin); make uninstall to undo
```

`make test-race` is what CI (`.github/workflows/ci.yml`) runs on every push
and pull request, alongside a plain `make build`, a cross-compile check for
`linux/amd64`, `linux/arm64`, and `darwin/arm64`, and a `scripts` job that
shellchecks `scripts/*.sh` and parse-checks `scripts/*.ps1`.
`.github/workflows/release.yml` runs `make dist` for six `GOOS/GOARCH` pairs
on a pushed `v*` tag, archives each, writes `SHA256SUMS`, and attests build
provenance.

Follow [CONVENTION.md](CONVENTION.md) for code style — it is enforced by
`make fmt`/`make test` where it can be, and by review where it can't.
