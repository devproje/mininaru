# mininaru

<img src="assets/logo.png" alt="mininaru" width="80" align="right">

An LLM harness in one Go binary: the agent runtime — a tool-calling loop with
bash, file edit, headless-browser, and MCP tools, plus persistent memory,
skills, and one-level delegation — reachable two ways.

- **`mininaru`** — a terminal REPL where every line goes straight to the
  agent; `/bash` runs one shell command on request (`/!bash` does the same
  without sharing the output with the agent), and dangerous tools are gated
  per directory.
- **An OpenAI-compatible HTTP + websocket API** backed by SQLite, with an
  admin CLI for providers, agents, and sessions.

This is a from-scratch rewrite, currently in the `1.0.0-alpha` series. An
earlier version had a Discord front end and a paired gRPC client; both are
gone. See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for how the pieces
fit together.

## Install

> **Alpha.** Releases start at `v1.0.0-alpha.1` and share nothing with the
> pre-refactor `0.x` line — the database schema, API, and on-disk layout are
> different, and a `0.x` binary will not run against this project's data.
> `install.sh` / `install.ps1` refuse to auto-install a release that resolves
> to a `0.x` tag (pass `--tag` to override, at your own risk).

**Linux / macOS**

```sh
curl -fsSL https://raw.githubusercontent.com/devproje/mininaru/master/scripts/install.sh | sh
```

**Windows** (PowerShell)

```powershell
irm https://raw.githubusercontent.com/devproje/mininaru/master/scripts/install.ps1 | iex
```

Downloads the latest release for your platform, checks it against the
release's `SHA256SUMS`, and installs `mininaru` — into `~/.local/bin` on
Linux/macOS (set `BINDIR` or `PREFIX` to change), or
`%LOCALAPPDATA%\mininaru\bin` on Windows (set `MININARU_BINDIR`); the Windows
script also adds that directory to your user `PATH`. Pass `--tag` (`-Tag` on
Windows) to pin a version; if `mininaru` is already on `PATH` the script
hands off to
`mininaru update` instead. To pass flags through the pipe on Windows, run
`& ([scriptblock]::Create((irm <url>))) -Tag v1.0.0-alpha.4` instead.
Building from a checkout is covered below.

`--path <dir>` (`-Path` on Windows) sets the data directory (default
`~/.mininaru`); `--uninstall` (`-Uninstall`) removes the binary, and on
Windows also clears the `NARU_PATH` user environment variable.

