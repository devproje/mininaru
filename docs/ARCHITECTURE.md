# Architecture

mininaru is a single Go binary that talks to OpenAI-compatible providers. It
ships two front ends over one core: a terminal chat client and an HTTP API.

## Packages

```
cli/       cobra commands, the bubbletea TUI, and the serve daemon
core/      providers, agents, sessions, messages, the tool-calling loop
modules/   tool implementations and their permission levels
server/    stateless OpenAI-compatible HTTP API
bot/       chat front ends that live inside the daemon (Discord)
config/    client.json preferences (thinking, context budget, tool switch)
util/      data directory layout, SQLite handle, migrations, version info
```

Dependencies point one way: `cli` depends on `server` and `bot`, both of which
depend on `core`, and `core` depends on `modules`, `config`, and `util`. Nothing
in `core` imports its callers, and `server` and `bot` do not import each other —
`cli/serve.go` is the only place that knows about both.

## Two chat paths, one engine

Every front end drives the same tool-calling loop, `completionRun` in
[core/complete.go](../core/complete.go). It streams a response, and while the
model keeps emitting tool calls it executes them and feeds the results back, up
to `maxToolRounds` (8) times.

| | `core.Chat` (TUI, `-p`) | `core.Complete` (server) |
|---|---|---|
| History | loaded from SQLite by session | supplied in the request |
| Persistence | messages and tool calls written | nothing written |
| Tools | `modules.DefaultTools()` | `modules.SafeTools()` |
| Dangerous tools | approval callback, or `--allow-dangerous-tools` | never offered |
| Context budget | `trimHistory` applies | client's responsibility |

The TUI and `-p` share the session-backed path and differ only in their
callbacks. The TUI supplies an approval callback that pauses for a keystroke;
`-p` supplies `nil`, which makes `executeTool` deny dangerous tools and hand the
denial back to the model as a tool error.

`-p` deliberately does not stream to stdout. Because the reply builder resets
per round, only the final round's text is the answer; streaming every round
would put pre-tool chatter in front of it and corrupt piped output. It prints
`message.Content` once, and sends tool and reasoning progress to stderr.

`completionRun.MessageId` is the switch: when empty, `toolCallStart` and
`executeTool` skip their SQL writes, which is what makes the server path
stateless.

Two details of the loop are deliberate and easy to misread:

- The reply builder resets at the start of every round, so a stored message
  holds only the final round's answer, not the chatter a model emits before
  calling a tool.
- The reasoning builder does *not* reset. Reasoning is never replayed to the
  model, so keeping the whole chain across rounds costs nothing and gives the
  user the complete thought process.

## Agents and providers

A `Provider` is an endpoint plus credentials. An `Agent` binds a name, a model,
a persona (`Role` + `Soul`), and a provider. Agents hold a live `*openai.Client`
built from their provider; changing an agent's provider rebuilds it.

`core.Global` is the agent created first and is stored separately from the
`core.Agents` slice — it is *not* a member of that slice. Use `core.AgentAll()`
to iterate everything and `core.AgentByName()` to resolve a name (falling back
to an id). The TUI defaults to the global agent; `--agent <name>` selects
another. The server resolves the request's `model` field the same way, which is
why an OpenAI model picker doubles as an agent picker.

Because of that split, `AgentDefault` swaps the two places: the target leaves
`Agents` and the outgoing global joins it. `AgentDelete` filters `Agents` first
and only then checks whether the target was the global, promoting `Agents[0]` if
so — otherwise deleting the global would strand every remaining agent behind a
"no agent configured" error. It also drops the agent's sessions, since
`sessions.agent_id` has no foreign key to lean on: agents live in JSON, not SQL.

## Storage

Everything lives under `.mininaru/`, or `NARU_PATH` if set. `InitFS` creates the
directory `0700` and chmods an existing one to `0700`, since it holds API keys.

