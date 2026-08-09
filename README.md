# mininaru

<img src="assets/logo.png" alt="mininaru" width="80" align="right">

Lightweight terminal chat client for OpenAI-compatible LLM APIs. It ships as one
Go binary and stores configuration and conversation history locally.

## Build

Requires the Go version declared in `go.mod`.

```sh
make build
./out/mininaru --version
```

Install or remove only the binary for the current user:

```sh
make install
make uninstall
```

## User daemon

The daemon command manages a systemd user service separately from the binary
installer. Create its environment file first; the command refuses files that
are readable by group or others.

```sh
mkdir -p ~/.config/mininaru
install -m 0600 /dev/null ~/.config/mininaru/env
$EDITOR ~/.config/mininaru/env    # add MININARU_API_KEY=<API_KEY>

mininaru daemon install
mininaru daemon reload      # restart it to pick up configuration changes
mininaru daemon uninstall
```

The service uses the current `NARU_PATH` and working directory at install time.
Use `--env-file` or `--working-directory` to override either daemon setting.

Providers, agents, bots, and mcp servers are read once at startup, so changes
made with the other commands need a `daemon reload` to take effect. The running
service also reloads its configuration on `SIGHUP` without dropping connections.

## First run

`setup` walks through the whole thing -- provider, agent, the thinking and tool
defaults, and finally the systemd user daemon -- asking one question at a time:

```sh
./out/mininaru setup
```

It keeps what is already configured unless you say otherwise, so it is safe to
run again later.

The daemon step is offered only when `systemctl` is on `PATH`, and it is opt-in.
Accepting it writes the environment file at mode `0600` with an API key you
supply, or one it generates and prints, and then installs and starts the unit.
An existing environment file that already defines `MININARU_API_KEY` is left
alone; one that exists without it is reported rather than overwritten. If the
daemon step fails, the provider and agent are already saved and setup says so
instead of unwinding.

To do it by hand instead, add an OpenAI-compatible provider and then the global
chat agent:

```sh
./out/mininaru provider add           \
  --name local                        \
  --base-url http://127.0.0.1:8080/v1 \
  --api-key '<API_KEY>'

./out/mininaru agent add              \
  --name naru                         \
  --model '<MODEL_NAME>'              \
  --role 'Helpful terminal assistant'

./out/mininaru
```

The first agent becomes the global agent used by the interactive client.

## Interactive prompts

The commands that create or change something -- `provider add`, `provider
update`, `agent add`, `agent update`, `bot add`, `bot update`, `mcp add`, and
`session rename` -- ask for whatever you did not pass as a flag:

```sh
$ mininaru provider add
provider name: openrouter
base url: https://openrouter.ai/api/v1
api key (leave empty to skip):
```

API keys and bot tokens are read without echo. An `update` with no flags walks
every field with the current value as the default, so pressing enter keeps it;
leaving a secret empty keeps the stored one rather than clearing it.

**Prompting only happens on a terminal.** When stdin or stderr is a pipe --
a script, a systemd unit, CI -- nothing is asked and a missing required value
is still an error, so automation behaves exactly as before:

```sh
$ mininaru provider add < /dev/null
Error: provider name is required, pass --name
```

Prompts are written to stderr, so stdout stays parseable.

## Common commands

```sh
mininaru setup                 # guided first run configuration
mininaru provider list
mininaru provider default [id-or-name]
mininaru agent list
mininaru agent default [id-or-name]   # show or set the global agent
mininaru agent remove <id-or-name>    # also deletes that agent's sessions
mininaru session list
mininaru session list --agent coder
mininaru session remove <id> --agent coder
mininaru session rename <id> --name 'New name'
mininaru --session             # resume the latest non-empty session
mininaru --session <id>        # resume a specific session
mininaru --agent coder         # chat with an agent other than the global one
mininaru thinking high --show
mininaru context 32768         # approximate character budget for history
mininaru tools list            # list every available tool and where it came from
mininaru tools on              # enable tool calling (default)
mininaru tools off             # disable for models without tool support
mininaru mcp list              # configured mcp servers and their connection state
mininaru skill list            # installed skills and which root they came from
mininaru skill show <name>     # exactly what the skill tool would return
mininaru web show              # search provider, endpoint, and masked api key
mininaru bot list              # chat bot front ends the daemon starts
mininaru --allow-dangerous-tools # expose file and shell tools for this run
```

