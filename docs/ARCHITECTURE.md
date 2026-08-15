# Architecture

mininaru is a single Go binary that talks to OpenAI-compatible providers. It
ships two front ends over one core: a terminal chat client and an HTTP API.

## Packages

```
cli/       cobra commands, the bubbletea TUI, and the serve daemon
core/      providers, agents, sessions, messages, the tool-calling loop
modules/   tool implementations, the in-process MCP server exposing them, the
           MCP client manager, mcp.json, web.json, and skill discovery
server/    stateless OpenAI-compatible HTTP API
bot/       chat front ends that live inside the daemon (Discord)
config/    client.json preferences (thinking, context budget, tool switch)
util/      data directory layout, SQLite handle, migrations, version info
```

Dependencies point one way: `cli` depends on `server` and `bot`, both of which
depend on `core`, and `core` depends on `modules`, `config`, and `util`. Nothing
in `core` imports its callers, and `server` and `bot` do not import each other —
`cli/serve.go` is the only place that knows about both. `modules` stays a leaf:
it imports `util` and the MCP SDK and nothing else in the tree. MCP process
lifetime is owned by `cli`; `core` never starts or stops anything.

One tool needs to point the other way, and does it without an import.
`agent_call` lives in `core` because it drives the completion loop, and reaches
the model by calling `modules.RegisterBuiltin` rather than by `modules` knowing
anything about it — see Delegation.

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
| Context budget | `compactHistory` then `trimHistory` | client's responsibility |
| Token accounting | recorded against the session | returned in the response |

Both tool sets are a snapshot of what was discovered over live MCP sessions —
the builtin thirteen plus every enabled `mcp.json` server. There is no non-MCP path
to a tool. The `core.Chat` row is also gated on `config.Client.Tools.Enabled`:
when tools are off, `defs` is empty and the loop degenerates to one round.

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

## The system prompt

`systemPrompt` in [core/prompt.go](../core/prompt.go) is the only place a system
message is built, and both chat paths call it. It emits a
`<mininaru-runtime>` block carrying `util.RuntimeIdentity()` — the same string
`--version` prints, so the two can never drift — followed by rules that claim
precedence over the persona. An `<mininaru-agent>` block follows with only the
active agent's id and name, then the prompt adds the agent's `Role` and `Soul`.

Four properties are deliberate:

- It is **unconditional**. An agent with no persona still gets the block; before
  this, such an agent was sent no system message at all.
- It is **one** system message, not two. Several local OpenAI-compatible servers
  merge system messages or honor only the first, so splitting the pin from the
  persona would make the pin transport-dependent.
- It is **first**, ahead of the client's own messages on the server path. The
  HTTP API accepts a `system` message from the caller; it lands after the pin.
- The skill catalog sits between the pin and the persona, and appears **only when
  the request also carries the tool that can load a skill**. `systemPrompt` takes
  the request's `defs` for exactly that reason — see Skills below.

`trimHistory` is charged the full prompt length, so neither the runtime block nor
the catalog can silently eat the context budget.

## MCP

Every tool reaches the model through an MCP client session. The thirteen builtin
tools are served by an in-process MCP server wired to the client over
`mcp.NewInMemoryTransports()` — no subprocess, no socket, but the same code path
external servers take. It bootstraps lazily on the first `DefaultTools()` call,
which is why one-shot commands and tests that never call `MCPInit` still get
tools.

External servers are declared in `mcp.json` and reached over one of two
transports: `stdio` (a child process, `mcp.CommandTransport`) or `http`
(`mcp.StreamableClientTransport`). Configured headers are applied by an
`http.RoundTripper` wrapper because the SDK's transport has no header field.

Names are qualified as `server__tool`; builtin tools keep their bare names so
`tool_calls` rows written before MCP still resolve on replay. OpenAI requires
`^[a-zA-Z0-9_-]{1,64}$`, so out-of-charset bytes become `_` and an over-long
name is truncated and suffixed with a hash of the original. Collisions are
resolved first-writer-wins, builtin first, and the loser is dropped with a
warning; reverse mapping is an exact lookup, never a re-parse of the name.