- `provider.json`, `agent.json`, `bot.json`, `client.json` — mode `0600`, written through
  `util.WriteFileAtomic` (temp file in the same directory, then `os.Rename`) so
  a crash mid-write leaves the previous version intact rather than a truncated
  file. Never call `os.WriteFile` on these directly.
- `mininaru.db` — SQLite (modernc, no cgo) with WAL, migrated on open by
  [util/migrations](../util/migrations)

Schema: `sessions` → `messages` → `tool_calls`. A `tool_calls` row hangs off the
**user** message of its turn, not the assistant reply.

A turn is written in two steps so a failure cannot leave an orphan. The user
message is inserted as `pending`; on success one transaction flips it to
`completed` and inserts the assistant reply. On failure it becomes `failed` or
`cancelled` with the error text. Only `completed` messages are replayed.

## History reconstruction

On resume, `historyMessages` rebuilds the OpenAI message sequence from storage:
for each user message it emits the user turn, then — if that turn's tool calls
are all finished — an assistant message carrying the `tool_calls` and one `tool`
message per result. A turn with a `pending` call is replayed without its tool
history, because a `tool_calls` message with no matching result is a protocol
error. Tool history is only reconstructed when tools are enabled for the
request.

`trimHistory` drops whole turns from the oldest end until the remainder fits
`context.max_chars`. It charges tool names, arguments, and results to the
budget, since those are sent. It does not charge reasoning text, which is not.
The budget counts bytes, so non-ASCII text costs more than its character count.

## The daemon and agent instances

`serve` is a long-running process, which changes two assumptions the one-shot
commands could make: config is no longer read once and thrown away, and more
than one caller can hit the same agent at once. `core.Registry` and
`core.Instance` in [core/instance.go](../core/instance.go) exist for exactly
that.

A `Registry` holds one `Instance` per agent name, built from `core.AgentAll()`.
An `Instance` owns its agent, its tool set, and the two ways to talk to it:

- `Complete` — stateless, for the HTTP API
- `Chat` — session-backed, for in-process front ends

Front ends inside the daemon do **not** talk over HTTP or any IPC. They are in
the same process, so they call `Instance.Chat` directly — the same thing the TUI
does with `core.ChatWithApproval`, only with different callbacks. A Discord bot
would map a channel to a session and pass callbacks that edit a Discord message
instead of a terminal buffer.

Every instance gets `modules.SafeTools()`. That is not an HTTP-specific rule but
a daemon-wide one: nothing in a daemon can prompt a human for approval, so
dangerous tools have no safe path. `Chat` passes a `nil` approval callback, so
if a dangerous tool ever is configured it is denied and the denial goes back to
the model as a tool error.

### Session locking

Turns on the *same* session are serialized; different sessions run in parallel.
Without that, two front ends could interleave turns and corrupt the replayed
history. The gate is a buffered channel rather than a `sync.Mutex` so a waiting
caller still honors context cancellation.

The lock table lives on the `Registry`, not the `Instance`, and is deliberately
carried across reloads. `Reload` builds brand-new `Instance` values — that is
how a changed provider takes effect — so if the locks lived on the instance, a
reload mid-turn would hand the next caller a fresh lock and the serialization
guarantee would silently disappear.

The table grows one entry per session id seen and is never pruned. For a
personal daemon that is bounded by the number of sessions; if that ever becomes
a real number, it needs refcounting.

### Reload

`Reload` re-reads `provider.json` and `agent.json` and swaps the instance map
under a write lock. Requests already in flight keep the `*Instance` pointer they
resolved, so they finish against the configuration they started with.
`cli/serve.go` wires `Reload` to `SIGHUP`; `kill -HUP <pid>` picks up
`agent add`, `agent default`, and provider edits without dropping connections.
Because the HTTP API and the bots share one `*core.Registry`, one signal updates
all of them.

### Attaching a conversation to something external

