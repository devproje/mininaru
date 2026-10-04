# Security policy

## Reporting a vulnerability

Report privately through GitHub: **Security → Report a vulnerability** on this
repository. That opens a draft advisory only you and the maintainer can read.

Please do not open a public issue for a suspected vulnerability. If GitHub's
private reporting is unavailable to you, email the address on the maintainer's
commits instead.

A useful report says what an attacker gets, and how to reproduce it. A patch is
welcome but never required.

This is a single-maintainer project. Expect an acknowledgement within a week
rather than within a day, and no bounty.

## Supported versions

Only the latest tagged release. mininaru is pre-1.0 and fixes land on `master`
and in the next tag rather than as backports.

## In scope

These are bugs. Report them.

- Reaching any `/api/v1` endpoint without a valid bearer token, or learning
  whether a guessed key was close.
- Escaping the working directory from `file_read`, `file_write`, or
  `bash_exec` in any mode other than Full Auto — anything that makes the
  root in `modules/file/file.go` not hold when the directory isn't
  explicitly set to Full Auto.
- Running a `dangerous` tool without the approval gate outside an explicitly
  trusted directory — Default mode still auto-running a call, Plan mode
  auto-running (instead of asking) `file_read`/`browser_read`/
  `browser_screenshot`, Auto Persist auto-running anything other than
  `file_read`/`file_write`/`file_edit` or auto-running one of those three
  outside its anchor, or a `-p` run executing a dangerous tool instead of
  denying it. This includes prompt injection that defeats the gate, as
  opposed to injection that merely asks for a tool.
- A provider API key or `MININARU_API_KEY` appearing in logs, terminal
  output, an API response, or an error message.
- Any data from one API request surfacing in another. `serve` is stateless and
  keeps no conversation between requests, so leakage across them is a bug.

## Not vulnerabilities

These are documented, deliberate behaviour. A report about them will be closed
with a pointer here.

- **`bash_exec` runs shell commands and `file_write` writes files.** That is
  what the tools are. In the chat client `bash_exec`/`browser_*` are gated
  by an approval prompt unless the directory is Full Auto, and
  `file_write`/`file_edit` unless it's Auto Persist or Full Auto (cycled
  with Shift+Tab, no slash command); `-p` denies every dangerous call
  outright because nobody is there to approve. In Plan mode specifically,
  mutating calls get no prompt at all — they're auto-rejected, which is
  more restrictive than any other mode, not less — while `file_read`/
  `browser_read`/`browser_screenshot` still ask.
- **Full Auto lets `file_read`/`file_write`/`file_edit` touch paths outside
  the working directory.** This is the one mode that intentionally loosens
  `util.SafeJoin`'s normal confinement — it's the explicit, opt-in trade-off
  the mode exists for, matching Claude Code's `bypassPermissions`
  philosophy. Escaping the working directory while the directory is in any
  other mode is still the bug described above.
- **In Plan mode, the model can call `plan_approval` to ask the operator for
  a temporary escalation.** It only ever offers Allow once or Allow session
  (Persist) — never Full Auto — and the choice is never written to
  `directory.json`; it's an in-memory override cleared at the start of the
  next turn, and the operator has to actually answer the prompt for
  anything to change — the model cannot escalate itself. A "Deny" answer
  interrupts the turn the same
  way Ctrl+C does.
- **Provider keys are stored unencrypted** in `provider.json`, written at mode
  `0600` inside a `0700` data directory. They are masked in list output, not
  protected at rest. Anyone who can read your home directory can read them.
- **Configuring an MCP server runs a program.** `mcp add --stdio <command>`
  means that command is launched on the next run. Adding one is equivalent to
  running it yourself.
- **A model asking to do something harmful is expected.** The approval gate,
  the permission tiers, and the working-directory root are the controls. Report
  a way past those, not the request itself.
- **Anyone who can write to the data directory controls the agent** — its
  system prompt, its skills, its MCP servers. The directory is the trust
  boundary.
- **`serve` binds `127.0.0.1` by default and has no TLS.** Exposing it with
  `--host 0.0.0.0` puts a bearer token on the wire in plaintext; terminate TLS
  in front of it.
