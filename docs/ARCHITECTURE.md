# Architecture

mininaru is a single Go binary: a stateless-per-request OpenAI-compatible HTTP
API backed by SQLite, a matching websocket for streaming chat, and two CLI
front ends — an admin CLI for providers/agents/sessions, and `mininaru shell`,
an interactive terminal that runs a bash prompt and an agent chat over one
line editor.

This is a rewrite in progress (`refactor/1.0.0-alpha`). An earlier version of
this project had skills, memory, subagent delegation, a Discord front end, a
paired gRPC client, and a full-screen TUI. None of that exists in this
branch; it was deliberately dropped in favor of starting the server and CLI
over from a small, well-understood core. If you are looking for any of that,
it is not merely undocumented — it is not built yet. Tool calling (bash, file
read/write/edit, MCP client) has come back — see "Tool calling" below — gated
by a directory-scoped trust model ("yolo mode") and a human-in-the-loop
approval round-trip over `/ws`.

## Packages

```
cli/            cobra root, `serve`, `shell`, and the provider/agent/session admin subcommands
cli/shell/      the `mininaru shell` line editor — bash mode, agent mode, slash commands
core/           Provider, Agent, Session, Message, ToolCall CRUD, the tool-calling chat loop, and yolo trust state
modules/        the Tool/Permission type — a leaf package, imports only util + the MCP SDK
modules/bash/   the bash_exec builtin tool
modules/file/   the file_read/file_write/file_edit builtin tools
modules/mcp/    the MCP client (stdio + streamable-HTTP transports, mcp.json config)
server/         gin HTTP API — OpenAI-compatible /api/v1, REST admin routes under /api, and /ws
util/           data directory layout, SQLite handle + migrations, logging, version/banner
```

Dependencies point one way: `server` and `cli` depend on `core`; `core`
depends on `util` and `modules` (plus `modules/bash`, `modules/file`,
`modules/mcp`); `modules/bash`, `modules/file`, and `modules/mcp` each depend
on `modules` and `util` but not on each other. Nothing in `core`, `modules`,
or `util` imports its callers.
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

## Tool calling — session-backed only

`SendChatMessage` (`core/chat.go`, `core/toolloop.go`), the entry point the
`/ws` handler calls, is a round loop (`maxToolRounds = 8`): it rebuilds the
session's message history via `historyUnion` — replaying each earlier turn's
recorded `tool_calls` back as an assistant tool-call message plus the
matching `openai.ToolMessage` results, so a resumed session doesn't have to
re-run anything — streams a completion, and if the model's response carries
tool calls, executes each one via `executeTool` and loops. Turns with no
`call_id` recorded yet or a `tool_calls` row still `pending` (a turn that was
cut off mid-flight, e.g. by a server restart) are not replayed.

`buildTools(root)` (`core/tools.go`) assembles the tool list every round:
`bash_exec` and the three file tools from `modules/bash`/`modules/file`
rooted at `root`, plus whatever `modules/mcp.Tools()` currently exposes from
`mcp.json`-configured MCP servers. Every `modules.Tool` carries a
`Permission` (`Safe`/`Dangerous`) — builtins are always `Dangerous`, MCP
tools infer it from `ToolAnnotations.ReadOnlyHint` unless a server or
per-tool override in `mcp.json` says otherwise. `executeTool` only consults
`Permission` and the caller-supplied `ApproveFunc`: a `Safe` tool always runs
unconditionally; a `Dangerous` one calls `approve(ctx, name, arguments)` and
runs only if the decision isn't `"deny"`. `core` itself has no opinion on
*when* to ask — that policy lives one layer up, in `server/sock`.

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
dim (the default) for `off`.

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

`cli/shell` mirrors this: `receiveAgent`'s `"approval_request"` case pauses
the ESC-to-interrupt watcher (`interruptWatch.pause()`, `cli/shell/client.go`)
before reading a synchronous y/a/n keypress — two goroutines can't safely
read raw stdin at once — prompts, then writes the decision frame back. The
watcher is not restarted afterward, so ESC-to-interrupt is unavailable for
the rest of that turn once a prompt has fired; nothing typed is lost, since
unread bytes just stay buffered in the terminal until the next `readLine()`.

`/ws` also sends a `{type: "tool", name, status: "started"|"finished"|
"failed"}` frame around each call purely for progress display, unrelated to
approval; `cli/shell` renders it as a `⚙ tool_name` line.

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
  every session over a single connection type. An inbound "message" frame is
  `{session_id, content, cwd}` (`cwd` feeds the yolo anchor); an inbound
  "approval" frame is `{type: "approval", session_id, decision}`. Outbound
  frames are `{type: "chunk"|"tool"|"approval_request"|"done"|"error", ...}`
  — `chunk` carries a completion delta, `tool` reports a call's `name`/
  `status`, `approval_request` carries `name`/`arguments` and blocks the
  turn until an approval frame answers it (see "Tool calling" below).
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

## `cli/shell/` — the interactive shell

`mininaru shell` is a line editor and terminal front end built from scratch
for this rewrite — unrelated to the earlier project's bubbletea TUI. It talks
to a `mininaru serve` instance only over `/api` and `/ws`; nothing in
`cli/shell` imports `server` or touches SQLite directly.

### One editor, two modes

`readLine()` (`input.go`) is the single byte-at-a-time raw-terminal reader
both modes share; `state.mode` only changes which prompt badge is drawn and
what a submitted line dispatches to. Shift+Tab toggles it. Switching into
agent mode with no live connection calls `connect()` lazily and falls back to
"still offline" on failure, so the shell works in bash-only mode with no
server reachable at all — the initial connect at startup is best-effort in
the same way.

### Line editing

- Left/Right move the cursor; typing and backspace act at the cursor
  position, not just at the end of the line.
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
  `$PATH`; anything containing `/`, or not at word-start, is path
  completion. Agent-mode completion, when the word starts with `/`, offers
  the registered slash-command names. Multi-candidate columns are sized with
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

### Agent mode

`sendAgent()` (`client.go`) posts `{session_id, content}` over the websocket
and streams the reply back, rendering `reply.Reasoning` — dimmed, under a
"thinking" heading — ahead of the answer text. While a turn is in flight, a
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

`/help`, `/reset` (start a fresh session against the same agent), `/session`
(show the current session id, agent, and creation time), `/clear`,
`/exit`/`/bash` (back to bash mode), `/yolo <off|persist|on>` (set the
dangerous-tool trust mode for the shell's current directory — see "Tool
calling" above) — a small name-keyed registry (`command.go`), dispatched
only in agent mode when a line starts with `/`.
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
