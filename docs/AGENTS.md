# Working on mininaru with a coding agent

This is for AI coding agents — Claude Code, Codex, Cursor, or mininaru itself
— asked to change this repository, and for the person supervising one. It
covers what to read first, how to verify a change, and which actions need a
human.

The rules that decide whether a change is acceptable live elsewhere:
[CONVENTION.md](CONVENTION.md) is the code style, and
[ARCHITECTURE.md](ARCHITECTURE.md) is the package layout. Read both before
the first edit. This document does not repeat them.

## This is a rewrite in progress

`refactor/1.0.0-alpha` dropped a lot of what an earlier version of this
project had: skills, memory, subagent delegation, a Discord front end, a
paired gRPC client, a full-screen TUI. (Tool calling, MCP, and delegation —
now the `agent_spawn` and `session_send` tools — came back; see "Tool
calling" in ARCHITECTURE.md. All three are a different, lighter design than
the old one, so don't assume the old shape.) If you find a stale reference
to any of the rest — in a comment, a doc, an old branch — it describes
something that used to exist, not something you are missing. Check
[ARCHITECTURE.md](ARCHITECTURE.md) against the actual package layout before
trusting any other description of "what mininaru does," this file included:
documentation lags the code here more often than the reverse.

## Read before editing

| You are touching | Read first |
| --- | --- |
| anything | [CONVENTION.md](CONVENTION.md) |
| `core/` or `server/` | the matching section of [ARCHITECTURE.md](ARCHITECTURE.md) |
| `cli/shell/` | the "The interactive shell" section of [ARCHITECTURE.md](ARCHITECTURE.md) |
| the HTTP API | "`server/` — three route groups, one gin engine" in ARCHITECTURE.md |

`core/chat.go` is the one place completion happens; both the `/api/v1`
controller and the `/ws` handler call into it. A change there affects both,
and a change that only fixes one of the two callers is usually in the wrong
place.

## The conventions that get violated most

`make fmt`/`make test` fail on unformatted files, but gofmt does not check
any of the rules below. They are the ones that come back in review:

- **No comments.** Every comment in this repository is either the two-line
  SPDX license header at the top of a file or a compiler directive. There
  are no explanatory comments anywhere; do not add the first one. A new
  `.go` file starts with the SPDX header — copy it from a neighbouring file.
- **No `:=`.** One `var` block at the top of the function, in first-use
  order, `err` last, assigned with `=` where the value is needed. This
  includes the init statements of `for`, `if`, and `switch`.
- **Dependency order.** A helper or callee appears immediately before its
  first caller. Independent helpers follow the order in which that caller
  invokes them. `main` is unconditionally the final function and final
  top-level declaration in its file; nothing may appear below it. Do not
  sort functions alphabetically or group them by role.
- **`util.Log` for diagnostics.** A constant lowercase message with
  everything variable in key/value attributes. Output meant for the user is
  not logging and keeps using `fmt`.

Match the surrounding code. If a file does something differently from what
you would write, the file wins.

## Verifying a change

```sh
make test-race   # gofmt -l + go vet + the suite under the race detector
```

That is what CI runs on every push and pull request, alongside a plain
`make build` and a cross-compile check for `linux/amd64`, `linux/arm64`, and
`darwin/arm64` — `os/exec`, `syscall`, and anything terminal-related in
`cli/shell/` deserve a `GOOS=darwin go build ./...` before you claim a change
is done, since that platform is not what most development happens on.

Most packages have tests as of this writing (`cli`, `cli/shell`, `core`,
`modules/bash`, `modules/browser`, `modules/file`, `modules/mcp`, `server`,
`server/controller`, `server/sock`, `util`) — check with `go test ./... -v`
which ones actually ran; "tests pass" after running one package is a false
statement about the rest. `modules/browser`'s integration test skips itself
when no Chrome/Chromium binary is reachable (`browser.Available()`) — a
green run on a machine without one didn't cover that code.

`core` tests reset the database per test with `setupTestDB(t)`
(`core/testing_helpers_test.go`), which points `util.DB` at a fresh
`t.TempDir()` SQLite file and closes it on cleanup — call it at the top of
any new `core` test, or it will see whatever the previous test left behind.

## Do not do these without being asked

- **Do not start `serve` against the real data directory.** Set `NARU_PATH`
  to a temporary directory for anything you run yourself. Every endpoint
  requires `Authorization: Bearer <key>` now (`server/auth.go`), but the key
  lives in `NARU_PATH/mininaru.key` right next to the data it protects —
  running against the real directory still means real provider credentials
  and a real `bash_exec`/file-tool blast radius sitting behind whatever you
  just spun up, so keep it scoped to a throwaway directory regardless.
- **Do not treat 1a-era "no gate" as the current state.** Dangerous tools
  (`bash_exec`, `file_write`, `file_edit`, `browser_*`) are gated by yolo
  mode + human-in-the-loop approval (`core/yolo.go`, `server/sock`) — see
  "Tool calling" in ARCHITECTURE.md. If you're driving the shell yourself,
  expect an approval prompt for any of them outside a directory you've
  already trusted with `/yolo`.
- **Do not read or echo secrets.** Provider API keys live in the `providers`
  table in SQLite (`.mininaru/data.db`), not a separate file, but they are
  still real credentials. `server/controller/provider.go`'s
  `toProviderResponse` masks them in every API response for this reason —
  do not add a code path that returns one in full.
- **Do not push, tag, or force anything.** Committing is fine when asked;
  publishing is a separate decision. Pushing a `v*` tag is the loudest of
  these: it triggers `.github/workflows/release.yml`, which builds six
  binaries and publishes a GitHub release that people will download.
  Deleting a release afterwards does not recall it.

The data directory is `.mininaru/` unless `NARU_PATH` says otherwise, and it
is gitignored. Nothing under it should ever be staged.

## Branches and commits

`master` is protected and takes changes through pull requests, so work goes
on a branch. **Cut a new branch for every piece of work.** A branch whose
pull request has been merged or closed is spent: do not push to it again and
do not check it out for the next task, however closely the two are related.
Branch from an updated `master` instead. [CONTRIBUTING.md](../CONTRIBUTING.md)
says why.

Conventional commits with a `type(scope):` subject, and a body in prose
rather than bullet points, explaining what was wrong before the change and
why this is the fix. Look at `git log` before writing one.

- One logical change per commit. If two changes share a file, split the file
  rather than the story.
- Every commit builds and passes tests on its own.
- Do not add `Co-Authored-By` or any other attribution trailer.
- Do not describe the change as a plan or a to-do list. Describe the code.

## When you are unsure

State the assumption and keep going, or ask — but do not narrow the task
quietly. Delivering half of what was asked without saying which half is
missing is worse than asking one question.
