# `cli/` — the admin surface

`cli/main.go` reads `NARU_PATH` (default `.mininaru`), calls `util.InitFS`,
opens the database itself, registers every subcommand, and calls
`root.Execute()`. `root.SilenceUsage = true` plus `os.Exit(1)` on a command
error means an ordinary failure like "session not found" prints one clean
`Error: …` line, not a Go stack trace.

`provider` and `agent` (`cli/provider.go`, `cli/agent.go`) each have
`add`/`list`/`show`/`set`/`remove`, plus `agent primary`. `session`
(`cli/session.go`) is read/cleanup only — `list`, `show <id>`, `remove
<id>`. Every subcommand that takes a positional id resolves it through a
small `resolveX(idOrName)` helper that tries reading by id first and falls
back to a name match, so `agent show naru` and `agent show <uuid>` both
work. `session list` defaults to every session (`core.SessionListAll`) and
narrows to one agent with `--agent`. Against a `--gateway`/`--url` remote
these read paths scan the `/api` list instead (the `/api/sessions` route
needs an `agent_id`, so a remote `session list` sweeps every agent). `mcp`
and `skill` are the other two
admin groups — `mcp` (see [Tool calling](tools.md)), `skill` with `list`/`show
<name>`/`uses`.

`cli/serve.go` starts the HTTP server and blocks on `ListenAndServe`.
`cli/daemon.go` is `mininaru daemon install`/`restart`/`uninstall` — it
switches on `runtime.GOOS` to write a `systemd --user` unit (Linux), a
`launchd` agent (macOS), or a per-logon Scheduled Task (Windows) that runs
`mininaru serve --host --port --cors-origin --web-dir` (`daemonExecArgs`)
with `NARU_PATH` pinned to the current data directory, and also pins
`export NARU_PATH` into the user's shell rc (`pinNaruPath`) so an
interactive `mininaru` shares that directory. `daemon install` with none of
those flags set and a tty attached runs an interactive wizard instead
(`daemonWizard`): it reads the already-installed unit/plist/scheduled task
back into a `daemonPreset` (`linuxExistingConfig`/`darwinExistingConfig`/
`windowsExistingConfig`, sharing a `parseServeArgs`/`shellTokenize` pair) so
a reinstall is pre-filled from what's actually running rather than the flag
defaults, then prints the resulting plan and asks for `y/N`
(`daemonConfirmPlan`) before writing anything. The bare `mininaru` command
(no subcommand) is the TUI: `cli/main.go`'s `execute` calls `shortPrompt`
(`cli/prompt.go`) when `-p` is set, otherwise `clientExecute` (`cli/client.go`),
which calls `tui.RunTuiSession`.

## `cli/update.go` — self-update

`mininaru update` fetches a release from `.github/workflows/release.yml`'s
output (`mininaru_<tag>_<os>_<arch>.tar.gz`/`.zip` + `SHA256SUMS` +
attestation), verifies the checksum, and replaces the running executable. It
does **not** call GitHub's `/releases/latest` — that endpoint excludes
prereleases, and every `1.0.0-alpha.x` tag is one (`release.yml` marks any
tag containing `-` as `--prerelease`). `updateLatestRelease` hits
`GET /repos/{repo}/releases` (the list endpoint) and takes the newest entry
instead, so "latest" doesn't fall through to an old, incompatible `0.x`
release. The `scripts/install.sh` / `install.ps1` installers additionally
refuse a resolved `0.x` tag unless `--tag` names it explicitly — a guard the
in-binary updater does not have.

The download is staged to a temp file next to the target executable, hashed
while downloading, and only extracted after the checksum matches
(`updateDownloadArchive` → `updateExtractTarGz`/`updateExtractZip`, chosen
by `updateAssetExt()`'s `runtime.GOOS` check — the two extractors themselves
take the binary's expected filename as a parameter rather than reading
`runtime.GOOS` internally, so tests can exercise the zip path on a Linux
runner). Replacing the file is also OS-dependent, split into two directly
testable functions rather than one branch: `updateReplaceUnix` is a plain
`os.Rename` over the running binary, which POSIX allows; `updateReplaceWindows`
renames the running `.exe` aside to `<name>.exe.old` first (Windows refuses to
overwrite an open file, but allows renaming one), moves the staged build into
place, then best-effort removes the `.old` file.

`util/update.go` (`UpdateNotice`) is read only from `cli/main.go`'s
`showVersion()`, under `--version` — neither `modules/client` nor
`modules/tui` reads it; there is no notice on a plain `mininaru` launch.
`updateCheckStart`, wired into `root.PersistentPreRunE` in `cli/main.go`,
runs a TTL-gated (`util.UpdateCacheTTL`, 24h) background check on every
command except `update` and `serve` itself, writing the cache
`showVersion()` reads, so the notice is usually a command or two behind
rather than triggering a network call on every invocation.

## `scripts/` — install helpers

Not built or imported by anything; hand-run, and `make install`/`uninstall`
shell out to `install-binary.sh`. Each has a `.sh` (POSIX, for
Linux/macOS) and a `.ps1` sibling.

- `install.sh` / `install.ps1` — resolve the newest release the same way
  `cli/update.go` does (`GET /repos/devproje/mininaru/releases`, first
  entry), download `mininaru_<tag>_<os>_<arch>.{tar.gz,zip}` + `SHA256SUMS`,
  verify, and unpack into `~/.local/bin` (`$BINDIR`/`$PREFIX` to change).
  When `mininaru` is already on `PATH` they hand off to `mininaru update`
  instead. A tag that resolves to `0.x` is refused unless `--tag` names it
  — the one guard the in-binary updater does **not** have. On an interactive
  terminal they offer to run `mininaru daemon install` (the background
  service, which is also what pins `NARU_PATH`); `.ps1` still pins
  `NARU_PATH` as a `User` env var directly since Windows has no shell rc.
- `install-binary.sh` / `.ps1` — install the local `out/` build; the
  target of `make install`. Does not touch `NARU_PATH`.

Registering the background service moved from a `register-daemon` script to
the `mininaru daemon` subcommand (`cli/daemon.go`); the shell-rc pin it
writes uses the `# >>> mininaru env >>>` sentinel block, shared with
`install.sh` so a pin from either side is idempotent.

`ci.yml`'s `scripts` job shellchecks the `.sh` files and parse-checks the
`.ps1` files.
