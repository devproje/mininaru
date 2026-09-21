# API

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

**Images.** `POST /api/sessions/:id/attachments` takes a multipart `file`
(image only, ≤20 MiB), stores it under `.mininaru/attachments/`, and returns
`{id, mime, bytes}`; `GET /api/attachments/:id` streams it back. Reference the
ids when you create the turn — `images: ["<id>", …]` on
`POST /api/sessions/:id/messages` or on the `/ws` chat frame — and they're
inlined as data URIs for the model. The OpenAI endpoint takes the standard
form directly: `content` as `[{"type":"text","text":"…"},{"type":"image_url","image_url":{"url":"data:image/png;base64,…"}}]`.

The client does the upload for you: `mininaru -p "…" --image a.png --image b.png`,
or `/img <path>` in the REPL (queued and sent with your next message).

`/ws` is what the REPL uses for chat (a browser authenticates it with the
`bearer.<key>` subprotocol, see [Serving](USAGE.md#serving)): send
`{"session_id": "...", "content": "...", "cwd": "..."}` and receive a stream
of `{"type": "chunk"|"tool"|"approval_request"|"done"|"error", ...}` frames.
Sending `{"type": "interrupt", "session_id": "..."}` cancels that session's
turn mid-stream. An `approval_request` frame blocks the turn until you answer with
`{"type": "approval", "session_id": "...", "decision": "once"|"session"|"deny"}`
— see [Tools](USAGE.md#tools) in docs/USAGE.md. `cwd` is read from the first
frame a session receives and pinned to it; later frames may carry it, but it
is ignored. Omit it and the session runs without the shell and file tools.