## One-shot prompts

`-p` runs a single turn without the TUI and prints the answer, which makes the
client scriptable.

```sh
mininaru -p 'summarize the release notes'
mininaru -p - < question.txt          # read the prompt from stdin
mininaru -p 'and then?' --session     # continue the latest session
mininaru -p 'review this' --agent coder
```

Only the answer goes to stdout, so the output pipes cleanly. Tool progress and
reasoning text go to stderr, and a failure exits non-zero with nothing on
stdout. `ctrl+c` cancels the request; the turn is stored as `cancelled` rather
than left half-written.

Each run starts a new session unless `--session` is given, so `mininaru -p` is a
fresh conversation and `mininaru -p ... --session` continues the last one.

Dangerous tools are denied in this mode because there is nobody to approve them
— the denial is returned to the model as a tool error so it can carry on. Pass
`--allow-dangerous-tools` to let them run unattended.

Inside the TUI, use `/help`, `/thinking`, or `ctrl+t`. Press `esc` to interrupt
the current response and `ctrl+c` to quit. The client runs in the terminal's
alternate full-screen buffer; use `PageUp` and `PageDown` to scroll the
transcript and `End` to return to the latest message.

## Storage and security

Data is stored in `.mininaru/` by default. Set `NARU_PATH` to use another
directory. The directory itself is created with mode `0700`, and an existing one
is tightened to `0700` on startup. Provider and agent settings are JSON files
with mode `0600`; chat sessions are stored in SQLite with WAL, foreign keys, and
a busy timeout enabled on every pooled connection.

Settings files are written to a temporary file and renamed into place, so an
interrupted write cannot truncate your providers or agents.

Provider API keys live in `provider.json` and bot tokens in `bot.json`. Both are
only masked in list output, never encrypted. Do not commit the data directory. A
provider cannot be deleted while an agent references it.

The first agent created becomes the global agent. Removing it promotes the next
agent automatically, and `agent default` sets it explicitly. Removing an agent
also deletes its sessions, which cascade to their messages and tool calls.

Completed messages are used as model history. Failed and cancelled requests are
kept in SQLite with their status and error for diagnostics, but are excluded
from replay and future model requests. Resuming a session also replays the tool
calls and results of each turn, so the model remembers what it already looked
up. When history exceeds the configured context character budget, the oldest
complete turns are omitted; tool arguments and results count toward that budget
because they are sent to the model.

The first agent you create becomes the global agent and is the default for the
interactive client. `--agent <name>` chats with any other agent, and sessions
stay scoped to the agent that owns them.

## Runtime identity

Every request carries a `<mininaru-runtime>` block as the opening of its system
message, naming the exact build the model is running inside:

```
mininaru v0.1.0-b7ec66c (branch: master) linux/amd64
```

It is the same string `mininaru --version` prints, it is sent whether or not the
agent has a persona, and it comes with instructions that outrank the persona and
tell the model to reject a user who insists the values are different. Ask any
agent what it runs on and you get the build that answered, not a guess.

## Tools

Every tool reaches the model over MCP. The eight built-in tools are served by an
MCP server running inside the mininaru process, and additional servers can be
configured in `mcp.json`.

Safe built-in tools are exposed through the OpenAI-compatible function-calling
protocol: `current_time`, `web_search`, `web_fetch`, and `skill`. See Web tools
below for the two network ones.

`file_read`, `file_write`, and `bash_exec` are rooted at the directory where the
process started. They reject lexical and symlink path escapes where applicable.
Without a flag, each dangerous call pauses the TUI and asks for approval: press
`y` to execute or `n`/`esc` to deny it. A denied call is returned to the model as
a tool error so the conversation can continue.

