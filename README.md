# mininaru

Lightweight terminal chat client for OpenAI-compatible LLM APIs. It ships as one
Go binary and stores configuration and conversation history locally.

## Build

Requires the Go version declared in `go.mod`.

```sh
make build
./out/mininaru --version
```

## First run

Add an OpenAI-compatible provider, then create the global chat agent:

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

## Common commands

```sh
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
mininaru tools list            # list built-in tools
mininaru tools on              # enable tool calling (default)
mininaru tools off             # disable for models without tool support
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
with mode `0600`; chat sessions are stored in SQLite with WAL enabled.

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

## Tools

Safe built-in tools are exposed through the OpenAI-compatible function-calling
protocol. `current_time` accepts an optional IANA timezone, while `web_search`
returns public web result titles, URLs, and snippets. Web search uses
DuckDuckGo's non-JavaScript search endpoint and does not store an API key.

`file_read`, `file_write`, and `bash_exec` are rooted at the directory where the
process started. They reject lexical and symlink path escapes where applicable.
Without a flag, each dangerous call pauses the TUI and asks for approval: press
`y` to execute or `n`/`esc` to deny it. A denied call is returned to the model as
a tool error so the conversation can continue.

Passing `--allow-dangerous-tools` bypasses every approval prompt for that run.
Treat this flag as unattended access to files and shell commands under your user
account; use it only in a dedicated working directory.

Tool calls and results are recorded in SQLite and the model may perform at most
eight tool rounds for one user request. The TUI prints a compact log when a tool
starts and when it completes or fails. Calls are inserted as `pending` before
approval or execution, then updated to `completed` or `failed`; stored logs are
shown again when the session is resumed. Arguments and results may contain
sensitive data, so protect the local database accordingly.

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
share one daemon safely. Agent and provider settings are read at startup; send
`SIGHUP` to pick up later changes without restarting:

```sh
mininaru agent add --name coder --model qwen
kill -HUP $(pgrep -f 'mininaru serve')
```

Requests already in flight finish against the configuration they started with.

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

For a throwaway run you can skip the config entirely with
`--discord-token` or `MININARU_DISCORD_TOKEN` (plus `--discord-agent` and
`--discord-guild`). Those flags **replace** the configured bots for that run
rather than adding to them. If any bot fails to start, `serve` stops the ones it
already started and exits with the failing bot's name.

In the
Discord developer portal the application needs the **Message Content Intent**
enabled — it is privileged, and without it the bot receives empty message bodies
and never answers. Invite it with the `bot` and `applications.commands` scopes.

The bot answers when it is mentioned in a server channel, and answers every
message in a DM. It shows Discord's typing indicator while generating, then
sends the completed reply split across Discord's 2000 character limit.

Each channel is bound to one session, so a channel is a running conversation
with all the history, tool replay, and context trimming the TUI gets.

- `/reset` starts a fresh conversation in the channel
- `/agent` shows which agent answers there, `/agent <name>` switches it

Switching agents starts a new conversation; the old one keeps its history and
simply stops being the channel's live session. The bot's `--agent` picks which
agent new channels start with, defaulting to the global agent. `--guild`
registers the slash commands to a single guild, which applies them immediately
instead of taking up to an hour to propagate globally.

Only safe tools are available, for the same reason as the HTTP API: a daemon has
nobody to approve a file write or a shell command.

The `model` field names a **mininaru agent**, not an upstream model. The server
resolves the agent, uses its provider and model, and prepends its role and soul
as the first system message, so a client selecting `naru` in a model picker gets
that agent's persona. `GET /api/v1/models` lists every configured agent by name.

```sh
curl -H 'Authorization: Bearer <KEY>' http://127.0.0.1:8080/api/v1/models

curl -H 'Authorization: Bearer <KEY>' -H 'Content-Type: application/json' \
  -d '{"model":"naru","messages":[{"role":"user","content":"hello"}]}' \
  http://127.0.0.1:8080/api/v1/chat/completions
```

`stream: true` returns `text/event-stream` chunks terminated by `data: [DONE]`.
Reasoning text is streamed as a `reasoning_content` delta. Per-request
`reasoning_effort` overrides the stored thinking level.

The server is stateless: it never reads or writes the SQLite session store, and
`messages` in the request is the entire history. Trimming is the client's job,
so the `context` budget does not apply here.

Only safe tools are exposed over HTTP. `current_time` and `web_search` run
server-side and are invisible to the client; `file_read`, `file_write`, and
`bash_exec` are never offered, because HTTP has no approval prompt and would
otherwise hand unattended shell access to any client that reaches the port.
`--allow-dangerous-tools` does not affect the server.

## Development

```sh
make test
```

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the package layout, how the
TUI and server share one tool-calling loop, and the testing patterns.
[docs/CONVENTION.md](docs/CONVENTION.md) defines the code style that `make test`
enforces, and [TODO.md](TODO.md) tracks the roadmap.
