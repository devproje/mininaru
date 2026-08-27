# mininaru

<img src="assets/logo.png" alt="mininaru" width="80" align="right">

An OpenAI-compatible chat server backed by SQLite, plus an admin CLI and an
interactive terminal shell — one Go binary, no external dependencies.

This is a from-scratch rewrite, currently in the `1.0.0-alpha` series. An
earlier version of this project had skills, memory, subagent delegation, a
Discord front end, and a paired gRPC client. Discord and gRPC are gone; they
were dropped on purpose to rebuild the server and CLI from a small core.
Tool calling (bash, file read/write/edit, browser automation, MCP),
delegation (`agent_spawn`, `session_send`), persistent memory, and skills
have come back — see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for what
is actually here.

## Install

> **Alpha.** Releases start at `v1.0.0-alpha.1` and share nothing with the
> pre-refactor `0.x` line — the database schema, API, and on-disk layout are
> different, and a `0.x` binary will not run against this project's data.
> `install.sh` / `install.ps1` refuse to auto-install a release that resolves
> to a `0.x` tag (pass `--tag` to override, at your own risk).

```sh
curl -fsSL https://raw.githubusercontent.com/devproje/mininaru/master/scripts/install.sh | sh
```

Downloads the latest release for your platform, checks it against the
release's `SHA256SUMS`, and installs `mininaru` (plus the `narush` alias)
into `~/.local/bin`. Set `BINDIR` or `PREFIX` to change that, `--tag` to
pin a version; if `mininaru` is already on `PATH` the script hands off to
`mininaru update` instead. On Windows run `scripts/install.ps1` from
PowerShell. Building from a checkout instead is covered below.