Passing `--allow-dangerous-tools` bypasses every approval prompt for that run.
Treat this flag as unattended access to files and shell commands under your user
account; use it only in a dedicated working directory.

`bash_exec` runs through `bash`, falling back to `sh` when there is no bash on
`PATH`; set `MININARU_SHELL` to pick a different one. A timed-out command has its
whole process group killed, so a backgrounded child cannot outlive the call or
hold the tool open past its timeout.

`memory` is the eighth built-in tool and the only **privileged** one. It stores
durable facts in a small global SQLite table shared by the interactive front
ends, capped at 4096 characters in total. It runs without an approval prompt,
because the front ends that can reach it are already trusted: the TUI and a
paired Discord admin. It is refused outright anywhere else, so it is never
offered over the HTTP API and a regular Discord user cannot call it.

Tool calls and results are recorded in SQLite and the model may perform at most
eight tool rounds for one user request. The TUI prints a compact log when a tool
starts and when it completes or fails. Calls are inserted as `pending` before
approval or execution, then updated to `completed` or `failed`; stored logs are
shown again when the session is resumed. Arguments and results may contain
sensitive data, so protect the local database accordingly.

## Web tools

`web_search` returns result titles, URLs, and snippets. The provider is
configurable and lives in `.mininaru/web.json` (mode `0600`, since it may hold an
API key):

```sh
mininaru web show                              # provider, endpoint, masked key
mininaru web provider duckduckgo               # default, no key needed
mininaru web provider searxng
mininaru web endpoint https://searx.example.com
mininaru web key '<API_KEY>'
mininaru web provider brave                    # refused until a key is set
```

| Provider | Needs | Notes |
|---|---|---|
| `duckduckgo` | nothing | Default. Scrapes the non-JavaScript HTML endpoint. |
| `searxng` | `endpoint` | Your own instance. Requires `search.formats: [html, json]` in its `settings.yml` — the JSON API is off by default and the failure mode is an opaque 403. |
| `brave` | `key` | Brave Search API, `X-Subscription-Token`. |
| `tavily` | `key` | Results carry longer content excerpts. |

The CLI validates strictly and refuses an incomplete configuration up front. A
`web.json` that is already broken on disk is a different case: mininaru warns on
stderr and falls back to DuckDuckGo rather than leaving you with no search at all.

`web_fetch` retrieves an http or https URL. HTML is converted to text (scripts,
styles, and markup dropped, block structure preserved); JSON and plain text pass
through unchanged; anything binary becomes a placeholder rather than being dumped
into the transcript. Pass `raw: true` for the unprocessed body and `max_chars` to
change the 24k default. The result carries the status, the content type, and the
**final** URL after redirects.

**`web_fetch` cannot reach your internal network.** Loopback, private, link-local
(including the `169.254.169.254` cloud metadata endpoint), CGNAT, multicast, and
reserved ranges are all refused — in IPv4, in IPv6, and in IPv4-in-IPv6 mapped
form. The check runs on the resolved IP immediately before `connect(2)`, so DNS
rebinding and redirect chains cannot slip past it, and proxy environment variables
are deliberately ignored because they would route around the check entirely.

The configured search endpoint is exempt, which is what lets a self-hosted SearXNG
on `127.0.0.1` work. That is safe because the endpoint comes from your `0600`
config file while a fetch URL comes from the model — possibly from a page the
model just read.

## Skills

A skill is a folder of instructions the model loads when it needs them. The
system prompt only carries a one-line summary of each; the full text arrives when
the model calls the `skill` tool. Adding skills therefore costs one line of
context each, not one tool schema each.

```
.mininaru/skills/pr-review/
  SKILL.md          # required
  scripts/diff.sh   # optional companion files
```

```markdown
---
name: pr-review
description: Review a pull request in this repository, focusing on correctness over style.
---

# Reviewing a PR

1. Run `scripts/diff.sh` to get the changed files.
2. Read the tests before the implementation.
```

