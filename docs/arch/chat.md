# `core/chat.go` — completion, streaming only for `/api/v1`

`chatClient` builds a fresh `openai.Client` from the provider `resolveProviderModel`
looks up on every call rather than caching one on the agent, so editing an
agent's `model` (`<provider>:<model>`) takes effect on the very next message.
`chatParams`/`chatParamsUnion` map an agent's stored `ThinkingLevel` to the
SDK's `ReasoningEffort`
(`low`/`medium` map directly, `high` and `max` both become `high`, and `off`
— or anything unrecognized — sets nothing, which is the SDK's own default).
`ChatCompletion`/`ChatCompletionStream` (the stateless functions the
`/api/v1/chat/completions` controller calls) never pass tools — that surface
stays completion-only, mirroring how it takes the caller's entire message
history with no server-side session.

Images ride the OpenAI content-parts shape. `imageUserMessage`
(`core/tool_loop.go`) turns a text string plus a list of image URLs / data
URIs into a `openai.UserMessage([]…ContentPartUnionParam{…})` — the same
form the browser-screenshot tool path already used. The stateless path fills
`ChatMessage.Images` from `parseOpenAIContent` (a `content` field that is
either a JSON string or a `[{type,text},{type,image_url}]` array); the
session-backed path's `historyUnion` calls `messageImages(msg.Id)`, which
reads each bound `attachments` row off disk and base64-inlines it as a
`data:` URI (a provider can't reach the mininaru server, so URLs are never
passed through). `POST /api/sessions/:id/attachments` and the `/ws` frame's
`images` array both carry attachment ids, bound to the message row by
`core.AttachmentBindMessage`.

`max_context` is an agent's input-context budget in tokens (24,000 by
default). mininaru reserves 20% for the model's output, then estimates the
serialized prompt conservatively before every provider round — this
pre-flight tiktoken estimate (`contextTokenEstimate`, `core/context.go`) is
separate from the `used` figure `GET /api/sessions/:id/usage` reports (see
[Mode](approval.md)), which is the provider's own real usage once a round has completed.
Session-backed turns that exceed the remaining input budget summarize old
completed turns into `session_summaries`, keeping the current turn and the
newest completed turn as raw OpenAI messages. Tool calls and their results
are folded together; the raw history is never deleted. If the current raw
turn, system context, or tool schema still exceeds the budget, the turn
fails before a provider call.
The stateless `/api/v1/chat/completions` surface never rewrites caller-supplied
history: it returns a `context_length_exceeded` error instead.

`chatStreamRound` (the session-backed path's per-round streaming call) guards
against a provider that stops sending data mid-stream without closing the
connection — a `time.AfterFunc` idle timer (`streamIdleTimeout`, 2 minutes)
resets on every chunk and cancels a context derived from the caller's if it
ever fires, turning what would otherwise be an indefinite "thinking…" hang
into a bounded failure. It only trips on true silence: an actively streaming
response (even one made of nothing but reasoning filler, or a long tool
turn) keeps resetting the timer and is never cut off.

Every request also carries `stream_options.include_usage` so the provider
returns real `usage` figures (see the usage endpoint in [Mode](approval.md)), and, unless
disabled, an Anthropic-style `cache_control` breakpoint (`cacheRequestOptions`,
`core/chat.go`) on the last message of the conversation's fixed leading
system-message run (summary, then memory, skills, soul — whichever comes
last) so a provider that supports prompt caching can reuse that unchanging
prefix across rounds and turns. Both are injected via
`option.WithJSONSet` since the OpenAI Go SDK's typed params have no field
for either. `--no-cache` (root CLI flag) stores nothing — it puts a marker on
the request's `context.Context` (`core.WithNoCache`) that `server/sock`
sets from the `Frame`/`inboundFrame`'s `no_cache` bit for that one turn.
