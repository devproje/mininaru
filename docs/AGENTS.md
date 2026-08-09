# Working on mininaru with a coding agent

This is for AI coding agents — Claude Code, Codex, Cursor, or mininaru itself —
asked to change this repository, and for the person supervising one. It covers
what to read first, how to verify a change, and which actions need a human.

The rules that decide whether a change is acceptable live elsewhere:
[CONVENTION.md](CONVENTION.md) is the code style, and
[ARCHITECTURE.md](ARCHITECTURE.md) is the package layout and the testing
patterns. Read both before the first edit. This document does not repeat them.

## Read before editing

| You are touching | Read first |
| --- | --- |
| anything | [CONVENTION.md](CONVENTION.md) |
| a package you have not seen | the matching section of [ARCHITECTURE.md](ARCHITECTURE.md) |
| a command, a flag, or terminal output | `cli/ui.go`, `cli/errors.go` |
| the chat loop, tools, or the system prompt | "Two chat paths, one engine" |
| the HTTP API | "Server", and the `/api/v1` rule in CONVENTION.md |

The TUI and the HTTP API run the same tool-calling loop. A change to that loop
affects both, and a change that only fixes one of them is usually in the wrong
place.

## The conventions that get violated most

`make test` fails on unformatted files, but gofmt does not check any of the
rules below. They are the ones that come back in review:

- **No comments.** Every comment in this repository is either the two-line SPDX
  licence header at the top of a file or a compiler directive. There are no
  explanatory comments anywhere; do not add the first one. A new `.go` file
  starts with the SPDX header — copy it from a neighbouring file.
- **No `:=`.** One `var` block at the top of the function, in first-use order,
  `err` last, assigned with `=` where the value is needed. This includes the
  init statements of `for`, `if`, and `switch`.
- **Procedural order.** Functions appear in the order they execute, not
  alphabetically and not grouped by role. New helpers go next to their caller,
  not at the end of the file.
- **`util.Log` for diagnostics.** A constant lowercase message with everything
  variable in key/value attributes. Output meant for the user is not logging and
  keeps using `fmt`.

Match the surrounding code. If a file does something differently from what you
would write, the file wins.

## Verifying a change

```sh
make test-race   # gofmt + go vet + the suite under the race detector
```

That is what CI runs on every push and pull request, alongside `make build`.
There is also a cross-compile job for `linux/amd64`, `linux/arm64`, and
`darwin/arm64`, so a change that only builds on one platform fails there rather
than locally — `os/exec`, `syscall`, and anything systemd-related deserve a
`GOOS=darwin go build ./...` before you claim it is done.

Running the binary is not the same as testing it. `go test ./cli/` executes the
cobra tree end to end, and `runPrompt` takes its writers as arguments so the
whole session-backed chat path can be exercised without a terminal. Prefer
those over asking the supervisor to try it by hand.

Report what you actually ran. Eight packages have tests; "tests pass" after
running one of them is a false statement about the other seven.

## Do not do these without being asked

Some of what an agent can reach here is the supervisor's live installation, not
a fixture:

- **Do not run `systemctl`, `mininaru daemon install`, or `daemon reload`.**
  A developer machine usually has `mininaru.service` already installed and
  running. `daemon reload` restarts it.
- **Do not start `serve` against the real data directory.** Set `NARU_PATH` to a
  temporary directory for anything you run yourself.
- **Do not connect a bot.** A Discord token in `bot.json` belongs to a real
  application; opening a gateway with it announces the bot as online.
- **Do not read or echo secrets.** Provider API keys live in `provider.json` and
  bot tokens in `bot.json`, both unencrypted. They are masked in list output
  precisely so they do not end up in a transcript.
- **Do not push, tag, or force anything.** Committing is fine when asked;
  publishing is a separate decision. Pushing a `v*` tag is the loudest of
  these: it triggers the release workflow, which builds six binaries and
  publishes a GitHub release that people will download. Deleting a release
  afterwards does not recall it.

The data directory is `.mininaru/` unless `NARU_PATH` says otherwise, and it is
gitignored. Nothing under it should ever be staged.

## Traps

Things that look like a bug or a simplification and are not:

- **Positional verbs are subcommand names.** `mcp add`, `tools off`, and
  `web provider` were positional arguments before they were subcommands. The
  names must stay identical or documented invocations break.
- **`cobra.EnableTraverseRunHooks` is on.** Every parent's `PersistentPreRunE`
  runs, not just the closest one. A parent hook that shadows the root one would
  skip `bootstrap`, so do not add one without checking.
- **Argument validation runs before the pre-run hooks.** A command with an
  `Args` validator rejects bad input without touching the database, which is why
  `usageArgs` can map it to exit code 2.
- **Exit codes are part of the interface.** 2 is usage, 3 is not configured yet,
  1 is everything else. A new error path picks one deliberately.
- **`core` tests share package globals.** `Providers`, `Agents`, and `Global`
  are reset per test. A test that forgets leaks into the next one.
- **Tools are exposed over MCP even when built in**, and each carries a `safe`,
  `dangerous`, or `privileged` permission. Do not widen a permission to make a
  test pass.

## Branches and commits

`master` is protected and takes changes through pull requests, so work goes on
a branch. **Cut a new branch for every piece of work.** A branch whose pull
request has been merged or closed is spent: do not push to it again and do not
check it out for the next task, however closely the two are related. Branch
from an updated `master` instead. [CONTRIBUTING.md](../CONTRIBUTING.md) says
why.

Conventional commits with a `type(scope):` subject, and a body in prose rather
than bullet points, explaining what was wrong before the change and why this is
the fix. Look at `git log` before writing one.

- One logical change per commit. If two changes share a file, split the file
  rather than the story.
- Every commit builds and passes tests on its own.
- Do not add `Co-Authored-By` or any other attribution trailer.
- Do not describe the change as a plan or a to-do list. Describe the code.

## When you are unsure

State the assumption and keep going, or ask — but do not narrow the task
quietly. Delivering half of what was asked without saying which half is missing
is worse than asking one question.
