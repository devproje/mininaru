# Architecture

mininaru is a single Go binary: a stateless-per-request OpenAI-compatible HTTP
API backed by SQLite, a matching websocket for streaming chat, and two CLI
front ends — an admin CLI for providers/agents/sessions, and `mininaru shell`,
an interactive terminal that runs a bash prompt and an agent chat over one
line editor.

This is a rewrite in progress (`refactor/1.0.0-alpha`). An earlier version of
this project had MCP tools, skills, memory, subagent delegation, a Discord
front end, a paired gRPC client, and a full-screen TUI. None of that exists in
this branch; it was deliberately dropped in favor of starting the server and
CLI over from a small, well-understood core. If you are looking for any of
that, it is not merely undocumented — it is not built yet.

## Packages

```
cli/         cobra root, `serve`, `shell`, and the provider/agent/session admin subcommands
cli/shell/   the `mininaru shell` line editor — bash mode, agent mode, slash commands
core/        Provider, Agent, Session, Message CRUD, and chat completion against the active provider
server/      gin HTTP API — OpenAI-compatible /api/v1, REST admin routes under /api, and /ws
util/        data directory layout, SQLite handle + migrations, logging, version/banner
```

Dependencies point one way: `server` and `cli` depend on `core`; `core`
depends on `util`; nothing in `core` or `util` imports its callers.
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
per file on every `NewDatabase` call — there is currently just
`0001_initial_schema.sql`.

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
```

Deleting an agent or a session cascades through the foreign keys; nothing in
Go application code has to clean up sessions or messages by hand.

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

## `core/chat.go` — completion, not a tool loop

`chatClient` builds a fresh `openai.Client` from `ProviderActive()` on every
call rather than caching one on the agent, so changing which provider is
active takes effect on the very next message. `chatParams` maps an agent's
stored `ThinkingLevel` to the SDK's `ReasoningEffort` (`low`/`medium` map
directly, `high` and `max` both become `high`, and `off` — or anything
unrecognized — sets nothing, which is the SDK's own default).

`SendChatMessage` is the persistence-aware entry point the websocket handler
calls: it loads the session's full history via `MessageList`, prepends the
agent's `Soul` as a system message when one is set, locates the `pending`
user message, and streams the completion — each chunk both forwarded to the
caller's callback and appended to a local builder. On success the user
message is marked `completed` and the assembled reply is inserted as a new
assistant message; on a stream error the user message is marked `failed`
with the error text instead.

There is no tool-calling loop here. A request goes in, a completion comes
back. Anything that sounds like "the model can act" — file access, shell
access, web search, delegating to another agent — is not part of this
codebase.

## `server/` — three route groups, one gin engine

`server.NewAppServer(host, port)` builds one `*gin.Engine` and wires:

- **`/api`** (`server/api.go`) — REST admin CRUD for agents, providers,
  sessions, and messages, one controller file per resource
  (`server/controller/*.go`), each doing bind → validate → `core` call → JSON.
  `ProviderList`/`ProviderRead`/etc. never return the raw API key; every
  response goes through `toProviderResponse`, which masks it to
  `sk-t...efgh` before it leaves the process.
- **`/api/v1`** (`server/openai.go`) — the OpenAI-compatible surface:
  `POST /chat/completions` (streaming SSE or a single JSON body) and
  `GET /models`. The `model` field of a chat request names a **mininaru
  agent** by its `Name`, resolved with `core.AgentByName` — not an upstream
  model string — so `GET /models` lists configured agents.
- **`/ws`** (`server/sock/sock.go`) — one generic websocket that multiplexes
  every session over a single connection type. An inbound frame is just
  `{session_id, content}`; the handler creates the user message, calls
  `core.SendChatMessage`, and streams back `{type: "chunk"|"done"|"error",
  ...}`. Reasoning deltas are pulled out of the chunk's raw JSON
  (`chunkReasoning`), because `openai.ChatCompletionChunk` has no typed field
  for `reasoning`/`reasoning_content` and different providers use either key.

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

### Slash commands

`/help`, `/reset` (start a fresh session against the same agent), `/session`
(show the current session id, agent, and creation time), `/clear`,
`/exit`/`/bash` (back to bash mode) — a small name-keyed registry
(`command.go`), dispatched only in agent mode when a line starts with `/`.
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
