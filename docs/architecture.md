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

Dangerous tools are gated by a directory-scoped, four-value mode (Default,
Plan, Auto Persist, Full Auto) with a human-in-the-loop approval round-trip
over `/ws`. Delegation is the
`agent_spawn` and `session_send` tools. Memory (`modules/memory`) is a
per-agent markdown store; skills (`modules/skill`) are instruction bundles
the model loads on demand and can also author itself. See [Tool calling](arch/tools.md).

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
core/           Provider, WebProvider, Agent, Session, Message, ToolCall CRUD, the tool-calling chat loop, and per-directory mode state
modules/          the Tool/Permission/WebBackend types — a pure leaf package, imports only the standard library
modules/bash/     the bash_exec builtin tool
modules/file/     the file_read/file_write/file_edit builtin tools
modules/browser/  the browser_* computer-use tools (chromedp), the one tool package with cross-call state, browser.json channel config + `cli/browser.go` admin CLI
modules/web_search/ the web_search tool (Brave/Tavily/Ollama Search backends)
modules/web_fetch/  the web_fetch tool (SSRF-guarded direct fetch, or Tavily extract)
modules/mcp/      the MCP client (stdio + streamable-HTTP transports, mcp.json config, oauth login for http servers with no manual bearer header, `cli/mcp.go` admin CLI)
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
without a shell on the box. `/api/browser` (`server/controller/browser.go`)
is the same relationship for the browser channel choice, minus the reload
step (see [Computer use](arch/tools.md)). `/api/skill`
(`server/controller/skill.go`, read-only) and `/api/agents/:id/memory`
(`server/controller/memory.go`, read/write/delete, wrapping new
`memory.List`/`Read`/`Write`/`Delete`) round out the surface — `skill`'s
local CLI stays as-is.

## Deep dives

- [Storage and core CRUD](arch/storage.md) — SQLite schema, `core/` CRUD
- [`core/chat.go`](arch/chat.md) — completion and streaming
- [Tool calling](arch/tools.md) — tool loop, MCP, memory, skills, platform prompt, browser, web search and fetch
- [Delegation](arch/delegation.md) — `agent_spawn`, `session_send`
- [Mode, approval, and questions](arch/approval.md) — trust policy, plan escape, HIL round-trip, `ask_user_question`
- [`server/`](arch/server.md) — route groups, one gin engine
- [`cli/`](arch/cli.md) — admin surface, self-update, install scripts
- [`modules/client/`](arch/client.md) — wire protocol and the `-p` renderer
- [`modules/tui/`](arch/tui.md) — the full-screen TUI

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
