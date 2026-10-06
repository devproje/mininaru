# Storage and core CRUD

## Storage

Everything lives under `.mininaru/`, or `NARU_PATH` if set. `util.InitFS`
creates that directory at mode `0700` and tightens an existing one to `0700`
on every start, since it holds API keys.

SQLite (`modernc.org/sqlite`, no cgo) is opened with WAL, `foreign_keys`, and
a five-second busy timeout (`util.databaseDSN`). Migrations are `.sql` files
embedded into the binary (`util/migrations/*.sql`), tracked one row per
applied version in a `migrations` table, and applied inside one transaction
per file on every `NewDatabase` call — `0001_initial_schema.sql`,
`0002_tool_calls.sql`, `0003_skill_uses.sql`, `0004_attachments.sql`,
`0005_session_cwd.sql`, `0006_session_summaries.sql`,
`0007_agent_selected_provider_active_removal.sql`,
`0008_session_last_prompt_tokens.sql`,
`0009_session_last_cached_tokens.sql`.

```
providers(id, name, api_key, base_url)
  -- every registered provider is always usable; there is no "active" one
agents(id, name, model, soul, thinking_level, max_context, selected)
  -- thinking_level is CHECKed against off/low/medium/high/max
  -- model is "<provider-name>:<model>"; a partial unique index on
  -- (selected) WHERE selected = 1 is what makes AgentSelect's
  -- "deselect all, select one" transaction safe. exactly one agent is
  -- selected once at least one agent exists (the first ever created
  -- auto-selects)
sessions(id, agent_id REFERENCES agents ON DELETE CASCADE, name, created_at,
         cwd, last_prompt_tokens, last_cached_tokens)
  -- last_prompt_tokens/last_cached_tokens cache the provider's own usage
  -- figures from the session's last completed round for the /usage
  -- endpoint; both reset to 0 on compact
messages(id, session_id REFERENCES sessions ON DELETE CASCADE, role, content,
         status, error, created_at)
  -- status is CHECKed against pending/completed/failed/cancelled
tool_calls(id, message_id REFERENCES messages ON DELETE CASCADE, call_id,
           name, arguments, result, status, error, created_at)
  -- status is CHECKed against pending/completed/failed; hangs off the user
  -- message whose turn produced the call, so a session resume can replay it
skill_uses(id, skill, scope, path, rel, session_id, call_id, created_at)
  -- one row per `skill` tool call (core/skill_use.go); session_id is indexed
  -- but not a foreign key, so it outlives the session it was logged in
attachments(id, session_id REFERENCES sessions ON DELETE CASCADE,
            message_id REFERENCES messages ON DELETE SET NULL, mime, bytes,
            path, created_at)
  -- uploaded chat images; the bytes live at .mininaru/attachments/<id>, only
  -- the metadata is in SQLite. message_id is NULL between upload and the turn
  -- that references it. deleting a session drops the rows but not the files
   -- (ponytail: orphaned files, add a sweep if it matters)
session_summaries(session_id REFERENCES sessions ON DELETE CASCADE, content,
                  through_message_id REFERENCES messages ON DELETE CASCADE,
                  updated_at)
  -- one rolling summary per session; raw messages remain in the database and
  -- through_message_id marks the last message replaced in model requests
```

Deleting an agent or a session cascades through the foreign keys; nothing in
Go application code has to clean up sessions or messages by hand.

Per-directory mode state is the one exception to "everything is SQLite": it
lives in `NARU_PATH/directory.json` — a plain JSON array of
`{root, mode, updated_at}` entries, rewritten whole via
`util.WriteFileAtomic` on every change (`core/mode.go`), the same pattern
`modules/mcp/config.go` uses for `mcp.json`. It is a flat list, not
relational data that needs joins or cascades, so a JSON file is simpler than
a table. `modules/mcp/oauth.go` keeps the same pattern for `mcp_oauth.json`:
one entry per server of the client id/secret and token an http server's
OAuth login (`mininaru mcp login <name>`) produced, so a daemon reconnect or
process restart reuses or refreshes it instead of reprompting. Both files'
secret-bearing fields are encrypted at rest the same way `mcp.json`'s
`Headers` are (`util.Encrypt`/`util.Decrypt`).

## `core/` — plain CRUD, no ORM

`agent.go`, `provider.go`, `session.go`, `message.go` each hand-build their
SQL with `fmt.Sprintf`, appending an `opts`/`values` pair per non-empty field
so an `Update` call only touches the columns the caller actually set — a
zero-value field on the struct passed to `AgentUpdate`/`ProviderUpdate`/etc.
means "leave this column alone," not "clear it." `AgentSelect` is the
one write that needs a transaction: it deselects every agent and selects
the requested one in the same `tx`, which is what the schema's partial
unique index is there to enforce.

`SessionList(agentId)` requires an agent id — that is what the HTTP API's
`GET /api/sessions?agent_id=` needs, since the query param is mandatory
there. `SessionListAll()` is unscoped and exists only for the CLI's
`session list` (which has no such requirement); the HTTP layer has no route
for it.
