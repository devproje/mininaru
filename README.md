# mininaru

<img src="assets/logo.png" alt="mininaru" width="80" align="right">

An OpenAI-compatible chat server backed by SQLite, plus an admin CLI and an
interactive terminal shell — one Go binary, no external dependencies.

This is a rewrite in progress (`refactor/1.0.0-alpha`). An earlier version of
this project had skills, memory, subagent delegation, a Discord front end,
and a paired gRPC client. None of that exists in this branch; it was dropped
on purpose to rebuild the server and CLI from a small core. Tool calling
(bash, file read/write/edit, browser automation, MCP) has come back — see
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for what is actually here.

## Build from source

Requires the Go version declared in `go.mod`.

```sh
make build
./out/mininaru --version
```

`make dist GOOS=linux GOARCH=arm64` cross-compiles a single release layout
into `dist/` — this is what the release workflow runs for each target on a
pushed `v*` tag, and it is currently the only way to get a mininaru binary;
there is no install script yet.

### Verifying a release download

Every tagged release publishes `SHA256SUMS` and a signed build provenance
attestation alongside the archives:

```sh
sha256sum -c SHA256SUMS --ignore-missing
gh attestation verify mininaru_v0.2.0_linux_amd64.tar.gz --repo devproje/mininaru
```

The attestation proves the archive came out of this repository's release
workflow, which a checksum alone cannot tell you.

## Storage

Everything lives under `.mininaru/` by default; set `NARU_PATH` to use
another directory. The directory is created at mode `0700`, and an existing
one is tightened to `0700` on every start. Chat history is SQLite
(`.mininaru/data.db`, WAL mode); the shell's bash command history is a plain
text file (`.mininaru/shell_history` by default, or `$HISTFILE`); the server's
API key is `.mininaru/mininaru.key` (mode `0600`, generated the first time
anything needs it); yolo trust state is `.mininaru/directory.json`, managed
through `/yolo` rather than hand-edited; MCP servers are configured in
`.mininaru/mcp.json`, which you do edit by hand.

## Set up a provider and an agent

A `provider` is a base URL and API key for an OpenAI-compatible endpoint. An
`agent` names a model against a provider, plus an optional system prompt
(`--soul`), reasoning effort, and context budget.

```sh
mininaru provider add openai --base-url https://api.openai.com/v1 --api-key '<KEY>' --activate
mininaru agent add naru --model gpt-4o-mini --soul 'terse and precise'
```

`show`/`set`/`remove`/`activate` all take either the id or the name printed
by `add`. `provider set`/`agent set` only change the fields you pass; leave a
flag out to keep the current value.

```sh
mininaru provider list
mininaru provider set openai --base-url https://api.openai.com/v1
mininaru provider activate openai
mininaru provider remove openai

mininaru agent list
mininaru agent set naru --thinking high
mininaru agent remove naru
```

`session list` (optionally `--agent <id-or-name>`), `session show <id>`, and
`session remove <id>` inspect and clean up conversations directly against
the local database — no server needs to be running for any of this. Provider
and agent administration always operates on the local `NARU_PATH` database;
`serve` and `shell` are the two commands that can instead point at a remote
one over `--url`.

## Serving

```sh
mininaru serve                              # 0.0.0.0:8223
mininaru serve --host 127.0.0.1 --port 8080
mininaru serve --debug                      # verbose gin request logging
```

This starts the HTTP API and the `/ws` websocket on the same listener. Every
request needs `Authorization: Bearer <key>`. The key is generated once, the
first time anything needs it, and stored at `NARU_PATH/mininaru.key` (mode
`0600`) — there is no setup step and no command to print it again later:

```sh
cat .mininaru/mininaru.key   # or $NARU_PATH/mininaru.key
```

`mininaru shell` picks this up automatically when it's talking to a loopback
address (the default). Talking to a server on another host needs the key
passed explicitly — `mininaru shell --url ws://host:8223/ws --api-key '<KEY>'`
or `MININARU_API_KEY` — since the local key file is never sent anywhere but
loopback.

## Tools

An agent can run `bash_exec`, read/write/edit files, and drive a headless
browser (`browser_navigate`/`browser_click`/`browser_type`/`browser_read`/
`browser_screenshot`) — plus whatever MCP servers you configure in
`NARU_PATH/mcp.json`. `browser_*` needs a Chrome or Chromium binary
reachable via `$PATH` or `MININARU_CHROME`; nothing else has an external
dependency.

Every one of those is gated by **yolo mode**, set per directory with
`/yolo <off|persist|on>` in the shell:

- `off` (default) — always ask before running one.
- `persist` — auto-run inside the directory you set it in.
- `on` — auto-run everywhere, no prompts; `/yolo on` asks you to confirm
  once before switching.

When a call needs asking, the shell shows the tool name and arguments and
you answer once / for the rest of the session / no.

