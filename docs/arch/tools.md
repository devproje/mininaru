# Tool calling — session-backed only

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
from `modules/bash`/`modules/file` rooted at `root` — the file tools built
with `unrestricted: true` only when `core.ModeLookup(root) == core.ModeFullAuto`
(see [Mode](approval.md)) — the six `modules/browser` tools scoped to `sessionId`
(see below), `web_search` (`modules/web_search`) and `web_fetch`
(`modules/web_fetch`), `ask_user_question` (`modules/ask_question`, bound to
`sessionId` and `ask` — see [`ask_user_question`](approval.md)), whatever
`modules/mcp.Tools()` currently exposes from `mcp.json`-configured MCP
servers, the three `modules/memory` tools scoped to `caller.Id`, the two
`modules/skill` tools, the `session_list`/`agent_list` discovery pair,
`plan_approval` (`core/plan_approval.go`, see [Escaping Plan mode mid-turn](approval.md)) only while the current mode is `plan`, and — only while `depth`
hasn't hit its cap — `agent_spawn` and `session_send` (see [Delegation](delegation.md)). Every `modules.Tool` carries a `Permission` (`Safe`/`Dangerous`):
`bash_exec`, the file tools, the `browser_*` tools, `agent_spawn`, and
`session_send` are `Dangerous`; `memory_*`, `skill`/`skill_create`,
`session_list`/`agent_list`, `web_search`, `web_fetch`,
`ask_user_question`, and `plan_approval` are `Safe` (pure reads, or writes
confined to a validated slug under a managed directory, or — for
`plan_approval` — a session-scoped in-memory override rather than a
filesystem write).
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

## MCP servers — `modules/mcp`

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

## Persistent memory — `modules/memory`

mininaru has no project/git-repo concept to scope memory to the way
Claude Code scopes its own auto-memory to a repository — the only durable
identity in the system is `Agent` (`core/agent.go`), a named persona
already injected into every turn via `agent.Soul`. Memory is scoped to
`agent.Id` for that reason: `root`/`anchor` (`core.ResolveAnchor`,
`core/mode.go`) was considered and rejected, since it's recomputed from
the client-reported cwd on every inbound message rather than persisted on
`Session`, so keying storage on it would drift if a client's cwd changed
mid-session.

Storage lives under the existing global `.mininaru/` data dir
(`util.RootDir`/`util.Path`, `util/narufs.go`), same tree `directory.json`
(mode state) and `mcp.json` already use:

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

## Skills — `modules/skill`

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

## Platform prompt — `core/platform_prompt.go`

The one system-content source that isn't per-agent and isn't file-backed
under a per-agent directory: a fixed description of what mininaru itself is
(the harness, the mode-gated tool set, the things an agent should keep in
mind), built in as `defaultPlatformPrompt` and overridable by dropping a
`NARU_PATH/CUSTOM_INSTRUCTION.md` — `PlatformPrompt()` reads it with
`os.ReadFile(util.Path("CUSTOM_INSTRUCTION.md"))` and falls back to the
built-in constant on `os.IsNotExist`, the same shape `core.ModeLoad` uses for
`directory.json`. Nothing writes the file; it's hand-authored, read fresh
every turn like `skill.Catalog()`. `SendChatMessage` and
`SessionContextUsage` both prepend it last, so it ends up first in the
message array — before `agent.Soul` — since it's the one layer that's fixed
regardless of which agent or operator customization is in play.

## Computer use — `modules/browser`

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
`modules/bash`), else a configured channel (below), else `$PATH` (checking
`headless-shell`/`chromium-headless-shell` ahead of the full-browser names —
a headless-only build works fine, chromedp always launches with
`--headless` regardless), falling back to chromedp's own default search;
`browser.Available()` is the same check used to skip `modules/browser`'s
integration tests when no Chrome/Chromium is installed. Browser sessions
are in-memory only — they don't survive a server restart, and a resumed
mininaru session just opens a fresh tab on its next `browser_navigate`.

When more than one browser is installed, `modules/browser/config.go`'s
`browser.json` (`{"channel": "chrome"|"chromium"|"edge"|"brave"}`) picks
which one — `mininaru browser set <channel>` and `/api/browser` both just
read/write this file directly, the same relationship `cli/mcp.go`/
`/api/mcp` have with `mcp.json`, and for the same reason there's no reload
step: unlike an MCP server, nothing about a browser channel choice is a
live out-of-process connection to re-dial, so `chromePath()` just reads
`browser.json` fresh the next time a browser session is lazily created.
A channel that's configured but not found on `$PATH` makes `chromePath()`
return `""` directly rather than falling back to the generic search — the
operator asked for a specific browser, so `Available()` reporting false is
less surprising than silently launching a different one. `browser.json`
only ever stores one of the four channel names, never an arbitrary path:
unlike `MININARU_CHROME` (which needs process-level env access), `/api/browser`
is reachable by any API client, so letting it set a path would let a
caller point the server at any binary on disk.

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

## Web search & fetch — `modules/web_search`, `modules/web_fetch`

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