Two locations are scanned, in this order:

1. `.mininaru/skills/` — the project (or `NARU_PATH`)
2. `~/.mininaru/skills/` — your user account

The project copy wins a name collision, so a repository can override a personal
skill. A folder without a `SKILL.md` is ignored silently; one that fails to parse
prints a line to stderr and is skipped without affecting the others.

**Existing Claude Code / Agent Skills bundles drop in unmodified.** Extra
frontmatter keys are parsed over and ignored — including **`allowed-tools`, which
is not honored**, since mininaru has no per-turn tool filter.

The `skill` tool is safe, so skills work in the TUI, over the HTTP API, and in
Discord. It returns the instructions, the bundle's absolute path, and a list of
companion files. Running one of those scripts is still `bash_exec`, with the same
approval prompt as any other shell command.

`mininaru skill show <name>` prints the exact string the model receives, so it
doubles as an audit of what your skill is really sending.

## MCP servers

Extra tools come from MCP servers listed in `.mininaru/mcp.json`. Both local
child processes and remote streamable-HTTP endpoints are supported.

```sh
mininaru mcp add fs --stdio npx --arg -y --arg @modelcontextprotocol/server-filesystem --arg .
mininaru mcp add notion --url https://mcp.notion.com/mcp --header 'Authorization=Bearer <TOKEN>'
mininaru mcp list                # name, transport, state, tool count, error
mininaru mcp disable notion      # keep it configured, stop connecting
mininaru mcp remove notion
```

Their tools appear as `server__tool` in `mininaru tools list`. A server that
fails to connect is reported on stderr and skipped — the rest of your tools keep
working.

Whether an MCP tool counts as safe or dangerous comes from the tool's own
`readOnlyHint` annotation; a tool that declares nothing is treated as dangerous
and needs approval. Override it per server with `--permission safe|dangerous`,
or per tool via `tool_permission` in `mcp.json`. Because that annotation is the
server's claim about itself, adding a server you do not trust is equivalent to
trusting it — `"daemon": false` (`--no-daemon`) keeps a server's tools in the
TUI while hiding them from the HTTP API and the bots, where nobody can approve
anything.

`mcp.json` may hold tokens in `env` and `headers`, so it is written with mode
`0600` like the other settings files.

## Server

`mininaru serve` exposes an OpenAI-compatible HTTP API. Every endpoint is under
`/api/v1`.

```sh
mininaru serve --api-key '<KEY>'                 # 127.0.0.1:8080
mininaru serve --host 0.0.0.0 --port 3000 --api-key '<KEY>'
```

An API key is required. Pass `--api-key` or set `MININARU_API_KEY`; the server
refuses to start without one and answers `401` unless the request carries
`Authorization: Bearer <KEY>`.

The daemon holds one instance per agent. Turns on the same session are
serialized while different sessions run in parallel, so several front ends can
share one daemon safely. Agent, provider, and MCP settings are read at startup;
send `SIGHUP` to pick up later changes without restarting:

```sh
mininaru agent add --name coder --model qwen
kill -HUP $(pgrep -f 'mininaru serve')
```

Requests already in flight finish against the configuration they started with.
MCP servers whose entry did not change keep their connection; changed, added, or
removed ones are re-dialed or closed.

## Logging

Diagnostics go to **stderr** as structured `log/slog` records. stdout is left
alone, so `mininaru -p` still pipes cleanly and the list commands stay
machine-readable.

```sh
mininaru --log-level=debug serve --api-key '<KEY>'
mininaru --log-format=json serve --api-key '<KEY>'

MININARU_LOG_LEVEL=debug  mininaru serve --api-key '<KEY>'
MININARU_LOG_FORMAT=text  mininaru serve --api-key '<KEY>'
```

