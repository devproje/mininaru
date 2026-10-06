# Delegation — `core/agent_spawn.go`, `core/session_connect.go`

`agent_spawn` lives in `core` rather than `modules/*` — it needs
`AgentByName`/`SessionCreate`/`MessageCreate`/`SendChatMessage` directly,
which a leaf package can't import without a cycle. `core` builds it, plus
`session_send`, `session_list`, and `agent_list`, by hand rather than
pulling them from a `modules` subpackage. Calling it creates a real
`Session` (named `"spawn: <prompt preview>"`) with the caller's anchor stored
as its `Cwd`, and a `Message` for the target agent, then recurses into
`SendChatMessage` for that session. A dangerous tool call inside the delegate
prompts over the same `/ws` connection, but approval and mode lookup use the
spawned session id and its inherited working directory rather than the parent
session id. The delegate starts with no memory of the calling conversation;
the prompt has to carry everything it needs. The tool's result is the
delegate's last assistant message, read back with `MessageList` once
`SendChatMessage` returns.

Depth is capped at one level via a `depth int` threaded through
`SendChatMessage` and `buildTools` (`core/tools.go`): `buildTools` only
appends `agent_spawn` to the tool list when `depth < maxSpawnDepth`, so a
delegate's own tool list never includes it — not a permission check the
delegate could route around, the tool simply isn't there.

Because the delegate's own streamed content never reaches the caller
(`onChunk` is a no-op in the recursive call — only the final answer comes
back), `agentSpawnTool` sends a few extra `onTool` events by hand so the
delegation doesn't look like a silent multi-round pause: a synthetic
`{name: target.Name, status: "started", message: "spawned by ..., running
independently — <prompt>"}` right before the recursive call, one more
`"finished"`/`"failed"` after it returns, and the delegate's *own* tool
calls forwarded as `{name: target.Name + "/" + toolName, ...}` via a wrapped
`onTool` closure.

`modules/client`'s renderer (`render.go`) turns each `tool` frame into a
one-line status: `"started"` starts a `spinner()` labelled with the tool
name, `"finished"`/`"failed"` stop it and print a settled `●` line (green or
red), and a multi-line `message` (a file diff) is printed under a
`+n -n` header with per-line numbers (`writeDiff`). Nested delegate activity
arrives already namespaced (`worker`, `worker/bash_exec`) from the wrapped
`onTool` closure, so it reads as a flat stream rather than a managed stack.

`session_send` is `agent_spawn`'s sibling: instead of creating a fresh
session for a fresh agent, it injects a message into a session that already
exists — **any** session, including one owned by a different agent than the
caller. The only refusal is the caller's own session, which would deadlock
on its own session lock (below). It reuses `agentSpawnTool`'s
`lastAssistantMessage` helper and the same `depth < maxSpawnDepth` gate in
`buildTools`. The nested `SendChatMessage` uses the target session's persisted
`Cwd`; an empty target `Cwd` therefore exposes no filesystem or Bash tools.
Approvals still travel over the initiating caller's `/ws` connection, but
carry the target session id and working directory.

Because a cross-agent injection would otherwise look, from the receiving
session's own history, indistinguishable from that agent's own user typing
a message, `markSenderAgent` (`session_connect.go`) prefixes the content
with `[message from agent "<caller>" via session_send]` whenever
`target.AgentId != caller.Id` — a same-agent send (still the common case)
is left untouched, byte for byte. The mirrored copy a live viewer sees
(`mirrorMessage`, below) carries the same marked text the target agent
actually received, not the raw un-marked input, so what a person watching
sees matches what was delivered.