A Discord channel needs a stable session across restarts, so `sessions` carries
`origin` and `external_id` (`0005_session_origin.sql`). A unique **partial**
index on `(origin, external_id)` — restricted to rows where both are non-blank —
enforces one live session per channel while leaving every local TUI session,
which has blank values, free to coexist.

`Instance.Bind` is get-or-create: it returns the channel's existing session if
that session already belongs to this agent, and otherwise calls
`SessionAttach`. `SessionAttach` blanks the previous row's `external_id` and
inserts a new session in one transaction, so switching agents or running
`/reset` starts a clean conversation while the old one keeps its history and
merely stops being the channel's live session.

## Bots

`bot/` holds front ends that run inside the daemon. They are not HTTP clients
and there is no IPC: they hold the same `*core.Registry` and call
`Instance.Chat` directly, exactly as the TUI calls `core.ChatWithApproval`. Only
the callbacks differ.

The Discord reply path is where the platform's constraints live, so they are
isolated in [bot/reply.go](../bot/reply.go):

- Discord's typing state expires, so a ticker refreshes it every eight seconds
  while the model is generating. The completed answer is sent only after the
  typing goroutine has stopped.
- Messages cap at 2000 characters. `splitMessage` counts runes, not bytes, and
  prefers to break on a newline in the second half of a chunk before sending
  the remaining chunks as follow-up messages.

Each incoming message is answered on its own goroutine, and per-session locking
in `Instance.Chat` is what keeps two messages in the same channel from
interleaving.

Which bots exist is configuration, not a flag: `core.Bot` in
[core/bot.go](../core/bot.go) persists to `bot.json` alongside providers and
agents, and `cli/serve.go` starts every enabled entry. `core` stores the bot as
plain data with a `kind` string and never imports `bot/` — `cli/serve.go` is
what turns a `core.Bot` into a running `bot.Discord`, which keeps the dependency
direction intact. The `--discord-*` flags build one throwaway `core.Bot` that
replaces the configured set for that run. If any bot fails to start, the ones
already started are stopped before `serve` returns, so a partial failure does
not leave half a daemon connected.

## Server

`/api/v1` prefix is mandatory — see [CONVENTION.md](CONVENTION.md) §3. Requests
need `Authorization: Bearer <key>`; the key comes from `--api-key` or
`MININARU_API_KEY` and the server refuses to start without one.

Request `content` accepts both a plain string and an array of content parts,
because real clients send both. `stream: true` returns SSE with reasoning
carried as `reasoning_content` deltas. Once the stream has started the status
code is already sent, so a mid-stream failure arrives as an `[error]` content
delta followed by `data: [DONE]`.

## Development

```sh
make build     # -> out/mininaru
make test      # gofmt -l, go vet, go test ./...
make install   # scripts/binary-install.sh
```

`make test` fails on unformatted files, so run it before committing. Follow
[CONVENTION.md](CONVENTION.md): no comments, no `:=`, one `var` block at the top
of each function in first-use order with `err` last, early returns, and
top-level declarations ordered types → consts → vars → functions with the
representative function last.

### Testing patterns

There is no mock provider type. Tests stand up an `httptest.Server` that speaks
SSE and point a provider's `BaseURL` at it — see `toolChunk` in
[core/tool_test.go](../core/tool_test.go) and `upstreamOnce` in
[server/completion_test.go](../server/completion_test.go). Asserting on the
request bodies that fake upstream captured is how the system prompt, tool
exposure, and replayed history get verified.

`core` tests reset the package globals (`Providers`, `Agents`, `Global`) and
call `util.InitFS(t.TempDir())` plus `util.InitDatabase` on a temp file;
`thinkingSetup` does all of it.

`runPrompt` takes its stdout and stderr as `io.Writer` arguments rather than
using `os.Stdout` directly, so a test can assert on both streams. That is also
the cheapest way to exercise the full session-backed chat path end to end,
including tool replay, without a terminal.