It also pins `export NARU_PATH=~/.mininaru` in your shell rc (a `User`
environment variable on Windows), so `mininaru` uses one data directory no
matter which working directory you start it from — see [Storage](#storage).
`--path <dir>` to choose another; `--uninstall` removes both the binary and
the pin.

Run interactively (not piped) it also offers to set up the background
service described next, unless one is already registered.

To keep a server up, `scripts/register-daemon.sh` installs a
`systemd --user` unit that runs `mininaru serve` — `--host`, `--port`,
`--path` to set the data directory, `--linger` to survive logout, `--shell`
to also drop the `exec narush` hook and the `NARU_PATH` export into your
shell rc so an interactive shell shares the service's data directory,
`--disable` to undo all of it. On Windows `scripts/register-daemon.ps1`
registers a per-user Scheduled Task that starts it at logon.

## Build from source

Requires the Go version declared in `go.mod`.

```sh
make build
./out/mininaru --version
```

`make build` also creates `out/narush`, an alias for `mininaru shell` — run
it directly (with the same `--url`/`--session`/`--agent`/`--api-key` flags)
instead of typing `mininaru shell`.

`make dist GOOS=linux GOARCH=arm64` cross-compiles a single release layout
into `dist/` — this is what the release workflow runs for each target on a
pushed `v*` tag. `dist/` carries the same `narush` alias alongside
`mininaru`.

From a local checkout, `make install` installs `out/mininaru` (plus the
`narush` alias) into `~/.local/bin` — `make install PREFIX=/usr/local` or
`BINDIR=...` for another location, `make uninstall` to remove it.

### Using narush as your interactive shell

Don't set `$SHELL`/your login shell to `narush` directly — it has no
non-interactive `-c` mode, so anything that invokes `$SHELL -c '...'`
(`ssh host 'command'`, git hooks, editor/terminal integrations, cron) would
break, and `bashPath()` (what shell mode itself execs commands through)
falls back to `$SHELL` too, so it would try to run every shell-mode command
through `narush` instead of an actual shell.

The safe way to get "narush opens automatically in every terminal" is an
`exec` hook at the *end* of `~/.bashrc`/`~/.zshrc`, guarded against both
non-interactive shells and recursion:

```sh
if [[ $- == *i* ]] && [ -z "$MININARU_ACTIVE" ]; then
    exec narush
fi
```

`$SHELL` stays your real shell — `-c` invocations never hit this line at
all, since it only runs for interactive shells. The `$MININARU_ACTIVE`
check matters because shell mode itself launches an interactive
`bash -i -c` per command (so `.bashrc` aliases/functions work, see
[The interactive shell](#the-interactive-shell)) — without the guard,
every single shell-mode command would immediately re-launch narush instead
of running. `mininaru shell` sets `MININARU_ACTIVE=1` in its own
environment as one of the first things it does, which every process it
spawns inherits, so the guard only ever suppresses the hook while already
inside a narush-owned shell tree.

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
which is when the notice appears at the top of `mininaru shell` and under
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
places gives you different data directories — and a shell talking to a
loopback server won't find its key unless both sides agree. `NARU_PATH` is
never re-exported, so pin it yourself for a stable location; the installers
do this (`export NARU_PATH=~/.mininaru` in your shell rc, a `User` variable
on Windows). The directory is created at mode `0700`, and an existing
one is tightened to `0700` on every start. Chat history is SQLite
(`.mininaru/data.db`, WAL mode); the shell's bash command history is a plain
text file (`.mininaru/shell_history` by default, or `$HISTFILE`); the server's
API key is `.mininaru/mininaru.key` (mode `0600`, generated the first time
anything needs it); yolo trust state is `.mininaru/directory.json`, managed
through `/yolo` rather than hand-edited; MCP servers are configured in
`.mininaru/mcp.json`, hand-editable or managed with `mininaru mcp` (see
[MCP servers](#mcp-servers)); each agent's persistent
memory (see [Tools](#tools)) lives under
`.mininaru/memory/<agent-id>/`, an `MEMORY.md` index plus one markdown
file per saved memory, managed entirely by the agent itself through the
`memory_*` tools rather than hand-edited; the shell's default agent
(set via `/agent global <id-or-name>`) is persisted in `.mininaru/shell.json`,
so it carries over to the next `mininaru shell` launch without needing
`--agent` again — `/agent current <id-or-name>` skips this file, affecting
only the running shell; the daily background update check caches the latest
known release tag in `.mininaru/update.json`.

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
`/yolo <off|persist|on>` in the shell:

- `off` (default) — always ask before running one.
- `persist` — auto-run inside the directory you set it in.
- `on` — auto-run everywhere, no prompts; `/yolo on` asks you to confirm
  once before switching.

When a call needs asking, the shell shows the tool name and arguments and
you answer once / for the rest of the session / no.

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

## The interactive shell

```sh
mininaru shell                                    # connects to ws://127.0.0.1:8223/ws
mininaru shell --url ws://example.com:8223/ws --api-key '<KEY>'
mininaru shell --session <id>                     # resume an existing conversation
mininaru shell --agent coder                       # pick an agent by name for a new session
```

`mininaru shell` runs a shell prompt and an agent chat over the same line
editor. **Shift+Tab** switches between them; if the server is unreachable it
starts in shell mode, so a running `mininaru serve` is optional for local
shell use. The connection is not something you have to manage: whenever
there isn't one, the shell keeps retrying in the background with a backoff,
and when it lands it re-attaches to the same session and puts you back in
whichever mode you were in. Restarting the server looks like a short pause.
Shift+Tab still retries immediately if you don't want to wait.

No session is created just by starting the shell — it picks an agent right
away (so the prompt shows its name and reasoning effort immediately), but
the session itself only comes into existence when you send your first
agent-mode message, named at that point with a random `adjective-noun`
pair (`quiet-otter`, `still-meadow`, ...) rather than anything you have to
pick yourself.

| Key | Shell mode | Agent mode |
|---|---|---|
| `Tab` | complete commands, args (real bash completion — branches, subcommands), and paths | complete `/`-commands and `@`-file references |
| `↑` / `↓` | recall shell history | recall agent history (kept separate) |
| `Ctrl+J` | insert a newline, keep typing | same — compose a multi-line message |
| `Ctrl+A` / `Ctrl+E` | start / end of line | same |
| `Ctrl+K` / `Ctrl+U` / `Ctrl+W` | kill to end / kill to start / kill word back | same |
| `Ctrl+Y` | yank the last kill back in | same |
| `Ctrl+L` | clear the screen | same |
| `Ctrl+←` / `Ctrl+→`, `Home` / `End` | word-wise and line-edge cursor movement | same |
| `Esc` / `Ctrl+C` | interrupt the shell's own subshell input | interrupt the response in flight |
| `Ctrl+D` | exit the shell | same |

Typing an incomplete bash construct (an open `for`/`if`, an unclosed quote,
a trailing `\`) automatically continues onto a `> ` prompt until it parses,
the same way an interactive `bash` does. `su` and `sudo` re-exec the shell
itself as the target user, carrying the session over, so the prompt and
history survive a privilege switch.

Each shell-mode line still runs as its own process — this isn't a single
persistent shell — but `export`ed variables, functions, and aliases you
define at the prompt carry over to the next line anyway; only things like
`cd` already worked this way before.

Inside agent mode, `@path/to/file` anywhere in a message pulls that file's
content into what gets sent to the agent (path resolved relative to the
shell's cwd, `~` expanded, tab-completes like a path) — the `@` is stripped
from the message text itself and the content is appended after it; a path
that doesn't resolve is left as plain text with nothing attached, no error.

```
/help       list available commands
/reset      start a fresh session with the same agent
/session    show the current session id, name, agent, and creation time
/agent      switch agent now: global <id-or-name> also persists as the default; current <id-or-name> is this shell only
/model      change the connected agent's model
/effort     change the connected agent's reasoning effort (off|low|medium|high|max)
/clear      clear the terminal screen
/bash       back to shell mode
/exit       quit mininaru shell
/yolo       set dangerous-tool trust for this directory (off|persist|on)
```

`history` is a GNU-bash-compatible builtin in shell mode: `history N`,
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