Its `session` argument accepts an id **or a name** — `resolveSessionRef`
tries `SessionRead` first and, only on `sql.ErrNoRows`, falls back to a
name match against `SessionListAll()` (every session, matching
`session_list`'s output). `session_list` shows the model each session's
`Name` alongside its id, and every session has a random `adjective-noun`
name (`core/session_name.go`), so a model shown `quiet-otter` can pass that
back verbatim. A miss on either path reports `no session %q — check
session_list`, not a raw `sql: no rows in result set`.

Because the target session may have a person watching it live over another
`/ws` connection, two extra pieces exist purely to serve that case:

- **`core.SessionLock(ctx, sessionId)` and `core.SessionTryLock(sessionId)`**
  (`core/session_lock.go`) — a `sync.Map` of per-session channel semaphores,
  `Load`-or-`Store`d by id. Normal WebSocket turns wait with their connection
  context, while `session_send` immediately returns a busy error when another
  turn owns its target. Every
  place that reads a session's history, appends a new pending message, and
  runs a `SendChatMessage` round holds this lock for the duration:
  `session_send`'s `Execute`, and `server/sock/sock.go`'s `handleFrame` (the
  normal per-frame path). Without it, `session_send` writing into a session
  that a person is concurrently typing into — or two fast frames on the same
  `/ws` connection — could interleave two `historyUnion` reads against the
  same "one pending message" invariant and corrupt the session.
- **The live-connection registry** (`server/sock/session.go`) — a
  `sessionId -> *safeConn` `sync.Map` (`liveConns`, alongside the
  `sessionAutoApprove` map in the same file), populated the moment a session
  is resolved and cleared for a connection's sessions when `SockHandler`'s
  loop exits. `core` can't import `server/sock` (cycle), so the wiring runs
  the other way: `core/session_router.go` exposes
  `SetSessionRouter(messageFn, chunkFn, toolFn, doneFn)`, and
  `server/sock/session.go`'s `init()` calls it once with closures that look a
  session up in `liveConns` and, if present, `writeFrame` the same frame
  shapes `handleFrame` sends. `session_send` calls these hooks
  unconditionally and they are no-ops when nobody's watching: `messageFn`
  right after the injected `Message` is persisted (so the viewer's transcript
  order matches the stored order), `chunkFn`/`toolFn` from the nested round's
  callbacks, and `doneFn` once the round settles — a `"done"` frame on
  success, an `"error"` frame carrying the failure otherwise. The `"message"`
  frame reuses the `Name` field for the *origin* session id and `Message` for
  the injected content.

  A session counts as live from the moment the client connects, not just
  once it sends a message: `RunTuiSession` (`modules/tui/tui_session.go`)
  writes a `{"type":"attach","session_id":...}` frame right after dialing,
  and `SockHandler` dispatches `"attach"` to `handleAttach`
  (`server/sock/sock.go`) — a synchronous, no-round path that validates the
  session exists (`core.SessionRead`) and calls the same
  `registerLiveConn`/`seen.Store` pair `handleFrame` uses.

The TUI's `pumpTuiReplies` goroutine reads the socket continuously for the
whole connection, not only during an active turn, so a message injected into
the session by `session_send` while its owner is idle is rendered on that
person's screen as soon as it arrives — it does not wait for their next
turn (see [the TUI](tui.md)).

`session_list` and `agent_list` (`core/session_tools.go`) exist so a model
can pick a valid target for the two tools above without being told one in
its prompt: `agent_list` is `AgentList()` unfiltered, and `session_list` is
`SessionListAll()` (**every** session, not just the caller's own agent's)
intersected with the same `liveConns` registry `session_send`'s mirroring
uses, via a second getter set alongside `SetSessionRouter`:
`core.SetLiveSessionsLister(fn func() []string)`, called from the same
`server/sock/session.go` `init()`. Each entry carries an `agent` field (the
owning agent's name, from a one-shot `AgentList()` id→name map built per
call) and a `current` bool (`item.Id == callerSessionId`). Every live
session here is a valid `session_send` target except `current`. Both tools
are `modules.PermissionSafe` — pure reads with no side effects, unlike
`bash_exec`/`file_*`/`browser_*`, which are `PermissionDangerous` because
they touch the filesystem or network.
