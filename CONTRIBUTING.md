# Contributing

Thanks for looking. This is a small project with one maintainer, so the fastest
path to a merged change is a small one that follows the house style.

## Licensing of contributions

mininaru is `GPL-3.0-or-later`. By opening a pull request you agree that your
contribution is licensed under the same terms — inbound matches outbound. There
is no CLA to sign.

Every `.go` file carries a two-line SPDX header. Keep it on files you touch and
add it to files you create. Do not change the copyright line on files you did
not write; if you want your own notice on a substantial new file, add a line
rather than replacing one.

Do not add artwork. `assets/` is all rights reserved and is not covered by the
GPL — see [COPYRIGHT.md](COPYRIGHT.md).

## Before you write code

Read [docs/CONVENTION.md](docs/CONVENTION.md). It is short, it is enforced in
review, and it is unusual on purpose: no comments, no `:=`, one `var` block per
function in first-use order with `err` last, functions in the order they
execute. Code that ignores it will be sent back regardless of how good it is.

[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) explains the package layout and,
more usefully, that the chat client and the HTTP server run the same
tool-calling loop. A change that fixes one of them and not the other is usually
in the wrong place.

If you are pointing a coding agent at this repository, hand it
[docs/AGENTS.md](docs/AGENTS.md) as well.

## Before you open a pull request

```sh
make test-race
```

That is gofmt, `go vet`, and the suite under the race detector, and it is what
CI runs. A change touching `os/exec`, `syscall`, or anything systemd-related
should also survive `GOOS=darwin go build ./...`, because CI cross-compiles.

New behaviour needs a test. There is no mock provider type — tests stand up an
`httptest.Server` that speaks SSE and point a provider's `BaseURL` at it. The
existing tests in `core/` and `server/` show the pattern.

## Branches

`master` takes changes through pull requests. Branch from an up to date
`master`, and give the branch a name that says what the change is.

**A branch is used once.** When its pull request is merged or closed, that
branch is finished — do not push more commits to it and do not check it out
again for the next piece of work, even when the next piece is related. Start a
new branch from `master` instead.

Reusing a branch reopens a merged pull request's history, mixes review threads
that were already resolved into a change nobody has read, and leaves the branch
carrying commits that master reached by another route. A branch is free; the
confusion is not.

## Commits and pull requests

Conventional commits: `type(scope): subject`, with a body in prose that says
what was wrong before the change and why this is the fix. Read `git log` for
the tone. One logical change per commit, and every commit builds and passes
tests on its own.

Keep the pull request focused. A refactor bundled with a fix takes far longer
to review than the two apart.

**Backtick `@everyone` and `@here` in commit messages and tag annotations.**
Both are real GitHub accounts. Written bare they become mentions of two people
who have never heard of this project, and unlike release notes, a commit
message cannot be edited after it is merged. The same goes for any `@name` you
did not mean as a mention.

## Reporting things

Bugs and feature ideas go in issues. Security problems do not — see
[SECURITY.md](SECURITY.md) for the private channel.
