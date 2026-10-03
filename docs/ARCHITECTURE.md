# Architecture

mininaru is an LLM harness in a single Go binary. `core` holds the agent
runtime — a tool-calling round loop with bash, file, headless-browser, and
MCP tools, per-agent memory, skills, and one-level delegation — and two front
ends put a user in front of it:

- **`mininaru`** (`modules/tui/`, built on the shared wire protocol in
  `modules/client/`) — a full-screen terminal chat client where every
  message goes straight to the agent; `/bash` and `/!bash` run one shell
  command on request, other slash commands manage the session, and pasting
  or dragging in an image attaches it. `mininaru -p "<prompt>"` is the same
  client wire protocol without the TUI: attach, one message, print the
  reply, exit.
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
TUI, and a dual-mode `mininaru shell` / `narush` terminal. The full-screen
TUI came back as `modules/tui` — a different, lighter bubbletea-based
design, not the old one; the rest don't exist here.

## Packages

```
cli/            cobra root, `serve`, `daemon`, and the provider/webprovider/agent/session/mcp/skill admin subcommands
modules/client/ shared wire protocol (Frame/Reply, HTTP + websocket helpers), the classic `-p` streamed-reply renderer and markdown formatter
modules/tui/    the `mininaru` full-screen TUI (bubbletea) — chat viewport, async slash commands, clipboard/drag-drop image paste
core/           Provider, WebProvider, Agent, Session, Message, ToolCall CRUD, the tool-calling chat loop, and yolo trust state
modules/          the Tool/Permission/WebBackend types — a pure leaf package, imports only the standard library
modules/bash/     the bash_exec builtin tool
modules/file/     the file_read/file_write/file_edit builtin tools
modules/browser/  the browser_* computer-use tools (chromedp), the one tool package with cross-call state
modules/web_search/ the web_search tool (Brave/Tavily/Ollama Search backends)
modules/web_fetch/  the web_fetch tool (SSRF-guarded direct fetch, or Tavily extract)
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

`modules/client` and `modules/tui` are the odd ones under `modules/` — not
tool packages but front ends. `modules/client` imports `core` for its
`Agent`/`Session` types and `util`, and reaches a `mininaru serve` instance
only through its public `/api` and `/ws` surface; it never touches SQLite
and nothing in `core` imports it. `modules/tui` imports `modules/client` for
that same wire protocol and layers the bubbletea UI on top of it; nothing
outside `cli/` imports `modules/tui`. The admin subcommands (`provider`,
`agent`, `session`) default to the opposite:
`cli/main.go` opens the local SQLite file itself and those commands call
`core/*` directly against the `NARU_PATH` database. Passing `--gateway <name>`
(a saved endpoint from `cli/gateway.go`, `.mininaru/gateways.json`) or a bare
`--url`/`--api-key` flips the read paths (`list`, `show`) and the
`session remove` / `provider remove` / `agent primary` paths to the
remote's `/api` instead, via `cli/remote.go`'s helpers over `client.Api`.
`add` and `set` are always local. MCP is also reachable over `/api/mcp`
(`server/controller/mcp.go`) — the same config CRUD the `mcp` CLI does, each
mutation followed by `mcp.Reload` so a remote server's tool set changes
without a shell on the box. `/api/skill` (`server/controller/skill.go`,
read-only) and `/api/agents/:id/memory` (`server/controller/memory.go`,
read/write/delete, wrapping new `memory.List`/`Read`/`Write`/`Delete`) round
out the surface — `skill`'s local CLI stays as-is.

## Storage

Everything lives under `.mininaru/`, or `NARU_PATH` if set. `util.InitFS`
creates that directory at mode `0700` and tightens an existing one to `0700`
on every start, since it holds API keys.

SQLite (`modernc.org/sqlite`, no cgo) is opened with WAL, `foreign_keys`, and
a five-second busy timeout (`util.databaseDSN`). Migrations are `.sql` files
embedded into the binary (`util/migrations/*.sql`), tracked one row per
applied version in a `migrations` table, and applied inside one transaction
per file on every `NewDatabase` call — `0001_initial_schema.sql`,
`0002_tool_calls.sql`, `0003_skill_uses.sql`, `0004_attachments.sql`,
`0005_session_cwd.sql`, `0006_session_summaries.sql`,
`0007_agent_selected_provider_active_removal.sql`,
`0008_session_last_prompt_tokens.sql`,
`0009_session_last_cached_tokens.sql`.

```
providers(id, name, api_key, base_url)
  -- every registered provider is always usable; there is no "active" one
agents(id, name, model, soul, thinking_level, max_context, selected)
  -- thinking_level is CHECKed against off/low/medium/high/max
  -- model is "<provider-name>:<model>"; a partial unique index on
  -- (selected) WHERE selected = 1 is what makes AgentSelect's
  -- "deselect all, select one" transaction safe. exactly one agent is
  -- selected once at least one agent exists (the first ever created
  -- auto-selects)
sessions(id, agent_id REFERENCES agents ON DELETE CASCADE, name, created_at,
         cwd, last_prompt_tokens, last_cached_tokens)
  -- last_prompt_tokens/last_cached_tokens cache the provider's own usage
  -- figures from the session's last completed round for the /usage
  -- endpoint; both reset to 0 on compact
messages(id, session_id REFERENCES sessions ON DELETE CASCADE, role, content,
         status, error, created_at)
  -- status is CHECKed against pending/completed/failed/cancelled
tool_calls(id, message_id REFERENCES messages ON DELETE CASCADE, call_id,
           name, arguments, result, status, error, created_at)
  -- status is CHECKed against pending/completed/failed; hangs off the user
  -- message whose turn produced the call, so a session resume can replay it
skill_uses(id, skill, scope, path, rel, session_id, call_id, created_at)
  -- one row per `skill` tool call (core/skill_use.go); session_id is indexed
  -- but not a foreign key, so it outlives the session it was logged in
attachments(id, session_id REFERENCES sessions ON DELETE CASCADE,
            message_id REFERENCES messages ON DELETE SET NULL, mime, bytes,
            path, created_at)
  -- uploaded chat images; the bytes live at .mininaru/attachments/<id>, only
  -- the metadata is in SQLite. message_id is NULL between upload and the turn
  -- that references it. deleting a session drops the rows but not the files
   -- (ponytail: orphaned files, add a sweep if it matters)
session_summaries(session_id REFERENCES sessions ON DELETE CASCADE, content,
                  through_message_id REFERENCES messages ON DELETE CASCADE,
                  updated_at)
  -- one rolling summary per session; raw messages remain in the database and
  -- through_message_id marks the last message replaced in model requests
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
means "leave this column alone," not "clear it." `AgentSelect` is the
one write that needs a transaction: it deselects every agent and selects
the requested one in the same `tx`, which is what the schema's partial
unique index is there to enforce.

`SessionList(agentId)` requires an agent id — that is what the HTTP API's
`GET /api/sessions?agent_id=` needs, since the query param is mandatory
there. `SessionListAll()` is unscoped and exists only for the CLI's
`session list` (which has no such requirement); the HTTP layer has no route
for it.

## `core/chat.go` — completion, streaming only for `/api/v1`

`chatClient` builds a fresh `openai.Client` from the provider `resolveProviderModel`
looks up on every call rather than caching one on the agent, so editing an
agent's `model` (`<provider>:<model>`) takes effect on the very next message.
`chatParams`/`chatParamsUnion` map an agent's stored `ThinkingLevel` to the
SDK's `ReasoningEffort`
(`low`/`medium` map directly, `high` and `max` both become `high`, and `off`
— or anything unrecognized — sets nothing, which is the SDK's own default).
`ChatCompletion`/`ChatCompletionStream` (the stateless functions the
`/api/v1/chat/completions` controller calls) never pass tools — that surface
stays completion-only, mirroring how it takes the caller's entire message
history with no server-side session.

Images ride the OpenAI content-parts shape. `imageUserMessage`
(`core/tool_loop.go`) turns a text string plus a list of image URLs / data
URIs into a `openai.UserMessage([]…ContentPartUnionParam{…})` — the same
form the browser-screenshot tool path already used. The stateless path fills
`ChatMessage.Images` from `parseOpenAIContent` (a `content` field that is
either a JSON string or a `[{type,text},{type,image_url}]` array); the
session-backed path's `historyUnion` calls `messageImages(msg.Id)`, which
reads each bound `attachments` row off disk and base64-inlines it as a
`data:` URI (a provider can't reach the mininaru server, so URLs are never
passed through). `POST /api/sessions/:id/attachments` and the `/ws` frame's
`images` array both carry attachment ids, bound to the message row by
`core.AttachmentBindMessage`.

`max_context` is an agent's input-context budget in tokens (24,000 by
default). mininaru reserves 20% for the model's output, then estimates the
serialized prompt conservatively before every provider round — this
pre-flight tiktoken estimate (`contextTokenEstimate`, `core/context.go`) is
separate from the `used` figure `GET /api/sessions/:id/usage` reports (see
below), which is the provider's own real usage once a round has completed.
Session-backed turns that exceed the remaining input budget summarize old
completed turns into `session_summaries`, keeping the current turn and the
newest completed turn as raw OpenAI messages. Tool calls and their results
are folded together; the raw history is never deleted. If the current raw
turn, system context, or tool schema still exceeds the budget, the turn
fails before a provider call.
The stateless `/api/v1/chat/completions` surface never rewrites caller-supplied
history: it returns a `context_length_exceeded` error instead.

`chatStreamRound` (the session-backed path's per-round streaming call) guards
against a provider that stops sending data mid-stream without closing the
connection — a `time.AfterFunc` idle timer (`streamIdleTimeout`, 2 minutes)
resets on every chunk and cancels a context derived from the caller's if it
ever fires, turning what would otherwise be an indefinite "thinking…" hang
into a bounded failure. It only trips on true silence: an actively streaming
response (even one made of nothing but reasoning filler, or a long tool
turn) keeps resetting the timer and is never cut off.

Every request also carries `stream_options.include_usage` so the provider
returns real `usage` figures (see the usage endpoint below), and, unless
disabled, an Anthropic-style `cache_control` breakpoint (`cacheRequestOptions`,
`core/chat.go`) on the last message of the conversation's fixed leading
system-message run (summary, then memory, skills, soul — whichever comes
last) so a provider that supports prompt caching can reuse that unchanging
prefix across rounds and turns. Both are injected via
`option.WithJSONSet` since the OpenAI Go SDK's typed params have no field
for either. `--no-cache` (root CLI flag) stores nothing — it puts a marker on
the request's `context.Context` (`core.WithNoCache`) that `server/sock`
sets from the `Frame`/`inboundFrame`'s `no_cache` bit for that one turn.

## Tool calling — session-backed only

`SendChatMessage` (`core/chat.go`, `core/tool_loop.go`), the entry point the
`/ws` handler calls, is a round loop (`maxToolRounds = 50`): it rebuilds the
session's message history via `historyUnion` — replaying each earlier turn's
recorded `tool_calls` back as an assistant tool-call message plus a matching
`openai.ToolMessage` per call — so a resumed session doesn't have to re-run
anything — streams a completion, and if the model's response carries tool
calls, executes each one via `executeTool` and loops. Turns with no
`call_id` recorded yet or a `tool_calls` row still `pending` (a turn that was
cut off mid-flight, e.g. by a server restart) are not replayed.

Only the *current*, in-flight turn's tool results are replayed verbatim.
Every earlier turn's `tool_calls.result` is replaced with a short
`replayToolResult` placeholder (`[tool result omitted from history, N
bytes — re-run the tool if you need it again]`) instead of the original
text — a bash or file-read result can be tens of thousands of bytes, and
replaying it in full on every subsequent round was the main way a
tool-heavy session burned through its context budget. The model can always
re-run the tool if it needs the content again.

Failed tools retain both their error and any returned output: the error stays
in `tool_calls.error`, while `tool_calls.result` and the current
`openai.ToolMessage` contain an `error:` header followed by the output. This
lets the model diagnose a failed command in the next round and preserves the
same details when a session is resumed. A canceled turn records the current
tool and pending message as failed, then stops before another tool call or
completion round can start.

`buildTools(root, sessionId, caller, depth, onTool, approve, ask)` (`core/tools.go`)
assembles the tool list every round: `bash_exec` and the three file tools
from `modules/bash`/`modules/file` rooted at `root`, the six
`modules/browser` tools scoped to `sessionId` (see below), `web_search`
(`modules/web_search`) and `web_fetch` (`modules/web_fetch`),
`ask_user_question` (`modules/ask_question`, bound to `sessionId` and
`ask` — see "`ask_user_question`" below), whatever `modules/mcp.Tools()`
currently exposes from `mcp.json`-configured MCP servers, the three
`modules/memory` tools scoped to `caller.Id`, the two `modules/skill`
tools, the `session_list`/`agent_list` discovery pair, and — only while
`depth` hasn't hit its cap — `agent_spawn` and `session_send` (see
"Delegation" below). Every `modules.Tool` carries a `Permission`
(`Safe`/`Dangerous`): `bash_exec`, the file tools, the `browser_*` tools,
`agent_spawn`, and `session_send` are `Dangerous`; `memory_*`,
`skill`/`skill_create`, `session_list`/`agent_list`, `web_search`,
`web_fetch`, and `ask_user_question` are `Safe` (pure reads, or writes
confined to a validated slug under a managed directory).
`web_search`/`web_fetch` read outbound-only —
`web_fetch` additionally resolves the target host itself and refuses any
address `net.IP` classifies as loopback, private, link-local, multicast, or
unspecified before dialing, closing the SSRF gap that would otherwise argue
for `Dangerous`. MCP tools infer it from
`ToolAnnotations.ReadOnlyHint` unless a server or per-tool override in
`mcp.json` says otherwise. `executeTool` only consults `Permission` and the
caller-supplied `ApproveFunc`: a `Safe` tool always runs
unconditionally; a `Dangerous` one calls
`approve(ctx, sessionId, root, name, arguments)` and runs only if the decision
isn't `"deny"`. `core` itself has no opinion on
*when* to ask — that policy lives one layer up, in `server/sock`.

`bash_exec` drains combined stdout and stderr into one synchronized bounded
buffer while the command runs. It captures at most 64 KiB including a
`[truncated]` suffix, but continues consuming later output so a noisy child
cannot grow process memory or block on a full pipe. A timeout or canceled
context returns the captured output together with its context error.

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
threshold, so they stay out of the TUI's interactive output.

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
(`core/skill_use.go`, hooked into `core/chat.go`'s tool-call loop) —
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
(`isScreenshotResult`, `core/tool_loop.go`) special-cases any tool result with
that prefix: the `tool_calls` row and the `ToolMessage` both get a short
`"screenshot captured"` placeholder instead of the raw data, and a synthetic
`UserMessage` carrying the image is appended right after — so the model sees
it as an attached image on its next round. The image itself is never
persisted to SQLite (avoids blob bloat); a resumed session replays the
placeholder text only, not the picture.

### Web search & fetch — `modules/web_search`, `modules/web_fetch`

`web_search` and `web_fetch` are separate from `modules/browser`: no
chromedp, no JS execution, no browser tab. Both read the currently selected
backend through `core.WebProviderSelected()` — `web_providers` is a table
shaped like `providers` (`kind`, `api_key`, `base_url`) plus a `selected`
column with the same unique-partial-index-backed single-selection pattern
`agents.selected` uses (`core/agent.go`'s `AgentSelect`/`AgentSelected`;
`providers.active` tried this shape once and was dropped unused in
`0007_agent_selected_provider_active_removal.sql` — `web_providers` reuses
the pattern the project kept, not the one it abandoned). `core/tools.go`'s
`resolveWebBackend` adapts a `*core.WebProvider` into the leaf
`modules.WebBackend{Kind, ApiKey, BaseUrl}` struct so `modules/web_search`
and `modules/web_fetch` don't need to import `core` (which would cycle,
since `core/tools.go` imports them). `mininaru webprovider add/list/set/
primary/remove` manages the table; `api_key` is encrypted at rest the same
way `providers.api_key` is (`util.Encrypt`/`util.Decrypt`).

`web_search`'s `kind` selects the backend at call time: `brave` (GET
`api.search.brave.com`, `X-Subscription-Token` header), `tavily` (POST
`api.tavily.com/search`), or `ollama` (POST `ollama.com/api/web_search`,
bearer token) — each mapped to a common `title`/`url`/`snippet` shape before
formatting. `web_fetch` defaults to fetching the URL directly; if the
selected backend is `tavily` it calls `api.tavily.com/extract` instead,
since that handles JS-rendered pages a raw GET can't. The direct path is an
SSRF-guarded `http.Client`: `guardedDialContext` resolves the host itself,
rejects the request if any resolved `net.IP` is loopback, private,
link-local, multicast, or unspecified, and dials the validated IP directly
rather than the hostname — resolving once and reusing that address closes
the DNS-rebinding gap between the check and the connection. An HTML
response is rendered to plain text via `browser.HTMLToText` (exported from
`modules/browser/htmldoc.go` for this reuse) rather than duplicating that
renderer.

### Delegation — `core/agent_spawn.go`, `core/session_connect.go`

`agent_spawn` lives in `core` rather than `modules/*` — it needs
`AgentByName`/`SessionCreate`/`MessageCreate`/`SendChatMessage` directly,
which a leaf package can't import without a cycle. `core` builds it, plus
`session_send`, `session_list`, and `agent_list`, by hand rather than
pulling them from a `modules` subpackage. Calling it creates a real
`Session` (named `"spawn: <prompt preview>"`) with the caller's anchor stored
as its `Cwd`, and a `Message` for the target agent, then recurses into
`SendChatMessage` for that session. A dangerous tool call inside the delegate
prompts over the same `/ws` connection, but approval and yolo lookup use the
spawned session id and its inherited working directory rather than the parent
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
`buildTools`. The nested `SendChatMessage` uses the target session's persisted
`Cwd`; an empty target `Cwd` therefore exposes no filesystem or Bash tools.
Approvals still travel over the initiating caller's `/ws` connection, but
carry the target session id and working directory.

Because a cross-agent injection would otherwise look, from the receiving
session's own history, indistinguishable from that agent's own user typing
a message, `markSenderAgent` (`session_connect.go`) prefixes the content
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
name (`core/session_name.go`), so a model shown `quiet-otter` can pass that
back verbatim. A miss on either path reports `no session %q — check
session_list`, not a raw `sql: no rows in result set`.

Because the target session may have a person watching it live over another
`/ws` connection, two extra pieces exist purely to serve that case:

- **`core.SessionLock(ctx, sessionId)` and `core.SessionTryLock(sessionId)`**
  (`core/session_lock.go`) — a `sync.Map` of per-session channel semaphores,
  `Load`-or-`Store`d by id. Normal WebSocket turns wait with their connection
  context, while `session_send` immediately returns a busy error when another
  turn owns its target. Every
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
  the other way: `core/session_router.go` exposes
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
  once it sends a message: `RunTuiSession` (`modules/tui/tui_session.go`)
  writes a `{"type":"attach","session_id":...}` frame right after dialing,
  and `SockHandler` dispatches `"attach"` to `handleAttach`
  (`server/sock/sock.go`) — a synchronous, no-round path that validates the
  session exists (`core.SessionRead`) and calls the same
  `registerLiveConn`/`seen.Store` pair `handleFrame` uses.

The TUI's `pumpTuiReplies` goroutine reads the socket continuously for the
whole connection, not only during an active turn, so a message injected into
the session by `session_send` while its owner is idle is rendered on that
person's screen as soon as it arrives — it does not wait for their next
turn (see "`modules/tui/` — the full-screen TUI" below).

`session_list` and `agent_list` (`core/session_tools.go`) exist so a model
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

Each dangerous tool passes its current session id and root to `approve`, so a
nested `agent_spawn` or `session_send` round cannot inherit a caller's yolo or
session approval merely because the caller initiated it. `core.YoloLookup`
(`core/yolo.go`) reads `directory.json` and returns
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
not part of a live turn. The TUI's `/yolo [off|persist|on]` command calls
it, and with no argument reads it back with `GET /api/yolo?cwd=` (see
"`modules/tui/` — the full-screen TUI" below for how the command itself
runs). There is no always-visible trust indicator — checking means running
`/yolo` with no argument.

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

### The HIL round-trip

When yolo mode says "ask," `server/sock`'s `approveFunc` closure
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

### `ask_user_question` — the same round-trip, for an actual question

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
`WebBackendLookup` instead of importing `core`, since `core` imports them
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
  carries `session_id`/`cwd`/`name`/`arguments` and blocks the turn until an approval frame
  answers it (see "Tool calling" below). Reasoning deltas are pulled out of
  the chunk's raw JSON (`chunkReasoning`), because
  `openai.ChatCompletionChunk` has no typed field for
  `reasoning`/`reasoning_content` and different providers use either key.

All three require `Authorization: Bearer <key>` (`server/auth.go`). The key
is a random 32-byte value generated on first use and stored at
`NARU_PATH/mininaru.key`, mode `0600` (`util.APIKey`, `util/api_key.go`) —
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
`add`/`list`/`show`/`set`/`remove`, plus `agent primary`. `session`
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
`mininaru serve --host --port --cors-origin --web-dir` (`daemonExecArgs`)
with `NARU_PATH` pinned to the current data directory, and also pins
`export NARU_PATH` into the user's shell rc (`pinNaruPath`) so an
interactive `mininaru` shares that directory. `daemon install` with none of
those flags set and a tty attached runs an interactive wizard instead
(`daemonWizard`): it reads the already-installed unit/plist/scheduled task
back into a `daemonPreset` (`linuxExistingConfig`/`darwinExistingConfig`/
`windowsExistingConfig`, sharing a `parseServeArgs`/`shellTokenize` pair) so
a reinstall is pre-filled from what's actually running rather than the flag
defaults, then prints the resulting plan and asks for `y/N`
(`daemonConfirmPlan`) before writing anything. The bare `mininaru` command
(no subcommand) is the TUI: `cli/main.go`'s `execute` calls `shortPrompt`
(`cli/prompt.go`) when `-p` is set, otherwise `clientExecute` (`cli/client.go`),
which calls `tui.RunTuiSession`.

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

`util/update.go` (`UpdateNotice`) is read only from `cli/main.go`'s
`showVersion()`, under `--version` — neither `modules/client` nor
`modules/tui` reads it; there is no notice on a plain `mininaru` launch.
`updateCheckStart`, wired into `root.PersistentPreRunE` in `cli/main.go`,
runs a TTL-gated (`util.UpdateCacheTTL`, 24h) background check on every
command except `update` and `serve` itself, writing the cache
`showVersion()` reads, so the notice is usually a command or two behind
rather than triggering a network call on every invocation.

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

## `modules/client/` — the wire protocol and the `-p` renderer

`modules/client` is the layer both front ends share: `Frame`/`Reply` and the
HTTP/WS helpers (`Api`, `Upload`, `Dial`, `Pump`, `Agent`, `Session`,
`FindSession`, `ResolveApiKey`, `ResolveCwd`, …), plus the markdown→ANSI
formatter and the classic streamed-reply renderer that only `-p` still uses.
It imports `core` for `Agent`/`Session` types and `util`, never `server`, and
never touches SQLite; it reaches a `mininaru serve` instance only over `/api`
and `/ws`.

`mininaru -p "<prompt>"` (`cli/prompt.go`) runs the wire protocol without any
UI: resolve a session (deleting it on exit unless `--session` was given),
dial, `attach`, upload any `--image <path>` (repeatable, via `client.Upload`
— multipart `POST /api/sessions/:id/attachments` before the frame goes out,
the returned ids riding the frame's `images` array), send one frame, and
`Receive` (`render.go`, fed by its own `Pump`) with a `nil` key stream so
there is no interrupt watcher.

`--format` / `-f` (`string` default, or `json`/`xml`) is checked by
`client.ValidFormat` and threaded into `Receive`. For `string` the read loop
drives `renderer.frame`, which prints a streamed transcript: content and
reasoning deltas go through `renderer.text`, switching "mode" between
`reasoning` (dimmed, under a `● thinking` heading) and `content`; on a TTY
(`r.rich`) content runs through `MdRenderer` (`markdown.go`), a
dependency-free, line-buffered markdown→ANSI pass — a line is styled and
emitted only once its newline arrives, so output streams per line, not per
token. It handles ATX headings, `-`/`*`/`+`/`1.`/`1)` list markers
(normalised to `•`), blockquotes, thematic breaks, fenced code blocks (a
gutter, no inline processing inside), inline `` `code` `` / `**bold**` /
`*em*` / `~~strike~~` / `[text](url)`, and GFM pipe tables (buffered row by
row in `MdRenderer.table` since column widths need every row, and drained by
`drainTable()` on the table's end or at `Flush()`). Piped (non-TTY) output
skips markdown and prints raw text. A `tool` frame with a multi-line message
(a file diff) is printed under a `+n -n` header with per-line numbers by
`FormatDiff`/`writeDiff`.

For `json`/`xml`, `Receive` drives `renderer.collect` instead, which writes
nothing to stdout — it accumulates the content deltas and terminal `tool`
statuses, auto-denies `approval_request` (no TTY to ask), and on
`done`/`error` marshals one `client.Result` (`format.go`) via
`marshalResult` and prints it. An `error` frame still prints the object and
returns the error, so the process exits non-zero. Non-TTY stdin to the TUI
(no `-p`) is refused with a pointer to `-p`.

## `modules/tui/` — the full-screen TUI

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

### One mode, and who owns the socket

Every submitted line either dispatches a `/command` or is sent to the agent
as a turn — there is no shell mode, no mode toggle, same as before.
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

### Key handling and focus

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

### Async commands and queued messages

Every network-touching slash command (`/usage`, `/compact`, `/session`,
`/gateway`, `/model`, `/effort`, `/yolo`, `/bash`, `/!bash`) runs through
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

### Attaching an image

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

### `/bash` and `/!bash`

`runLocalBash` (`tui_command.go`) runs one `$SHELL -c <args>` in the
session's `cwd` and captures combined output with a plain `exec.Command`;
unlike the old REPL there is no process-group setup and no kill path — the
child is not interruptible from the TUI once started, and `Ctrl+C` while one
is running only sends the websocket interrupt frame, which has no effect on
it. `/bash` POSTs the command, exit status, and captured output (capped at
`bashShareLimit`, 8000 bytes, via `bashTranscript`) to
`POST /api/sessions/:id/messages` as a `user` message once it finishes, so
the agent reads it on its next turn; `/!bash` skips that POST.

### Slash commands

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
directly via `patchAgent`), `/effort <off|low|medium|high|max>` (same
`patchAgent` path against `/api/agents/:id`), and `/yolo [off|persist|on]`
(see "Yolo mode" above). There is no `/agent` (agent switching happens with
`mininaru agent primary`, outside the TUI) and no `/reset`.

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