`--log-level` takes `debug`, `info` (default), `warn`, or `error`. `--log-format`
takes `auto` (default), `text`, or `json`; `auto` picks `text` when stderr is a
terminal and `json` otherwise, which is what makes a systemd unit emit JSON while
an interactive run stays readable. Flags win over the environment variables.

Every HTTP request is logged once on the way out with its method, path, status,
response size, and duration:

```
level=INFO msg="request completed" request_id=e3310957 method=GET path=/api/v1/models status=200 bytes=98 duration_ms=0
level=WARN msg="request rejected" request_id=55c773b7 method=GET path=/api/v1/models status=401 bytes=107 duration_ms=0
```

A `4xx` logs at `warn` and a `5xx` at `error`, so `--log-level=warn` shows only
the requests that went wrong. Each request carries a `request_id` that is also
returned in the `X-Request-Id` response header, and an inbound `X-Request-Id` is
reused rather than replaced, so a record can be traced from a proxy through to
the completion that produced it:

```
level=ERROR msg="completion failed" request_id=4bb09850 method=POST path=/api/v1/chat/completions agent=naru model=test-model duration_ms=1367 error="..."
level=ERROR msg="request failed"    request_id=4bb09850 method=POST path=/api/v1/chat/completions status=502 bytes=91 duration_ms=1367
```

A successful completion logs its agent, model, whether it streamed, and the token
usage the upstream reported. API keys are never logged — a failed authorization
records only whether a key was presented at all.

The interactive client holds diagnostics in memory while it owns the terminal, so
a warning cannot corrupt the alternate screen buffer; held records are flushed to
stderr when the TUI exits.

## Discord

The daemon runs Discord bots alongside the API. Register one the same way you
register a provider or an agent, and `serve` starts every enabled bot:

```sh
mininaru bot add --name naru-bot --token '<BOT_TOKEN>'
mininaru bot add --name coder-bot --token '<BOT_TOKEN>' --agent coder --guild '<GUILD_ID>'
mininaru bot list
mininaru serve --api-key '<KEY>'
```

Bots are stored in `bot.json` with mode `0600` and tokens are masked in list
output, exactly like provider API keys. Each bot can be bound to an agent with
`--agent`; without one it uses the global agent. Several bots can run in one
daemon, so a separate Discord application per agent is fine.

```sh
mininaru bot update coder-bot --guild '<GUILD_ID>'
mininaru bot update coder-bot --agent ''    # fall back to the global agent
mininaru bot disable naru-bot               # keep it configured, stop starting it
mininaru bot enable naru-bot
mininaru bot remove naru-bot
```

If any bot fails to start, `serve` stops the ones it already started and exits
with the failing bot's name.

In the
Discord developer portal the application needs the **Message Content Intent**
enabled — it is privileged, and without it the bot receives empty message bodies
and never answers. Invite it with the `bot` and `applications.commands` scopes.

Create a one-time admin pairing code before first use:

```sh
mininaru bot pair naru-bot
```

Run `/pair code:<code>` in Discord within 10 minutes. The paired Discord user
becomes an admin. Admins can add regular users with `/user add`; users are
scoped to that configured bot.

The bot answers when an authorized user mentions it in a server channel, and
answers every authorized message in a DM without requiring a mention. A guild
mention starts a thread when Discord permits it and falls back to the current
channel if thread creation fails. Messages inside a thread created by the bot
do not need to mention it again. It shows Discord's typing indicator while
generating, then sends the completed reply split across Discord's 2000 character
limit. A single reply is capped at ten minutes, and stopping the daemon cancels
whatever turns are still running instead of leaving them to finish unobserved.

Discord messages can include up to four supported attachments. PNG, JPEG, GIF,
and WebP images are sent as vision input; text, source code, JSON, and similar
text formats are included as text; PDF files are sent as file input. Each file
is limited to 10 MiB and the combined input to 20 MiB. Only HTTPS Discord CDN
attachment URLs are downloaded. `/chat` also accepts one optional `attachment`.

Each channel is bound to one session, so a channel is a running conversation
with all the history, tool replay, and context trimming the TUI gets.