A `CallToolResult` becomes the `(string, error)` that `Def.Execute` must return:
text content is joined, image and audio content are replaced by a placeholder so
base64 never lands in `tool_calls.result` and gets replayed every turn, and
`IsError` becomes an error — which `executeTool` turns into `MessageFailed` and
`"error: " + text`, exactly as a direct tool failure did.

Startup deliberately degrades rather than dies: a server that fails to connect
is reported on stderr and skipped, and the rest of the tools still work. That is
the opposite of the bot policy, where a failed start aborts `serve` — a missing
tool is a smaller loss than a chat front end that silently isn't there.

## Skills

A skill is a directory holding a `SKILL.md` with YAML frontmatter (`name`,
`description`) plus optional companion files. Discovery lives in
[modules/skill.go](../modules/skill.go), the tool in
[modules/skilltool.go](../modules/skilltool.go).

**Progressive disclosure.** The system prompt carries one line per skill —
`name: description` — and nothing else. The body is fetched only when the model
calls the `skill` tool. The alternative, one MCP tool per skill, was rejected on
cost: the catalog is one line per skill, whereas a tool per skill puts a full
JSON Schema per skill into *every* request whether or not it is relevant.

**Two roots, project wins.** `util.Path("skills")` (project — recall `util.RootDir`
is already `NARU_PATH` or `./.mininaru`) then `$HOME/.mininaru/skills` (user).
Both are resolved through `toolRoot` before comparison, so running in `$HOME`
with no `NARU_PATH` — or through a symlinked `$HOME` — collapses to one root
instead of double-listing every skill. A name collision is won by the project
root and the loser is dropped with a warning. A missing root, a directory with no
`SKILL.md`, or a dotfile is skipped **silently**; only something that looks like a
skill and fails to parse warns. That is the `mcpAccept` policy: never fail the
load over one bad entry.

**Drop-in compatibility is the point.** Frontmatter is parsed with a plain
`yaml.Unmarshal` into a two-field struct — deliberately *not* `KnownFields(true)`
— so an ecosystem bundle carrying `allowed-tools`, `license`, or a nested
`metadata:` map loads unmodified. Note that **`allowed-tools` is parsed over and
ignored**: mininaru has no per-turn tool-filter seam, and half-honoring it would
be worse than not claiming it.

The canonical name is the frontmatter `name` when it passes `util.SafeSegment`
and the name pattern, otherwise the directory name. That check is what stops a
hostile `name: ../../etc` from becoming a lookup key.

**Bounded by construction.** `maxSkills` (64), `maxSkillDescription` (200 runes),
`maxCatalogChars` (4096), `maxSkillBody` (64 KiB). The catalog matters twice over
because `core/complete.go` does not trim history, so on the HTTP path its size is
pure additive cost.

**Sandboxing is per bundle, not per root.** `Skill.Path` holds the symlink-resolved
bundle directory and the tool's optional `path` argument is checked against *that*
with the existing `readPath`. This deliberately differs from the root-prefix model
in [modules/file.go](../modules/file.go), and the reason is that installing a
shared skill by symlinking it into `skills/` is a normal thing to do.

**No new execution path.** The tool returns the body, the bundle's absolute path,
and a listing of companion files. Running a script is still `bash_exec`, so the
approval prompt and its sandbox apply unchanged.

The tool is classified **safe**, so the Discord bot and the HTTP API get skills.

**Creation is privileged, not dangerous.** `skill_create`
([modules/skillcreate.go](../modules/skillcreate.go)) is grouped with `memory`,
not with `file_write`. The reason is what the write does after it lands: the new
bundle enters `SkillCatalog()` and therefore every later system prompt. That is
durable state feeding the prompt, which is exactly the property that made
`memory` privileged, and a skill carries more of it than a memory line does. The
practical effect is that `SafeTools()` filters it out, so the HTTP API cannot
reach it at all and no approval prompt has to stand in for that.

It writes nothing the loader would reject. Name, description, and body are put
through `skillNamePattern`, `skillDescription`, and `maxSkillBody` — the same
functions `skillParse` uses on the way back in — and the frontmatter is produced
by `yaml.Marshal`, not string concatenation, so a description containing a colon
quotes itself. The round-trip test in
[modules/skillcreate_test.go](../modules/skillcreate_test.go) is what keeps the
writer and the reader from drifting apart. `skillName`'s fall back to the
directory name is deliberately *not* reused here: at read time a usable fallback
beats dropping a bundle, but at write time the caller should be told it asked for
something impossible. Overwriting requires an explicit flag and cannot cross
scopes, so a project write can never shadow-then-replace a user skill.

