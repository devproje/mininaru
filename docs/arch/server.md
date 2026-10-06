# `server/` — three route groups, one gin engine

`server.NewAppServer(host string, port uint16, apiKey string)` builds one
`*gin.Engine` and wires:

- **`/api`** (`server/api.go`) — REST admin CRUD for agents, providers,
  sessions, and messages, one controller file per resource
  (`server/controller/*.go`), each doing bind → validate → `core` call → JSON.
  `ProviderList`/`ProviderRead`/etc. never return the raw API key; every
  response goes through `toProviderResponse`, which masks it to
  `sk-t...efgh` before it leaves the process. `POST /api/mode` is the one
  extra route here that isn't resource CRUD — it upserts a per-directory mode
  entry (see [Tool calling](tools.md)).
- **`/api/v1`** (`server/openai.go`) — the OpenAI-compatible surface:
  `POST /chat/completions` (streaming SSE or a single JSON body) and
  `GET /models`. The `model` field of a chat request names a **mininaru
  agent** by its `Name`, resolved with `core.AgentByName` — not an upstream
  model string — so `GET /models` lists configured agents.
- **`/ws`** (`server/sock/sock.go`) — one generic websocket that multiplexes
  every session over a single connection type. Inbound frames are dispatched
  by their `type`: a chat frame carries no type at all and is
  `{session_id, content, cwd}` (`cwd` feeds the mode anchor), `{type:
  "approval", session_id, decision}` answers a pending prompt, `{type:
  "interrupt", session_id}` cancels that session's in-flight round, and
  `{type: "attach", session_id}` registers the connection as that session's
  live viewer without running a round. Because an absent field survives a
  `json.Unmarshal` into a reused struct, `SockHandler` zeroes its
  `inboundFrame` on every iteration — otherwise an `attach` would leave its
  type behind and swallow the next chat frame. Outbound frames are
  `{type: "message"|"chunk"|"tool"|"approval_request"|"done"|"error", ...}`
  — `message` echoes a `session_send` injection (`name` is the *origin*
  session id), `chunk` carries a completion delta plus a `reasoning` string,
  `tool` reports a call's `name`/`status`/`message`, `approval_request`
  carries `session_id`/`cwd`/`name`/`arguments` and blocks the turn until an approval frame
  answers it (see [Tool calling](tools.md)). Reasoning deltas are pulled out of
  the chunk's raw JSON (`chunkReasoning`), because
  `openai.ChatCompletionChunk` has no typed field for
  `reasoning`/`reasoning_content` and different providers use either key.

All three require `Authorization: Bearer <key>` (`server/auth.go`). The key
is a random 32-byte value generated on first use and stored at
`NARU_PATH/mininaru.key`, mode `0600` (`util.APIKey`, `util/api_key.go`) —
there is no setup step and no separate command to reveal it again later;
reading the file is the only way. `cli/serve.go` calls `util.APIKey()` at
startup and passes it into `NewAppServer`; `modules/client` resolves the key
it sends as `--api-key` flag > `MININARU_API_KEY` env var > (only when the
target `--url` host is loopback) reading that same local file
(`ResolveApiKey`, `client.go`) — a client pointed at a remote `--url` never
reads the local key file, so a locally-generated key cannot leak to whatever
host `--url` happens to name.