- `/reset` starts a fresh conversation in the channel
- `/agent` shows which agent answers there, `/agent <name>` switches it

Switching agents starts a new conversation; the old one keeps its history and
simply stops being the channel's live session. The bot's `--agent` picks which
agent new channels start with, defaulting to the global agent. `--guild`
registers the slash commands to a single guild, which applies them immediately
instead of taking up to an hour to propagate globally.

Regular users can only use safe daemon tools. Admins can also use dangerous
tools, but each dangerous call requires an explicit Approve/Deny click in
Discord and expires after five minutes. Only the admin who made the request can
answer its approval prompt. Admins also reach the privileged `memory` tool,
which regular users never see.

When the Discord application has **User Install** enabled, two global message
commands are available from a message's **Apps** context menu in guilds, bot
DMs, regular DMs, and group DMs:

- `Message Analyzer` explains the selected text's likely intent, tone, request,
  ambiguity, and assumptions without storing a session
- `Content Search` searches the public web for the selected text's topic and
  claims, then returns linked sources

`/chat content:<message> ephemeral:<true|false>` runs an independent, one-turn
chat without reading or writing conversation history. `ephemeral` is optional
and defaults to `true`, so the result is only visible to the caller unless
explicitly disabled. The message context commands only receive the explicitly
selected message; they do not scan the surrounding channel history.

## API

The `model` field names a **mininaru agent**, not an upstream model. The server
resolves the agent, uses its provider and model, and prepends the runtime pin
and the agent's role and soul as the first system message, so a client selecting
`naru` in a model picker gets that agent's persona. A `system` message in the
request is kept, but it lands after that one. `GET /api/v1/models` lists every
configured agent by name.

```sh
curl -H 'Authorization: Bearer <KEY>' http://127.0.0.1:8080/api/v1/models

curl -H 'Authorization: Bearer <KEY>' -H 'Content-Type: application/json' \
  -d '{"model":"naru","messages":[{"role":"user","content":"hello"}]}' \
  http://127.0.0.1:8080/api/v1/chat/completions
```

`stream: true` returns `text/event-stream` chunks terminated by `data: [DONE]`.
Reasoning text is streamed as a `reasoning_content` delta. Per-request
`reasoning_effort` overrides the stored thinking level.

The compatibility surface is deliberately small. `model`, `messages`, `stream`,
and `reasoning_effort` are the only request fields read; **anything else is
ignored, not rejected** — including `temperature`, `top_p`, `max_tokens`,
`stop`, `n`, and a client-supplied `tools` array, since the agent's own tool set
is the one that runs. Message content may be a string or an array of parts, but
only the `text` of each part is kept, so images sent over the API are dropped
(the Discord front end does handle them). Request bodies are capped at 1 MiB and
concurrent completions at 16, beyond which the server answers `429`.

The server is stateless: it never reads or writes the SQLite session store, and
`messages` in the request is the entire history. Trimming is the client's job,
so the `context` budget does not apply here.

Only safe tools are exposed over HTTP. `current_time`, `web_search`, `web_fetch`,
and `skill` run server-side and are invisible to the client; `file_read`, `file_write`, and
`bash_exec` are never offered, because HTTP has no approval prompt and would
otherwise hand unattended shell access to any client that reaches the port.
`--allow-dangerous-tools` does not affect the server. MCP tools follow the same
rule: only ones classified safe are exposed, and a server configured with
`--no-daemon` is skipped entirely.

## Development

```sh
make fmt         # gofmt check, fails on unformatted files
make vet         # go vet
make test        # fmt + vet + unit tests
make test-race   # the same suite under the race detector
make test-cover  # race + coverage, writes out/coverage.out
make test-all    # test-race and test-cover together
```

`make test-race` is what CI runs on every push and pull request.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the package layout, how the
TUI and server share one tool-calling loop, and the testing patterns.
[docs/CONVENTION.md](docs/CONVENTION.md) defines the code style that `make test`
enforces.