**Usage is recorded in its own table.** `skill_uses`
([util/migrations/0009_skill_uses.sql](../util/migrations/0009_skill_uses.sql))
duplicates something `tool_calls` already half-holds, and that is on purpose.
`tool_calls` has a foreign key to `messages`, so a tool call made through
`core.Complete` — the HTTP API, the Discord user-app commands — has no row at
all; `skill_uses` has no such key and records those. It also stores the
**resolved** scope and bundle path, which the raw arguments cannot tell you when
a project and a user skill share a name.

The write happens in `executeTool`, the one point both front-end paths funnel
through, gated on a completed status so a load of a skill that does not exist is
not counted as use. `completionRun` carries a `SessionId` purely to give that row
its session; `core.Complete` leaves it empty rather than inventing one. A failure
to record is logged and swallowed: the skill has already been loaded and the
answer is already in flight, so failing the turn over an audit row is the wrong
trade.
That looks like it contradicts `file_read` being dangerous, but the reachable set
here is finite and enumerable — only the bundles the operator installed, whose
names and summaries the same request already carries in its prompt. The tool does
not widen what the model can see; it defers content the operator already chose to
advertise. The `path` argument exists for the same reason: without it, companion
files would be reachable only through `file_read`, which is dangerous *and* rooted
at cwd, so the daemon, `-p`, and every user-scoped bundle would be locked out of
the feature that exists to serve them.

The scan runs eagerly in `main()` — unlike `MCPInit`, it spawns nothing and opens
nothing, and `systemPrompt` is reached from four entry points, so a lazy variant
would need a `sync.Once` reachable from `core` for no benefit. `SIGHUP` re-scans.
The slice is behind a `sync.RWMutex` rather than being a bare global like
`modules.MCP`, because a reload rewrites it while request goroutines are building
prompts from it.

## Delegation

`agent_call` in [core/subagent.go](../core/subagent.go) runs one turn of another
configured agent and returns its answer as a tool result. It is the one builtin
that does not live in `modules`, and it cannot: it needs `AgentByName` and the
completion loop, and `modules` is a leaf that must not import `core`.

The way out is that `modules` owns the *table* and `core` owns the *tool*.
`modules.RegisterBuiltin` appends to the builtin list, `core.InstallAgentTool`
calls it, and `cli/main.go` calls that from `bootstrap` before anything asks for
a tool. From there `agent_call` is served by the same in-process MCP server as
the other twelve, so the "no non-MCP path to a tool" rule still holds and the
leaf invariant is intact. Registration after the builtin server has started is
refused with an error log rather than silently dropped, and the ordering is
pinned by a `TestMain` in the `core` tests that installs before any test runs.

A subagent is deliberately not a session. It gets a fresh system prompt built
for the target agent, one user message, and nothing else — no history, no
`tool_calls` rows, no session id. That keeps delegation cheap and keeps a
subagent from inheriting a conversation it was never meant to see, at the cost
of making the prompt the caller writes the whole of the subagent's context.

What it *does* inherit is the calling turn's execution policy: the same defs,
the same `AllowDangerous` and `AllowPrivileged`, and the same approval callback.
A dangerous tool reached through a subagent therefore raises the same prompt it
would have raised in the parent turn, rather than quietly running because it is
one level down.

Recursion is stopped twice over. `childDefs` strips `agent_call` from what the
subagent is offered, so it is not advertised a tool it may not use, and
`completionRun.Depth` carries a counter that refuses a second level even if a
def slips through another way. The depth lives on the run rather than only in
the context because `dispatch` rebuilds the context policy on every round, and a
counter read back from the context it just wrote would always be zero. An agent
is also refused delegation to itself, compared by id.

## Token accounting

Three of the last four features spend tokens the user did not directly ask for —
a delegated turn, a summarising call, a tool result large enough to matter — and
none of them were visible. `core/usage.go` records every model call made on a
session's behalf into `token_usage`, tagged with what spent it: `turn`,
`compaction`, or `subagent`.

