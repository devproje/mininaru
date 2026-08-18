# mininaru

<img src="assets/logo.png" alt="mininaru" width="80" align="right">

Local-first multi-provider, multi-agent terminal runtime — MCP tools, skills, persistent memory, an OpenAI-compatible server, and a Discord frontend, all in one Go binary.

## Install

`scripts/install.sh` fetches the binary for your platform from this
repository's [GitHub releases](https://github.com/devproje/mininaru/releases),
verifies it, and puts it on your `PATH`. No Go toolchain, no clone.

```sh
curl -fsSL https://raw.githubusercontent.com/devproje/mininaru/master/scripts/install.sh | sh
```

It installs to `~/.local/bin`, checks the download against the release's
`SHA256SUMS` and refuses to install anything that does not match. Nothing is
written outside your home directory and sudo is never used.

**Run the same line again to update.** It reads the version you already have
and stops without downloading anything when that is already the release being
asked for.

Options go after `-s --`:

```sh
URL=https://raw.githubusercontent.com/devproje/mininaru/master/scripts/install.sh

curl -fsSL $URL | sh -s -- --version v0.2.0   # pin a release
curl -fsSL $URL | sh -s -- --force            # reinstall the same one
curl -fsSL $URL | sh -s -- --bin-dir ~/bin
curl -fsSL $URL | sh -s -- --uninstall
```

Prefer not to pipe a script into a shell? Download it, read it, then run it —
it is one file, POSIX `sh`, with no dependencies beyond `curl` or `wget` and
`tar`.

```sh
curl -fsSLO https://raw.githubusercontent.com/devproje/mininaru/master/scripts/install.sh
less install.sh
sh install.sh
```

