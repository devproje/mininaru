# `modules/client/` — the wire protocol and the `-p` renderer

`modules/client` is the layer both front ends share: `Frame`/`Reply` and the
HTTP/WS helpers (`Api`, `Upload`, `Dial`, `Pump`, `Agent`, `Session`,
`FindSession`, `ResolveApiKey`, `ResolveCwd`, …), plus the markdown→ANSI
formatter and the classic streamed-reply renderer that only `-p` still uses.
It imports `core` for `Agent`/`Session` types and `util`, never `server`, and
never touches SQLite; it reaches a `mininaru serve` instance only over `/api`
and `/ws`.

`mininaru -p "<prompt>"` (`cli/prompt.go`) runs the wire protocol without any
UI: resolve a session (deleting it on exit unless `--session` was given),
dial, `attach`, upload any `--image <path>` (repeatable, via `client.Upload`
— multipart `POST /api/sessions/:id/attachments` before the frame goes out,
the returned ids riding the frame's `images` array), send one frame, and
`Receive` (`render.go`, fed by its own `Pump`) with a `nil` key stream so
there is no interrupt watcher.

`--format` / `-f` (`string` default, or `json`/`xml`) is checked by
`client.ValidFormat` and threaded into `Receive`. For `string` the read loop
drives `renderer.frame`, which prints a streamed transcript: content and
reasoning deltas go through `renderer.text`, switching "mode" between
`reasoning` (dimmed, under a `● thinking` heading) and `content`; on a TTY
(`r.rich`) content runs through `MdRenderer` (`markdown.go`), a
dependency-free, line-buffered markdown→ANSI pass — a line is styled and
emitted only once its newline arrives, so output streams per line, not per
token. It handles ATX headings, `-`/`*`/`+`/`1.`/`1)` list markers
(normalised to `•`), blockquotes, thematic breaks, fenced code blocks (a
gutter, no inline processing inside), inline `` `code` `` / `**bold**` /
`*em*` / `~~strike~~` / `[text](url)`, and GFM pipe tables (buffered row by
row in `MdRenderer.table` since column widths need every row, and drained by
`drainTable()` on the table's end or at `Flush()`). Piped (non-TTY) output
skips markdown and prints raw text. A `tool` frame with a multi-line message
(a file diff) is printed under a `+n -n` header with per-line numbers by
`FormatDiff`/`writeDiff`.

For `json`/`xml`, `Receive` drives `renderer.collect` instead, which writes
nothing to stdout — it accumulates the content deltas and terminal `tool`
statuses, auto-denies `approval_request` (no TTY to ask), and on
`done`/`error` marshals one `client.Result` (`format.go`) via
`marshalResult` and prints it. An `error` frame still prints the object and
returns the error, so the process exits non-zero. Non-TTY stdin to the TUI
(no `-p`) is refused with a pointer to `-p`.