**Rounds are summed, not overwritten.** `completionRun.execute` used to assign
`result.Usage` each round, so a turn that took three tool rounds reported only
the third. Each round resends the whole conversation and each is billed, so the
sum is the honest number. This also changes what the HTTP API returns: `usage` in
a response now covers every round the server ran on the caller's behalf, which is
not what OpenAI means by the field but is what the caller actually caused.

**Usage has to be asked for.** `stream_options.include_usage` was set only in
`Complete`, so the session path and the delegation path never received usage at
all. Both now set it; without that the table stays empty on exactly the paths
this exists for.

**It is a table, not columns on `messages`.** A compaction call happens before
the turn's message row exists — `compactHistory` runs ahead of `messageStart` —
and a turn can delegate more than once, so one set of columns cannot hold it.
`message_id` is a plain column with an empty default for the rows that have no
message, the same shape and for the same reason as `skill_uses`.

**Recording never fails a turn.** `usageRecord` logs and swallows, like the
`skill_uses` write: the answer is already in flight and losing an accounting row
is the cheaper failure. It also writes nothing when the session id is empty (the
stateless API) or the provider reported no tokens.

No prices are stored or computed. mininaru talks to arbitrary OpenAI-compatible
providers under arbitrary model names, so any table of rates would be a guess
that goes stale; the tokens are reported and the conversion is left to whoever
knows their own contract.

The Discord `/usage` command reads the same totals and is admin-only and
channel-scoped, resolving its session with `SessionByExternal` exactly as
`/compact` does. Unlike `/compact` it answers straight away rather than
acknowledging and editing: it is a database read, so there is no model call to
outrun the three-second interaction deadline.

## Memory

Durable memory is a small, global SQLite store shared by trusted interactive
front ends. The `memory` tool supports list, add, replace, and remove operations,
deduplicates exact entries, and rejects writes beyond a 4096-character total.
The current snapshot is added to the system prompt only when that request also
has the memory tool, so possession of the tool and visibility of the data cannot
drift apart.

Memory has `PermissionPrivileged`: CLI requests use `DefaultTools()` and receive
it, while the stateless HTTP API and ordinary paired Discord users use
`SafeTools()` and receive neither the tool nor the prompt block. A Discord admin
created through the one-time owner pairing path uses `DefaultTools()`, so the
owner and CLI see and update the same store regardless of session or agent.

## Web tools

`web_search` and `web_fetch` share HTML helpers
([modules/webhtml.go](../modules/webhtml.go)) and nothing else — different
clients, different trust models.

**Search providers** are a table of `{Name, Request, Parse}` in
[modules/web.go](../modules/web.go), the same shape as `builtinTools()`. Keeping
a provider's request builder next to its parser is the point: a bare `switch`
would need two of them, and they drift. `searchRun` owns everything shared — the
2 MiB cap, the 2xx check, the zero-results error, the limit trim — so a provider
is only the two functions that actually differ. Selection lives in `web.json`
(`0600`, it may hold an API key). A broken config warns on stderr and falls back
to DuckDuckGo rather than leaving the model with no search, the same policy
`mcpAccept` and `ClientInit` already use; the CLI validates strictly instead, so
mistakes are caught where a human is watching.

**The `web_fetch` guard hooks `net.Dialer.Control`, not the URL.** `Control`
receives the literal `ip:port` after resolution and before `connect(2)`, once per
Happy Eyeballs candidate. That placement is the whole design:

- DNS rebinding fails, because the address checked is the address dialed.
- Redirects are re-validated for free — every hop is a fresh dial.
- Connection reuse is safe, because the pool is keyed on host:port.

A URL-level allow/deny list can do none of those. The pre-dial checks in
`fetchTarget` (scheme, userinfo, port, literal IPs) exist only to give the model a
clean error instead of a dial failure; they are not the control.

`fetchAddrAllowed` unmaps IPv4-in-IPv6 **before** testing anything —
`::ffff:127.0.0.1` returns false from every `netip` predicate in its mapped form.
It then adds prefixes the predicates miss, most importantly `100.64.0.0/10`:
`netip.Addr.IsPrivate` does not cover CGNAT, which is where Tailscale and its
`100.100.100.100` metadata endpoint live. 6to4, Teredo, and NAT64 prefixes are
blocked whole rather than un-embedding and re-checking the inner IPv4 — three
lines instead of thirty, and it fails closed. There is no special case for
`169.254.169.254`; it is link-local and already blocked, and a host list would
only create false confidence about what the control is.