Run interactively (not piped) it also offers to run `mininaru daemon
install` — the background service described next — unless one is already
registered. That is also what pins `export NARU_PATH` in your shell rc (a
`User` environment variable on Windows), so `mininaru` uses one data
directory no matter which working directory you start it from — see
[Storage](#storage).

To keep a server up, `mininaru daemon install` registers a per-user service
that runs `mininaru serve` (with `NARU_PATH` pinned to the current data
directory) and starts it — `--host` and `--port` to change the bind address.
`mininaru daemon restart` restarts it after config changes;
`mininaru daemon uninstall` removes it. The backend is:

- **Linux** — a `systemd --user` unit; run `loginctl enable-linger` to keep
  it running after logout.
- **macOS** — a `launchd` agent in `~/Library/LaunchAgents`.
- **Windows** — a per-user Scheduled Task named `mininaru` that starts at
  logon (runs `mininaru serve` in the background, `NARU_PATH` taken from your
  user environment). `mininaru daemon restart` re-runs it;
  `mininaru daemon uninstall` deletes the task.

`mininaru daemon` needs systemd on Linux; it prints a clear error on a
platform where none of the three backends is available.

## Build from source

Requires the Go version declared in `go.mod`.

```sh
make build
./out/mininaru --version
```

`make dist GOOS=linux GOARCH=arm64` cross-compiles a single release layout
into `dist/` — this is what the release workflow runs for each target on a
pushed `v*` tag.

From a local checkout, `make install` installs `out/mininaru` into
`~/.local/bin` — `make install PREFIX=/usr/local` or `BINDIR=...` for another
location, `make uninstall` to remove it.

### Verifying a release download

Every tagged release publishes `SHA256SUMS` and a signed build provenance
attestation alongside the archives:

```sh
sha256sum -c SHA256SUMS --ignore-missing
gh attestation verify mininaru_v1.0.0-alpha.1_linux_amd64.tar.gz --repo devproje/mininaru
```

The attestation proves the archive came out of this repository's release
workflow, which a checksum alone cannot tell you.

## Updating

```sh
mininaru update                 # install the latest release
mininaru update --check         # print the installed and latest versions only
mininaru update --tag v1.0.0-alpha.1   # install a specific release
mininaru update --force         # reinstall the version already running
```

The archive is checked against the release's `SHA256SUMS` **before** anything
is replaced. On Linux and macOS the new file is moved into place with a
rename, which works even while the old one is still running. Windows refuses
to overwrite a running `.exe`, so there the current executable is renamed to
`<name>.exe.old` first and the staged build takes its place; the `.old` file
is removed on a best-effort basis (it may still be locked until the process
exits — a later `update` run cleans it up automatically).

Versioning restarts at `1.0.0-alpha.1` for this rewrite and does not
continue the pre-refactor `0.x` line, so `update` looks at the full release
list (including prereleases) rather than GitHub's "latest" endpoint, which
excludes prereleases and would otherwise resolve to the old, incompatible
architecture once a stable release exists again.

Once a day, at most, mininaru checks GitHub for the latest release tag in
the background and caches the answer in `update.json` under `NARU_PATH`. The
check never blocks a command: the result is written for the *next* run,
which is when the notice appears at the top of the REPL and under
`--version`.

```
a newer version is available: v1.0.0-alpha.2 (run `mininaru update`)
```

A `dev` build never shows it. Set `MININARU_NO_UPDATE_CHECK=1` to turn the
check off entirely.

## Storage

Everything lives under `.mininaru/` by default; set `NARU_PATH` to use
another directory. The default is resolved **relative to the working
directory** each process starts in, so running `mininaru` from different
places gives you different data directories — and the REPL talking to a
loopback server won't find its key unless both sides agree. `NARU_PATH` is
never re-exported, so pin it yourself for a stable location; the installers
do this (`export NARU_PATH=~/.mininaru` in your shell rc, a `User` variable
on Windows). The directory is created at mode `0700`, and an existing
one is tightened to `0700` on every start. Chat history is SQLite
(`.mininaru/data.db`, WAL mode); the REPL's input history is a plain text
file (`.mininaru/history` by default, or `$NARU_HISTFILE`); the server's
API key is `.mininaru/mininaru.key` (mode `0600`, generated the first time
anything needs it); yolo trust state is `.mininaru/directory.json`, managed
through `/yolo` rather than hand-edited; MCP servers are configured in
`.mininaru/mcp.json`, hand-editable or managed with `mininaru mcp` (see
[MCP servers](#mcp-servers)); each agent's persistent
memory (see [Tools](#tools)) lives under
`.mininaru/memory/<agent-id>/`, an `MEMORY.md` index plus one markdown
file per saved memory, managed entirely by the agent itself through the
`memory_*` tools rather than hand-edited; the daily background update check
caches the latest known release tag in `.mininaru/update.json`.

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
`serve` and the REPL are the two that can instead point at a remote one over
`--url`.

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

`mininaru` picks this up automatically when it's talking to a loopback
address (the default). Talking to a server on another host needs the key
passed explicitly — `mininaru --url ws://host:8223/ws --api-key '<KEY>'` or
`MININARU_API_KEY` — since the local key file is never sent anywhere but
loopback.

## Tools

An agent can run `bash_exec`, read/write/edit files, drive a headless
browser (`browser_navigate`/`browser_click`/`browser_type`/`browser_read`/
`browser_screenshot`/`browser_close`), delegate one self-contained task to
another of your configured agents with `agent_spawn`, and inject a message
into one of its own already-running sessions with `session_send` — plus
whatever MCP servers you configure (see [MCP servers](#mcp-servers) below).
`browser_*`
needs a Chrome or Chromium binary reachable via `$PATH` or
`MININARU_CHROME`; nothing else has an external dependency. `agent_spawn`
runs the delegate as a real session (it shows up in `session list` like any
other), starts it with no memory of the calling conversation; `session_send`
targets any session that already exists, including one owned by a
completely different agent — the only session it refuses is the caller's
own — and if a person is connected to it live, the injected message appears
on their screen straight away — marked with the session it came from — and
the reply streams in under it, whether or not they were typing at the time.
A message sent into a different agent's session is prefixed with which
agent it came from, so that agent's history doesn't read as if its own user
just said it. Neither tool can be handed to whatever it delegates to or
messages — one level of delegation, no chains. Two read-only discovery tools
round these out and always run with no approval needed: `agent_list` (every
configured agent, for picking an `agent_spawn` target) and `session_list`
(every session, across every agent, that currently has a live viewer
connected — marks which one is the current conversation, and any of them is
a valid `session_send` target).

Every agent also has persistent memory, `memory_save`/`memory_read`/
`memory_forget`. Memory is scoped to the agent (not the directory or
session), so it carries over to every future conversation with that agent,
in any working directory. A saved memory (`name`, `description`, `type` —
`user`/`feedback`/`project`/`reference` — and free-form markdown content)
shows up as a one-line entry in that agent's index at the start of every
future turn; the full content is only fetched with `memory_read` when the
agent actually needs it. These three tools are always safe to run — no
approval prompt — since they're confined to that agent's own memory
directory, never an arbitrary path.

Every dangerous tool above is gated by **yolo mode**, set per directory with
`/yolo <off|persist|on>` in the REPL:

- `off` (default) — always ask before running one.
- `persist` — auto-run inside the directory you set it in.
- `on` — auto-run everywhere, no prompts; `/yolo on` asks you to confirm
  once before switching.

When a call needs asking, the REPL shows the tool name and arguments and you
answer once / for the rest of the session / no.

## MCP servers

`.mininaru/mcp.json` can be hand-edited, or managed with `mininaru mcp`:

```sh
mininaru mcp add files --stdio npx --arg -y --arg @modelcontextprotocol/server-filesystem
mininaru mcp add remote --url https://example.com/mcp --header Authorization="Bearer token"
mininaru mcp list                 # dials every enabled server and reports connected/tool count/error
mininaru mcp show files
mininaru mcp disable files        # keeps its configuration, just stops connecting to it
mininaru mcp remove files
```

`--permission safe|dangerous` forces every tool on a server to one tier
regardless of the server's own `readOnlyHint` annotations; `--tool-permission
<tool>=safe|dangerous` (repeatable) overrides one tool by name — a per-tool
override always wins over a per-server one. A running `mininaru serve`
reloads its MCP connections on `SIGHUP` (`kill -HUP <pid>`), so changes made
with `mininaru mcp` take effect without restarting it.

## The interactive REPL

```sh
mininaru                                          # connects to ws://127.0.0.1:8223/ws
mininaru --url ws://example.com:8223/ws --api-key '<KEY>'
mininaru --session <id>                           # resume an existing conversation
mininaru --agent coder                            # pick an agent by name for a new session
```

Every line you type goes straight to the agent — there's no shell mode to
switch into; `/bash`/`/!bash` cover running a one-off shell command instead
(see [Tools](#tools) below for what the agent itself can run). A session is
created as soon as the REPL starts (unless `--session` names an existing
one), named at that point with a random `adjective-noun` pair
(`quiet-otter`, `still-meadow`, ...) rather than anything you have to pick.

| Key | Effect |
|---|---|
| `↑` / `↓` | recall input history |
| `Shift+Enter` (or `Ctrl+J`) | insert a newline, keep composing |
| `Ctrl+A` / `Ctrl+E` | start / end of line |
| `Ctrl+K` / `Ctrl+U` / `Ctrl+W` | kill to end / kill to start / kill word back |
| `Ctrl+Y` | yank the last kill back in |
| `Ctrl+L` | clear the screen |
| `Ctrl+←` / `Ctrl+→`, `Home` / `End` | word-wise and line-edge cursor movement |
| `Ctrl+C` (typing) | cancel the current line |
| `Esc` / `Ctrl+C` (response in flight) | interrupt the agent's turn |
| `Ctrl+D` | exit |

```
/help       list available commands
/exit       quit
/clear      clear the terminal screen
/bash       run one shell command; the command and its output are posted to the agent
/!bash      same, without sharing the output with the agent
/session    show or switch the current session
/gateway    pick a saved gateway and a session on it, then reconnect
/agent      switch agent on a new session
/model      change the connected agent's model
/effort     change the connected agent's reasoning effort (off|low|medium|high|max)
/yolo       set dangerous-tool trust for this directory (off|persist|on)
```

Input history is a plain text file, `.mininaru/history` by default (or
`$NARU_HISTFILE`); `$HISTSIZE`/`$HISTFILESIZE` cap what's kept in memory and
written back, the same way bash honors them.

## Non-interactive (`-p`)

```sh
mininaru -p "<prompt>"                            # one-shot, streams the transcript
mininaru -p "<prompt>" --session <id>             # run the turn on an existing session
mininaru -p "<prompt>" --format json              # -f json | xml | string (default)
```

`-p` runs a single turn without the REPL and exits. With no `--session` the
throwaway session it creates is deleted on exit.

`--format string` (the default) prints the same streamed transcript the REPL
shows. `--format json` and `--format xml` suppress the transcript and print one
object when the turn ends:

```json
{
  "session_id": "quiet-otter",
  "content": "the final answer text",
  "tools": [{ "name": "bash", "status": "finished" }]
}
```

In `json`/`xml` mode there is no prompt to approve tool calls, so any
approval request is auto-denied; a failed turn still prints the object (with an
`error` field) and exits non-zero.

## Gateways

A gateway is a saved name for a remote mininaru's websocket URL and api key,
so you don't retype `--url`/`--api-key` every time.

```sh
mininaru gateway add prod ws://box:8223/ws --api-key '<KEY>'
mininaru gateway list
mininaru gateway show prod          # api key is masked
mininaru gateway set prod --api-key '<NEW>'
mininaru gateway remove prod
```

Entries live in `.mininaru/gateways.json` (file mode `0600`, api keys stored
in the clear, same as `.mininaru/mininaru.key`).

`--gateway <name>` (or a bare `--url`/`--api-key`) then works anywhere:

```sh
mininaru --gateway prod                 # REPL against the remote
mininaru -p "hi" --gateway prod         # one-shot against the remote

mininaru agent list --gateway prod      # inspect the remote's agents
mininaru provider list --gateway prod
mininaru session list --gateway prod
mininaru session remove <id> --gateway prod
mininaru provider activate <name> --gateway prod
```

Management commands hit the remote's `/api` for reads (`list`, `show`) and for
`session remove` / `provider remove` / `provider activate`. `add` and `set` stay
local — configure a remote box on that box. `mcp` and `skill` have no remote API
and are always local.

`--gateway` cannot be combined with an explicit `--url`. With neither,
everything is local: `ws://127.0.0.1:8223/ws` and the local SQLite DB.

Inside the REPL, `/gateway` opens an arrow-key picker over the saved
gateways, then over that gateway's sessions (or `＋ new session`), and
reconnects the shell to the chosen one — no restart needed.

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
(`GET/POST/PATCH/DELETE`), which is what the REPL and the `provider`/
`agent`/`session` commands ultimately talk to. A provider's API key is never
returned in full over this API; list and read responses mask it.

`/api/mcp` manages MCP servers the same way `mininaru mcp` does — `GET /api/mcp`
(each server with its live connection status), `GET/DELETE /api/mcp/:name`,
`POST /api/mcp` (body is the `mcp.json` server object), `POST /api/mcp/:name/enable`
/ `disable`, and `POST /api/mcp/reload`. Every mutation reconnects the server's
MCP pool, so a `--gateway` client changes a remote box's tools without shell access.

`/api/skill` is read-only: `GET /api/skill` lists the skills found on disk,
`GET /api/skill/:name` returns the rendered text a skill puts in front of the
model, `GET /api/skill/uses?session=<id>` the load counts.

`/api/agents/:id/memory` is that agent's persistent memory store —
`GET` (the `MEMORY.md` index plus the file list), `GET /:file`,
`PUT /:file` (body `{description, type, content}`), `DELETE /:file`.

`/ws` is what the REPL uses for chat: send
`{"session_id": "...", "content": "...", "cwd": "..."}` and receive a stream
of `{"type": "chunk"|"tool"|"approval_request"|"done"|"error", ...}` frames.
Sending `{"type": "interrupt", "session_id": "..."}` cancels that session's
turn mid-stream. An `approval_request` frame blocks the turn until you answer with
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