## The interactive shell

```sh
mininaru shell                                    # connects to ws://127.0.0.1:8223/ws
mininaru shell --url ws://example.com:8223/ws --api-key '<KEY>'
mininaru shell --session <id>                     # resume an existing conversation
mininaru shell --agent coder                       # pick an agent by name for a new session
```

`mininaru shell` runs a bash prompt and an agent chat over the same line
editor. **Shift+Tab** switches between them; if the server is unreachable it
starts in bash mode and retries the connection on every switch, so a running
`mininaru serve` is optional for local shell use.

| Key | Bash mode | Agent mode |
|---|---|---|
| `Tab` | complete commands and paths | complete `/`-commands |
| `↑` / `↓` | recall bash history | recall agent history (kept separate) |
| `Ctrl+J` | insert a newline, keep typing | same — compose a multi-line message |
| `Ctrl+U` | clear the current line | same |
| `Esc` | — | interrupt the response in flight |
| `Ctrl+D` | exit the shell | same |

Typing an incomplete bash construct (an open `for`/`if`, an unclosed quote,
a trailing `\`) automatically continues onto a `> ` prompt until it parses,
the same way an interactive `bash` does. `su` and `sudo` re-exec the shell
itself as the target user, carrying the session over, so the prompt and
history survive a privilege switch.

Inside agent mode:

```
/help       list available commands
/reset      start a fresh session with the same agent
/session    show the current session id, agent, and creation time
/info       show the splash banner and current connection/session info
/clear      clear the terminal screen
/bash       back to bash mode
/exit       quit mininaru shell
/yolo       set dangerous-tool trust for this directory (off|persist|on)
```

`history` is a GNU-bash-compatible builtin in bash mode: `history N`,
`history -c` (clear), `history -d offset` (delete one entry, a negative
offset counts from the end), `history -w`/`-r` (write/read the history
file). `HISTSIZE`, `HISTFILESIZE`, and `HISTFILE` are honored the way bash
honors them.

## API

`POST /api/v1/chat/completions` and `GET /api/v1/models` are OpenAI-compatible.
The `model` field names a **mininaru agent** by its configured name, not an
upstream model string — `GET /models` lists your agents.

```sh
KEY=$(cat .mininaru/mininaru.key)

curl -H "Authorization: Bearer $KEY" http://127.0.0.1:8223/api/v1/models

curl -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"model":"naru","messages":[{"role":"user","content":"hello"}]}' \
  http://127.0.0.1:8223/api/v1/chat/completions

curl -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"model":"naru","stream":true,"messages":[{"role":"user","content":"hello"}]}' \
  http://127.0.0.1:8223/api/v1/chat/completions
```

`stream: true` returns `text/event-stream` chunks ending in `data: [DONE]`.
The server is stateless per request here: nothing is read from or written to
the session store, and `messages` in the request body is the entire history
you want considered — there is no server-side history for this endpoint.

`/api` exposes plain REST CRUD for agents, providers, sessions, and messages
(`GET/POST/PATCH/DELETE`), which is what `cli/shell` and the `provider`/
`agent`/`session` commands ultimately talk to. A provider's API key is never
returned in full over this API; list and read responses mask it.

`/ws` is what `mininaru shell` uses for agent-mode chat: send
`{"session_id": "...", "content": "...", "cwd": "..."}` and receive a stream
of `{"type": "chunk"|"tool"|"approval_request"|"done"|"error", ...}` frames.
An `approval_request` frame blocks the turn until you answer with
`{"type": "approval", "session_id": "...", "decision": "once"|"session"|"deny"}`
— see "Tools" above.

## Development

```sh
make build      # -> out/mininaru
make fmt        # gofmt -l, fails on unformatted files
make vet        # go vet ./...
make test       # fmt + vet + go test ./... -v
make test-race  # the same suite under the race detector
make test-cover # race + coverage, writes out/coverage.out
```

`make test-race` is what CI runs on every push and pull request. See
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the package layout and how
the pieces fit together, and [docs/CONVENTION.md](docs/CONVENTION.md) for the
code style `make fmt`/`make test` enforce.
[docs/AGENTS.md](docs/AGENTS.md) is what to hand an AI coding agent before it
touches this repository.

## License

mininaru is free software under the **GNU General Public License, version 3
or later** (`GPL-3.0-or-later`). See [LICENSE](LICENSE) for the full text.

**The artwork is not covered by the GPL.** `assets/logo.png` and the naru
character are all rights reserved; see [COPYRIGHT.md](COPYRIGHT.md) for what
that allows. The software is unaffected by this — nothing in the program
depends on the artwork.

[CONTRIBUTING.md](CONTRIBUTING.md) covers how to send a change, and
[SECURITY.md](SECURITY.md) is where to report a vulnerability privately —
please do not open a public issue for one.