Linux and macOS, amd64 and arm64. On Windows, take the `.zip` from the
[releases page](https://github.com/devproje/mininaru/releases) and put
`mininaru.exe` on your `PATH`; the `daemon` command needs systemd and will not
work there, but the chat client and `serve` do.

### Verifying a download yourself

Every release ships `SHA256SUMS` and a signed build provenance attestation.

```sh
sha256sum -c SHA256SUMS --ignore-missing
gh attestation verify mininaru_v0.2.0_linux_amd64.tar.gz --repo devproje/mininaru
```

The attestation proves the archive came out of this repository's release
workflow, which a checksum alone cannot tell you.

## Build from source

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

`make dist GOOS=linux GOARCH=arm64` cross-compiles a single release layout into
`dist/`, which is what the release workflow runs for each target.

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

## Updating

`mininaru update` replaces the running executable with a published release
build, and restarts the user daemon afterwards if one is installed.

```sh
mininaru update                 # install the latest release
mininaru update --check         # print the installed and latest versions only
mininaru update --tag v0.3.0    # install a specific release
mininaru update --force         # reinstall the version already running
mininaru update --no-restart    # leave the daemon alone
```

The archive is checked against the release's `SHA256SUMS` **before** anything is
replaced, and the verified file is moved into place with a rename. A failed
download, a checksum mismatch, or a broken archive therefore leaves the current
executable exactly as it was -- there is no window where the binary is half
written.

The new file is staged in the same directory as the executable it replaces, so
updating a binary somewhere you cannot write (`/usr/local/bin` as a normal user)
fails immediately with a permission error rather than half way through. Install
it yourself in that case, or keep mininaru under `~/.local/bin`.

Linux and macOS only. On Windows the command refuses and points at the releases
page, because the daemon it would restart is a systemd unit anyway.

### Update notices

Once a day, at most, mininaru asks GitHub for the latest release tag in the
background and caches the answer in `update.json` under your data directory. The
check never blocks a command: the result is written for the *next* run, which is
when the notice appears under `--version` and at the top of the chat client.

```
a newer version is available: v0.4.0 (run `mininaru update`)
```

Builds reporting `dev` never show it. To turn the check off entirely, set
`update.check` to `false` in `client.json`, or export
`MININARU_NO_UPDATE_CHECK=1` for a single environment such as CI.

## First run

`setup` first asks whether this installation is a server or a paired client.
Client setup asks for the remote gRPC address, verifies its fingerprint, and
waits for pairing approval. Server setup walks through provider, agent, thinking
and tool defaults, and finally the systemd user daemon -- asking one question at
a time:

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

To do it by hand instead, add a provider and then the global chat agent. The
default provider kind is an OpenAI-compatible Chat Completions endpoint:

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

Native Anthropic Messages API providers use `--kind anthropic`. Its base URL is
the API origin, without the OpenAI-style `/v1` suffix:

```sh
mininaru provider add \
  --name anthropic \
  --kind anthropic \
  --base-url https://api.anthropic.com \
  --api-key '<API_KEY>' \
  --cache ephemeral

mininaru agent add --name claude --provider anthropic --model claude-sonnet-4-6
```

`--cache` accepts `auto`, `off`, `ephemeral`, or `ephemeral_1h`. `auto` leaves
OpenAI and other provider-native automatic caches alone, enables Anthropic's
automatic five-minute cache, and adds the same cache control for Claude models
routed through OpenRouter. The explicit ephemeral modes add top-level
`cache_control`; use `ephemeral_1h` only when the longer, more expensive cache
write is worthwhile.

OpenRouter can also cache an entire identical API response. This is separate
from prompt caching and is deliberately opt-in because it can return a previous
answer without running the model:

```sh
mininaru provider add \
  --name openrouter \
  --base-url https://openrouter.ai/api/v1 \
  --api-key '<API_KEY>' \
  --cache auto \
  --response-cache \
  --response-cache-ttl 300
```

Response caching stores request/response data temporarily and is unavailable
with OpenRouter account-level Zero Data Retention. Keep it off for sensitive or
time-dependent conversations.

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
mininaru session usage         # what the latest session has spent
mininaru session usage <id>
mininaru session remove <id> --agent coder
mininaru session rename <id> --name 'New name'
mininaru --session             # resume the latest non-empty session
mininaru --session <id>        # resume a specific session
mininaru --agent coder         # chat with an agent other than the global one
mininaru thinking high --show
mininaru context               # show context management settings
mininaru context compact off   # disable automatic summarisation
mininaru tools list            # list every available tool and where it came from
mininaru tools on              # enable tool calling (default)
mininaru tools off             # disable for models without tool support
mininaru mcp list              # configured mcp servers and their connection state
mininaru skill list            # installed skills and which root they came from
mininaru skill show <name>     # exactly what the skill tool would return
mininaru skill uses            # which skills the model has actually loaded
mininaru web show              # search provider, endpoint, and masked api key
mininaru bot list              # chat bot front ends the daemon starts
mininaru update --check        # compare the running build against the latest release
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

Inside the TUI, use `/help`, `/thinking`, `/usage`, or `ctrl+t`. `/compact` folds the
conversation so far into a summary straight away, without waiting for the
model context window to force it; token usage refreshes after the next response.
Press `esc` to interrupt the current response, and
`/exit`, `/quit`, or `ctrl+c` to leave. The client runs in the terminal's
alternate full-screen buffer; use `PageUp` and `PageDown` to scroll the
transcript and `End` to return to the latest message. Completed assistant
messages render as Markdown, while thinking output is shown as a quoted block.

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
up. The TUI records the last provider-reported input token count. For providers
that expose the model's context window, such as llama-server's `/props`, it also
shows the capacity and uses it to decide when automatic compaction is needed.

At 90% of a known model context window, completed turns are **summarised**. The summary is one running
paragraph per conversation, rewritten rather than appended to each time more
turns fall out, and it rides in the system prompt so what was decided earlier
survives even though the wording does not. It costs one extra model call, and
only on a turn where automatic compaction runs. `mininaru context compact off`
turns automatic summarisation off without imposing a separate local history
limit; a summary already saved for a conversation keeps being used either way.
Summaries live in their own table and are deleted with their session.

### What a conversation costs

Every model call made on a session's behalf is recorded against it, so
`session list` carries a `TOKENS` column and `mininaru session usage` breaks the
total down by what spent it. `/usage` reports the running total inside the TUI.

```
KIND         PROMPT  CACHE READ  CACHE WRITE  COMPLETION   TOTAL
turn        120,411      94,208       12,000       8,204  128,615
compaction    6,890       4,096        1,024         412    7,302
subagent     31,204      24,576        2,048       2,110   33,314
total       158,505     122,880       15,072      10,726  169,231
```

`turn` is the answers you asked for, `compaction` is the summarising above, and
`subagent` is one agent delegating to another — the two things that spend tokens
without you asking directly, which is exactly why they are itemised.

A turn that takes several tool rounds sends the whole conversation again on each
one, and each is billed, so its total is the sum of every round rather than the
last. The same applies to what the HTTP API reports back: the `usage` in a
response covers every round the server ran on the caller's behalf.

`CACHE READ` is the subset of prompt tokens served from a provider cache, while
`CACHE WRITE` is the number written into a new cache entry. OpenAI-compatible
providers report these as `cached_tokens` and `cache_write_tokens`; Anthropic
reports `cache_read_input_tokens` and `cache_creation_input_tokens`. A zero
means the provider reported no cache activity (or the prompt was shorter than
that model's cache minimum), not that mininaru maintains a second local cache.

**These are tokens, not money.** mininaru talks to whatever provider you point it
at and has no idea what yours charges, so the conversion is yours to do. Usage
rows are deleted with their session, and a provider that does not report usage
simply records nothing.

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

Every tool reaches the model over MCP. The thirteen built-in tools are served by
an MCP server running inside the mininaru process, and additional servers can be
configured in `mcp.json`.

Safe built-in tools are exposed through the OpenAI-compatible function-calling
protocol: `current_time`, `web_search`, `web_fetch`, and `skill`. See Web tools
below for the two network ones.

`file_read`, `file_write`, `file_edit`, `glob`, `grep`, and `bash_exec` are rooted
at the directory where the process started. They reject lexical and symlink path
escapes where applicable. Without a flag, each dangerous call pauses the TUI and
asks for approval. Move with the arrow keys and choose with `enter`:

```
  approve bash_exec?  {"command":"go test ./..."}
  ▸ Allow once
    Allow bash_exec for the rest of this session
    Deny
```

`esc` denies without moving. A denied call is returned to the model as a tool
error so the conversation can continue.

The middle choice is what stops a long turn from asking ten times for the same
thing. **It is scoped to the tool, not to the arguments**, so allowing
`bash_exec` for the session means every later shell command runs unattended —
the same reach as `--allow-dangerous-tools`, narrowed to that one tool. The
choice names the tool so it is clear what is being handed over. It lasts as long
as the process does and is never written to disk, so a new `mininaru` starts by
asking again.

`file_edit` replaces one exact string with another. The string has to occur
exactly once in the file, otherwise the call is refused and the model is asked for
more surrounding context; `replace_all` lifts that restriction. `file_read` takes
`offset` and `limit` to read a range of lines rather than a whole file. Reading an
existing file also records its content revision. A later `file_edit`, overwrite,
or append is refused if that read never happened or another process changed the
file in between. Writes use an atomic replacement, preserve an existing file's
mode, reject binary content, and return the applied unified diff. New files can
still be created without a preceding read.

`glob` lists files by path pattern, where `**` matches across directories, as in
`**/*.go`. `grep` searches file contents by regular expression and answers with
`path:line:text`. Both walk from the startup directory, never follow a symlink out
of it, and skip dot-directories, `node_modules`, `vendor`, binaries, and anything
over 5 MB.

Passing `--allow-dangerous-tools` bypasses every approval prompt for that run.
Treat this flag as unattended access to files and shell commands under your user
account; use it only in a dedicated working directory.

`bash_exec` runs through `bash`, falling back to `sh` when there is no bash on
`PATH`; set `MININARU_SHELL` to pick a different one. A timed-out command has its
whole process group killed, so a backgrounded child cannot outlive the call or
hold the tool open past its timeout.

`memory`, `skill_create`, and `agent_call` are the three **privileged** built-ins.
They run without an approval prompt, because the front ends that can reach them
are already trusted: the TUI and a paired Discord admin. They are refused
outright anywhere else, so none is offered over the HTTP API and a regular
Discord user cannot call them.

`memory` stores durable facts in a small global SQLite table shared by the
interactive front ends, capped at 4096 characters in total.

`agent_call` hands one self-contained task to another agent you have configured
and returns its answer as the tool result. The named agent answers with its own
model, role and soul, which is the point: a cheap model can hold the
conversation and hand the parts that need a stronger one over to it, or a
reviewer persona can look at something without the main agent's history
colouring it.

The subagent starts with **no memory of the conversation** — it sees only the
prompt it is given, so that prompt has to carry everything it needs. It inherits
the calling turn's tools and approval policy unchanged, so a dangerous tool it
reaches still raises the same prompt the caller would have raised. The one tool
it does not inherit is `agent_call` itself: delegation is one level deep, and an
agent cannot delegate to itself.

`skill_create` writes a skill bundle to disk and reloads the catalog. It is
privileged rather than dangerous because what it writes is not just a file: the
new skill joins the catalog in every later system prompt, which is the same
durable-state-feeding-the-prompt shape as `memory`, with more reach.

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

### Creating a skill from a conversation

The privileged `skill_create` tool lets the model write a bundle for you. It
takes `name`, `description`, and `body` — the frontmatter is generated, so the
model never hand-writes the YAML — plus an optional `scope` (`project`, the
default, or `user`) and `overwrite`.

Every value is checked against the rules the loader itself uses, so the tool
cannot produce a bundle that then fails to load: the name must match
`^[a-zA-Z0-9_-]{1,64}$`, the description is collapsed to one line and truncated
at 200 characters, and the body is capped at 64 KiB. Writing over an existing
skill needs `overwrite`, and even then only within the same scope — a project
skill cannot silently displace your personal one.

After a successful write the catalog is reloaded, so the new skill is listed
from the next turn onward without restarting anything.

### Tracking which skills were used

Every successful `skill` call is recorded with the resolved scope and bundle
path, including calls made over the HTTP API where there is no session to attach
them to.

```sh
mininaru skill uses                  # NAME / SCOPE / USES / LAST USED
mininaru skill uses --session <id>   # only loads from one session
```

The scope column reads `removed` when a skill has been used in the past but no
longer exists on disk. In the TUI and the `-p` log a skill load is labelled
`skill - <name>` instead of the raw arguments, and reading a companion file
shows as `skill - <name>/<file>`.

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

The same command starts the native session-aware gRPC server on
`127.0.0.1:9090`. Its listener is independent from the HTTP API:

```sh
mininaru serve --api-key '<KEY>' --grpc-host 0.0.0.0 --grpc-port 9090
mininaru serve --grpc-only --grpc-host 0.0.0.0
```

The HTTP API remains stateless and API-key authenticated. The native gRPC API
owns sessions on the server and accepts only paired client devices over mutual
TLS; HTTP credentials and gRPC identities are not interchangeable.

### Pairing a gRPC client

The server creates a local Ed25519 certificate authority and server identity on
first start. Its public-key fingerprint is printed when the gRPC listener
starts. On the client machine, compare that value while pairing:

```sh
mininaru pair naru.example.com:9090 --name laptop
```

The client shows the server fingerprint before trusting it, creates its own
Ed25519 key locally, and prints a six-digit code plus its client fingerprint.
The server operator approves that request on the server host:

```sh
mininaru client pending
mininaru client approve 482193
```

Codes expire after five minutes and pairing attempts are rate limited. The
client private key never leaves the client; approval returns a client
certificate signed by the server's local CA. Pair non-interactively only when
the expected fingerprint came through another trusted channel:

```sh
mininaru pair naru.example.com:9090 \
  --name ci-runner \
  --fingerprint 'SHA256:...'
```

Successful pairing writes the server address to `client.json`, so ordinary
commands use it automatically. `--server` overrides that default:

```sh
mininaru
mininaru -p 'summarise the current session' --session
mininaru session list
mininaru session usage <id>
mininaru --server naru.example.com:9090
```

The TUI streams answer and reasoning deltas, tool progress, cancellation, and
dangerous-tool approval over one bidirectional RPC. Sessions, history, tool
logs, compaction, and token usage stay in the server's SQLite database.

Manage paired devices on the server host:

```sh
mininaru client list
mininaru client deny 482193
mininaru client revoke 'SHA256:...'
```

Revocation takes effect on the next RPC, including over an already open HTTP/2
connection. Server CA keys and server keys live under `.mininaru/pki`; client
keys and certificates live under `.mininaru/identity`. Private material and the
trust store are written with mode `0600`.

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
answers every authorized message in a DM without requiring a mention. A mention
from someone who is not paired is ignored without a reply, so an unauthorized
attempt leaves nothing in the channel; run the daemon at `--log-level debug` to
see who tried. Slash commands do answer, privately, because Discord shows its
own failure notice when an interaction goes unanswered. A guild
mention starts a thread when Discord permits it and falls back to the current
channel if thread creation fails. Messages inside a thread created by the bot
do not need to mention it again. It shows Discord's typing indicator while
generating, then sends the completed reply as an ordinary message, split across
Discord's 2000 character limit when it has to be. When the answer lands in the
same channel as the question -- a DM, or a follow-up inside a thread -- it is
sent as a reply to that message, without pinging its author. A reply that opens
a new thread has no reference to make: the question is already the thread's
first message.

Alongside the reply the bot keeps one **execution card** per turn. Its heading
is what the agent is doing right now -- thinking, reasoning, or the tool it is
running -- and underneath it a line is added for each tool as it finishes, with
the reason when one fails. When the turn ends the heading becomes its outcome
and the card stays as the record of what happened. The answer itself is never
put inside the card: a reply is a normal message so it reads like one. A single reply is capped at ten minutes, and stopping the daemon cancels
whatever turns are still running instead of leaving them to finish unobserved.

Discord messages can include up to four supported attachments. PNG, JPEG, GIF,
and WebP images are sent as vision input; text, source code, JSON, and similar
text formats are included as text; PDF files are sent as file input. Each file
is limited to 10 MiB and the combined input to 20 MiB. Only HTTPS Discord CDN
attachment URLs are downloaded. `/chat` also accepts one optional `attachment`.

Each channel is bound to one session, so a channel is a running conversation
with all the history, tool replay, and context trimming the TUI gets.

Because a channel is shared, every message reaching the model is prefixed with
who sent it and what they are:

```
[discord from=<@123456789> role=user]
what does this error mean?
```

Admins are marked `[discord role=admin]` without a mention, on the assumption
that the model knows who the admin is from memory. That prefix is stored with
the turn, so the model can still tell the speakers apart when the conversation
is replayed later. Several admins in one channel are not distinguishable from
each other.

This is context, not access control. A user who types the prefix themselves
gets it stripped and replaced with their real role, but even if one slipped
through it would change nothing: permissions are enforced by which tools are
handed to the model, never by what the prompt claims. Dangerous tools are not
in the list a non-admin's turn is given.

- `/reset` starts a fresh conversation in the channel
- `/agent` shows which agent answers there, `/agent <name>` switches it
- `/mention` shows whether the bot may ping you, `/mention on|off` sets it
- `/compact` folds that channel's conversation into a summary. **Admin only**,
  and it only ever touches the conversation bound to the channel it was run in,
  so there is no way to reach another channel's history with it. A channel that
  has no conversation yet is told so rather than having one created.
- `/usage` shows what that channel's conversation has spent, itemised by turns,
  compaction, and delegation. **Admin only**, scoped to the channel the same way
  `/compact` is, and the reply is only visible to whoever asked. See
  [What a conversation costs](#what-a-conversation-costs) — these are tokens,
  not money.

### Pings

**The bot pings nobody by default.** Every message it sends goes out with an
allowed-mentions list, so `@everyone`, `@here`, and role mentions never fire no
matter what the model writes, and a mention of a person renders as a link
without a notification.

Being pinged is opt-in and per person: `/mention on` adds you to the list of
users the bot may notify, and only you. The setting survives a change of role
and applies to every channel that bot serves. `/mention off` puts it back.

This is not a filter on the text — mentions still appear in the message and the
model is free to write them. Discord is simply told which of them are allowed
to notify anyone, which is why `@everyone` cannot fire even if the model is
talked into typing it.

Switching agents starts a new conversation; the old one keeps its history and
simply stops being the channel's live session. The bot's `--agent` picks which
agent new channels start with, defaulting to the global agent. `--guild`
registers the slash commands to a single guild, which applies them immediately
instead of taking up to an hour to propagate globally.

Regular users can only use safe daemon tools. Admins can also use dangerous
tools, but each dangerous call requires an explicit Approve/Deny click in
Discord and expires after five minutes. Only the admin who made the request can
answer its approval prompt. Admins also reach the privileged `memory` and
`skill_create` tools, which regular users never see.

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
and `skill` run server-side and are invisible to the client; `file_read`,
`file_write`, `file_edit`, `glob`, `grep`, `bash_exec`, `memory`, `skill_create`,
and `agent_call` are never offered, because HTTP has no approval prompt and would
otherwise hand unattended shell access to any client that reaches the port.
`--allow-dangerous-tools` does not affect the server. MCP tools follow the same
rule: only ones classified safe are exposed, and a server configured with
`--no-daemon` is skipped entirely.

## Development

```sh
make generate    # regenerate protobuf and gRPC Go sources
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
enforces. [docs/AGENTS.md](docs/AGENTS.md) is what to hand an AI coding agent
before it touches this repository.

## License

mininaru is free software under the **GNU General Public License, version 3 or
later** (`GPL-3.0-or-later`). See [LICENSE](LICENSE) for the full text.

Version 3 rather than 2 because the binary links `cobra`, `openai-go`, and the
MCP SDK, which are Apache-2.0 — a licence compatible with GPLv3 but not with
GPLv2.

**The artwork is not covered by the GPL.** `assets/logo.png` and the naru
character are all rights reserved; see [COPYRIGHT.md](COPYRIGHT.md) for what
that allows. The software is unaffected by this: nothing in the program depends
on the artwork.

[CONTRIBUTING.md](CONTRIBUTING.md) covers how to send a change, and
[SECURITY.md](SECURITY.md) is where to report a vulnerability privately —
please do not open a public issue for one.
