# Usage

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
`memory_*` tools rather than hand-edited; uploaded chat images live under
`.mininaru/attachments/` with a row per file in the `attachments` table
(cascades with its session); the daily background update check caches the
latest known release tag in `.mininaru/update.json`.

## Set up a provider and an agent

A `provider` is a base URL and API key for an OpenAI-compatible endpoint —
every registered provider is usable at once, there's no "active" one to
switch. An `agent` names its model as `<provider>:<model>` (e.g.
`openai:gpt-4o-mini`), plus an optional system prompt (`--soul`), reasoning
effort, and context budget. Exactly one agent is the "primary" one at a time,
managed with `mininaru agent primary`; the first agent you create becomes
primary automatically, and it's the agent the REPL, `-p`, and a gateway's
"+ new session" connect to when none is named explicitly.

```sh
mininaru provider add openai --base-url https://api.openai.com/v1 --api-key '<KEY>'
mininaru agent add naru --model openai:gpt-4o-mini --soul 'terse and precise'
```

`--model` also accepts `--pick-model` instead of a value — it lists every
configured provider's models (queried live from each provider's own
`/v1/models`) and lets you choose one by number:

```sh
mininaru agent add naru --pick-model --soul 'terse and precise'
mininaru agent set naru --pick-model
```

`show`/`set`/`remove` all take either the id or the name printed by `add`.
`provider set`/`agent set` only change the fields you pass; leave a flag out
to keep the current value.

`--max-context` is the agent's total input-context budget in tokens (default
24,000). mininaru reserves 20% for output, summarizes older completed session
turns when needed, and keeps the original history in its local database. The
stateless OpenAI-compatible API does not rewrite supplied messages and returns
`context_length_exceeded` when they exceed the configured budget.

```sh
mininaru provider list
mininaru provider set openai --base-url https://api.openai.com/v1
mininaru provider remove openai

mininaru agent list
mininaru agent set naru --thinking high
mininaru agent set naru --max-context 48000
mininaru agent primary naru
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
mininaru serve --cors-origin tauri://localhost --cors-origin http://localhost:1420
mininaru serve --web-dir ./dist             # also serve a built web client at /
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

A browser can't set `Authorization` on a websocket, so `/ws` also accepts the key
as a subprotocol: `new WebSocket(url, ["bearer.<KEY>"])`. The server echoes it back
and the key never appears in a URL. REST requests ignore this header.

`--cors-origin` (repeatable) lets a web page or the desktop client on another
origin call the API; preflight requests are answered before the key check, and
an origin that isn't listed gets no CORS headers. Nothing is allowed by default.
`--web-dir` serves a built client from a directory at `/`, falling back to its
`index.html` for unknown paths; `/api` and `/ws` are never shadowed.

## Tools

Which tools a session gets depends on whether it has a working directory.
A session opened from the CLI is pinned to one — the directory you started
in, or whatever `--cwd` names — and gets everything below; that pin is
taken from the first message and never moves, so the agent keeps resolving
paths against the directory you opened even if you carry on from somewhere
else. A session with no directory, which is what you get over the REST API
unless you pass a `cwd`, runs without `bash_exec` or the file tools at all;
the browser, memory, skill, MCP, and delegation tools still work. A remote
client may only pin a directory under the server's home; anything else is
refused and the session stays without one.

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
mininaru --cwd ~/src/project                      # pin the session to another directory
```

Every line you type goes straight to the agent — there's no shell mode to
switch into; `/bash`/`/!bash` cover running a one-off shell command instead
(see [Tools](#tools) above for what the agent itself can run). A session is
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
/usage      refresh and show the current context input budget usage
/compact    summarize completed conversation turns
/bash       run one shell command; the command and its output are posted to the agent
/!bash      same, without sharing the output with the agent
/session    show or switch the current session
/gateway    pick a saved gateway and a session on it, then reconnect
/img        attach an image file to your next message
/model      pick a provider:model from a live-fetched numbered list, or set one directly
/effort     change the connected agent's reasoning effort (off|low|medium|high|max)
/yolo       set dangerous-tool trust for this directory (off|persist|on)
```

The first prompt line shows `ctx:used/limit (percent)` next to the Git branch.
It is refreshed after each turn, session change, and `/usage`; `limit` is
80% of `max_context` (mininaru reserves the rest for output). Before a
session's first completed round, `used` is a local estimate; after that it's
the provider's own reported `prompt_tokens + completion_tokens` for the last
round. A cyan `cache:percent` segment is appended when the provider reports
cached prompt tokens.

Input history is a plain text file, `.mininaru/history` by default (or
`$NARU_HISTFILE`); `$HISTSIZE`/`$HISTFILESIZE` cap what's kept in memory and
written back, the same way bash honors them.

## Non-interactive (`-p`)

```sh
mininaru -p "<prompt>"                            # one-shot, streams the transcript
mininaru -p "<prompt>" --session <id>             # run the turn on an existing session
mininaru -p "<prompt>" --format json              # -f json | xml | string (default)
mininaru -p "<prompt>" --image shot.png           # attach an image, repeatable
mininaru -p "<prompt>" --cwd ~/src/project        # run the turn against another directory
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

`--no-cache` (root flag, works for both the REPL and `-p`) disables provider
prompt caching for that one run only — nothing is written back to the agent
or session. By default mininaru marks the conversation's fixed system prefix
with an Anthropic-style `cache_control` breakpoint so providers that support
prompt caching (e.g. Claude behind a compatible gateway) can reuse it.

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
mininaru agent primary <name> --gateway prod
```

Management commands hit the remote's `/api` for reads (`list`, `show`) and for
`session remove` / `provider remove` / `agent primary`. `add` and `set` stay
local — configure a remote box on that box. `mcp` and `skill` have no remote API
and are always local.

`--gateway` cannot be combined with an explicit `--url`. With neither,
everything is local: `ws://127.0.0.1:8223/ws` and the local SQLite DB.

Inside the REPL, `/gateway` opens an arrow-key picker over the saved
gateways, then over that gateway's sessions (or `＋ new session`), and
reconnects the shell to the chosen one — no restart needed.

See [docs/API.md](API.md) for the HTTP/websocket API this all sits on top of.