`Proxy` is set to `nil` on the fetch transport. With `HTTP_PROXY` set, the dial
goes to the proxy and the guard never sees the real destination — a complete
bypass that is invisible in review.

**The search client is deliberately unguarded**, which is what lets a self-hosted
SearXNG on `127.0.0.1` work. The asymmetry is about provenance, not convenience:
the search endpoint comes from an operator-written `0600` file — the same trust
boundary `mcp.json` sits on, where the operator can already declare a stdio server
running arbitrary commands — while a fetch URL is chosen by the model, possibly
from a page it just read. The model cannot influence the endpoint either
(`web_search`'s schema is `query` + `limit`, `additionalProperties: false`), and a
search response is parsed into `[]SearchResult` rather than returned verbatim, so
pointing it at an internal service yields a parse error, not that service's body.

Search results are URLs the model may hand to `web_fetch`, so a poisoned result
*is* a path back in — and it is already closed, at dial time, wherever the URL
came from. Result-URL filtering is deliberately absent: it would be redundant
against the dial check and would suggest the URL list is the control.

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

- `provider.json`, `agent.json`, `bot.json`, `client.json`, `mcp.json` — mode `0600`, written through
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

## Compaction

`trimHistory` on its own loses whatever those turns held, silently. `compactHistory`
in [core/compact.go](../core/compact.go) wraps it: it still decides what leaves,
but summarises the departing turns into the conversation first and carries the
result in the system prompt as a `<mininaru-summary>` block. `trimHistory` itself
is untouched and remains the only place that decides what fits.

The summary is one row per session in `session_summaries`, rewritten in place
rather than appended to, with `through_message_id` marking the newest message it
covers. **It is deliberately not a row in `messages`.** A summary living in the
history would be charged to the budget by the very function it exists to soften,
and could be trimmed away — the failure mode would be losing the compression of
the thing you were trying not to lose. Keeping it out also leaves `MessageList`
meaning "what was actually said", which is what the TUI replays.

The marker is a message id rather than a rowid because `MessageList` does not
select rowids and `Message` should not grow a storage detail to carry one. If the
marker is not found in the history, the tail is treated as uncovered and every
turn is replayed: a duplicated turn is a cheaper failure than a silently dropped
one, and it is logged.

Budgeting is charged the summary's **current** length, not a reserve for the one
about to be written. Charging a fixed reserve up front would make every
conversation hit the budget earlier than it does today, including ones that never
compact. The turn where compaction happens is therefore slightly off — the new
summary may differ in length from the old — and the next turn corrects it. A cap
of `maxSummaryChars` keeps that drift bounded, enforced both in the instruction
and by truncating what comes back.

Summarising uses `Complete` with no tool definitions, so the summary turn cannot
reach a tool: `Complete` leaves `AllowPrivileged` false and passes no defs. It
runs on the conversation's own agent and model.

**Failure never breaks the user's turn.** A summary call that errors, a context
that cancels, an empty answer, a failed write — each is logged at warn and falls
through to today's behaviour of dropping the turns. Compaction is an improvement
on a lossy path, not a new way for a chat to fail.

`context.compact` turns new summarisation off. An existing summary keeps being
applied, because it is already paid for and dropping it would lose more than it
saves.

`CompactNow` is the same machinery behind an explicit request — `/compact` in the
TUI, `/compact` in Discord — and differs from the automatic path in three ways
that all follow from the user having asked for it. It does not consult the
budget: everything not already covered by the summary is folded in, leaving
nothing to replay. It ignores `context.compact`, because that toggle governs
compaction the user did not ask for. And it returns its error instead of logging
and falling back, because a request that failed should say so rather than
silently do nothing.

The Discord command is admin-only and resolves its session with
`SessionByExternal(OriginDiscord, channelID)`, so it can only ever act on the
conversation bound to the channel it was run in. A channel with no conversation
is refused rather than having one created. Summarising is a model call and would
blow the three-second interaction deadline, so the handler acknowledges first and
edits the reply from a goroutine, the same shape `chatCommand` uses.

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

For external MCP tools, safe versus dangerous is derived from the tool's
`ToolAnnotations.readOnlyHint` and can be overridden per server (`permission`)
or per tool (`tool_permission`) in `mcp.json`; a tool with no annotations is
dangerous. Twelve of the builtins are classified by a fixed table in
[modules/builtin.go](../modules/builtin.go), and `agent_call` carries its own
classification in from `core` when it registers; neither is overridable —
`file_read` is honestly read-only and annotated as such, but classifying it safe
would put arbitrary filesystem reads on the HTTP API with no human in the loop.

`grep` and `glob` are dangerous for the same reason, and `grep` is the sharper
case: it is annotated `readOnlyHint`, but the pattern `.` turns it into a whole
file dump, so classifying it safe would reopen through the search tool exactly
what keeping `file_read` dangerous closes. `glob` leaks names rather than
contents and is held to the same line so the filesystem has one classification
and not two.

`readOnlyHint` is the remote server's own claim about itself, so adding an entry
to `mcp.json` is a trust decision. `"daemon": false` keeps a server's tools in
the TUI while hiding them from the API server and the bots.

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

`cli/serve.go` also runs `modules.MCPReload` on the same signal, before
`Registry.Reload`. It keeps sessions whose `mcp.json` entry is byte-identical
and re-dials the rest, so unchanged servers are not restarted. The in-flight
guarantee is weaker here than for agents: a request holding an old `*Instance`
may hold `Def.Execute` closures over a session the reload just closed, and such
a call fails back to the model as a tool error.

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

## Self update

`cli/update.go` swaps the running executable for a release build and, if the
systemd unit exists, restarts it. The unit's `ExecStart` is the symlink-resolved
absolute path written at install time, so replacing the file in place and
restarting is all it takes — nothing rewrites the unit.

**Verify, then swap; never the other way round.** The archive streams through an
`io.TeeReader` into a `sha256.Hash` while it is being un-tarred, so the checksum
from the release's `SHA256SUMS` is confirmed on the bytes that were actually
read, not on a second download. The extracted file is staged **in the same
directory as its target** and only then `os.Rename`d over it. Same directory
because rename cannot cross filesystems; rename because it is atomic and works
on a running binary, which a plain write cannot (`ETXTBSY`). Every failure path
deletes the staged file and leaves the current executable untouched, so there is
no half-written state to recover from and no `.old` backup to clean up.

The extractor takes exactly one entry — a regular file whose `filepath.Base` is
`mininaru` — and ignores everything else in the archive. Path traversal stops
being a question you have to answer when you never join an archive-supplied path
onto a directory.

`modules/webguard.go`'s SSRF guard is deliberately **not** used here. That guard
exists because the model chooses those URLs; this host is a constant in the
source.

**The check is one release behind on purpose.** `updateCheckStart` fires a
goroutine after `bootstrap()` and nothing waits for it. It writes the latest tag
to `update.json`, and the notice is rendered from that cache on a *later* run.
Blocking startup on a network call to display a courtesy message is a bad trade,
and the freshness that buys is worth nothing when the TTL is a day anyway. The
check is skipped entirely for `update`, `serve`, and `daemon` — the first does
its own lookup and the other two are long-running processes where a
start-time check answers a question nobody asked.

`--tag` never writes the cache. The cache means "the newest release that
exists", so recording a deliberately pinned older tag there would silence the
very notice it is meant to raise.

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

**The answer is a plain message; the card is a log.** v0.5.0 tried the opposite
— it edited the progress card into the answer to save a message — and the
result was worse in every way that mattered. A Components V2 container renders
as a bordered, tinted box, so every reply stopped looking like chat; and since
only the first chunk could be an edit, a long answer came out half in a box and
half not. `executionStatus` in
[bot/discord/handlers/ui.go](../bot/discord/handlers/ui.go) now owns only the
card: a heading holding the current state, a line appended per finished tool,
and a footer with the elapsed time. `render` rebuilds the whole card from that
state and `publish` skips the edit when the result is byte-identical, which is
what keeps a tool-heavy turn from spending its rate limit on redundant edits.

The thread welcome and the thread-creation failure notice used to be their own
messages, which put three cards in a channel for one question. They are now a
single `note` line inside the card, chosen by `conversationTarget.note`.

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
