# mininaru

<img src="assets/logo.png" alt="mininaru" width="80" align="right">

An LLM harness in one Go binary: the agent runtime — a tool-calling loop with
bash, file edit, headless-browser, and MCP tools, plus persistent memory,
skills, and one-level delegation — reachable two ways.

- **`mininaru`** — a terminal REPL where every line goes straight to the
  agent; `/bash` runs one shell command on request (`/!bash` does the same
  without sharing the output with the agent), and dangerous tools are gated
  per directory.
- **An OpenAI-compatible HTTP + websocket API** backed by SQLite, with an
  admin CLI for providers, agents, and sessions.

This is a from-scratch rewrite, currently in the `1.0.0-alpha` series. An
earlier version had a Discord front end and a paired gRPC client; both are
gone. See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for how the pieces
fit together.

## Install

> **Alpha.** Releases start at `v1.0.0-alpha.1` and share nothing with the
> pre-refactor `0.x` line — the database schema, API, and on-disk layout are
> different, and a `0.x` binary will not run against this project's data.
> `install.sh` / `install.ps1` refuse to auto-install a release that resolves
> to a `0.x` tag (pass `--tag` to override, at your own risk).

**Linux / macOS**

```sh
curl -fsSL https://raw.githubusercontent.com/devproje/mininaru/master/scripts/install.sh | sh
```

**Windows** (PowerShell)

```powershell
irm https://raw.githubusercontent.com/devproje/mininaru/master/scripts/install.ps1 | iex
```

Downloads the latest release for your platform, checks it against the
release's `SHA256SUMS`, and installs `mininaru` — into `~/.local/bin` on
Linux/macOS (set `BINDIR` or `PREFIX` to change), or
`%LOCALAPPDATA%\mininaru\bin` on Windows (set `MININARU_BINDIR`); the Windows
script also adds that directory to your user `PATH`. Pass `--tag` (`-Tag` on
Windows) to pin a version; if `mininaru` is already on `PATH` the script
hands off to
`mininaru update` instead. To pass flags through the pipe on Windows, run
`& ([scriptblock]::Create((irm <url>))) -Tag v1.0.0-alpha.4` instead.
Building from a checkout is covered below.

`--path <dir>` (`-Path` on Windows) sets the data directory (default
`~/.mininaru`); `--uninstall` (`-Uninstall`) removes the binary, and on
Windows also clears the `NARU_PATH` user environment variable.

Run interactively (not piped) it also offers to run `mininaru daemon
install` — the background service described next — unless one is already
registered. That is also what pins `export NARU_PATH` in your shell rc (a
`User` environment variable on Windows), so `mininaru` uses one data
directory no matter which working directory you start it from — see
[Storage](docs/USAGE.md#storage).

To keep a server up, `mininaru daemon install` registers a per-user service
that runs `mininaru serve` (with `NARU_PATH` pinned to the current data
directory) and starts it — `--host` and `--port` to change the bind address.
`mininaru daemon restart` restarts it after config changes;
`mininaru daemon uninstall` removes it. The backend is:

- **Linux** — a `systemd --user` unit; run `loginctl enable-linger` to keep
  it running after logout.
- **macOS** — a `launchd` agent in `~/Library/LaunchAgents`.
- **Windows** — a per-user Scheduled Task named `mininaru` that starts at
  logon (runs `mininaru serve` in the background, `NARU_PATH` taken from your
  user environment). `mininaru daemon restart` re-runs it;
  `mininaru daemon uninstall` deletes the task.

`mininaru daemon` needs systemd on Linux; it prints a clear error on a
platform where none of the three backends is available.

## Build from source

Requires the Go version declared in `go.mod`.

```sh
make build
./out/mininaru --version
```

`make dist GOOS=linux GOARCH=arm64` cross-compiles a single release layout
into `dist/` — this is what the release workflow runs for each target on a
pushed `v*` tag.

From a local checkout, `make install` installs `out/mininaru` into
`~/.local/bin` — `make install PREFIX=/usr/local` or `BINDIR=...` for another
location, `make uninstall` to remove it.

### Verifying a release download

Every tagged release publishes `SHA256SUMS` and a signed build provenance
attestation alongside the archives:

```sh
sha256sum -c SHA256SUMS --ignore-missing
gh attestation verify mininaru_v1.0.0-alpha.1_linux_amd64.tar.gz --repo devproje/mininaru
```

The attestation proves the archive came out of this repository's release
workflow, which a checksum alone cannot tell you.

## Updating

```sh
mininaru update                 # install the latest release
mininaru update --check         # print the installed and latest versions only
mininaru update --tag v1.0.0-alpha.1   # install a specific release
mininaru update --force         # reinstall the version already running
```

The archive is checked against the release's `SHA256SUMS` **before** anything
is replaced. On Linux and macOS the new file is moved into place with a
rename, which works even while the old one is still running. Windows refuses
to overwrite a running `.exe`, so there the current executable is renamed to
`<name>.exe.old` first and the staged build takes its place; the `.old` file
is removed on a best-effort basis (it may still be locked until the process
exits — a later `update` run cleans it up automatically).

Versioning restarts at `1.0.0-alpha.1` for this rewrite and does not
continue the pre-refactor `0.x` line, so `update` looks at the full release
list (including prereleases) rather than GitHub's "latest" endpoint, which
excludes prereleases and would otherwise resolve to the old, incompatible
architecture once a stable release exists again.

Once a day, at most, mininaru checks GitHub for the latest release tag in
the background and caches the answer in `update.json` under `NARU_PATH`. The
check never blocks a command: the result is written for the *next* run,
which is when the notice appears at the top of the REPL and under
`--version`.

```
a newer version is available: v1.0.0-alpha.2 (run `mininaru update`)
```

A `dev` build never shows it. Set `MININARU_NO_UPDATE_CHECK=1` to turn the
check off entirely.

## Usage

Once you have a provider and an agent configured, `mininaru` (REPL) or
`mininaru -p "<prompt>"` (one-shot) talk to it. Full detail on storage
layout, provider/agent setup, `serve`, the tool set and `/yolo` gating, MCP
servers, the REPL, `-p`, and gateways lives in
[docs/USAGE.md](docs/USAGE.md); the HTTP/websocket API is in
[docs/API.md](docs/API.md).

## Development

```sh
make build      # -> out/mininaru
make fmt        # gofmt -l, fails on unformatted files
make vet        # go vet ./...
make test       # fmt + vet + go test ./... -v
make test-race  # the same suite under the race detector
make test-cover # race + coverage, writes out/coverage.out
```

`make test-race` is what CI runs on every push and pull request. See
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the package layout and how
the pieces fit together, and [docs/CONVENTION.md](docs/CONVENTION.md) for the
code style `make fmt`/`make test` enforce.
[docs/AGENTS.md](docs/AGENTS.md) is what to hand an AI coding agent before it
touches this repository.

## License

mininaru is free software under the **GNU General Public License, version 3
or later** (`GPL-3.0-or-later`). See [LICENSE](LICENSE) for the full text.

**The artwork is not covered by the GPL.** `assets/logo.png` and the naru
character are all rights reserved; see [COPYRIGHT.md](COPYRIGHT.md) for what
that allows. The software is unaffected by this — nothing in the program
depends on the artwork.

[CONTRIBUTING.md](CONTRIBUTING.md) covers how to send a change, and
[SECURITY.md](SECURITY.md) is where to report a vulnerability privately —
please do not open a public issue for one.
