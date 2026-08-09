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
  `bash_exec` — anything that makes the root in `modules/file.go` not hold.
- Reaching a private, link-local, loopback, or cloud-metadata address through
  `web_fetch`, past the guard in `modules/webguard.go`.
- Running a `dangerous` or `privileged` tool without the approval gate, when
  `--allow-dangerous-tools` was not passed. This includes prompt injection that
  defeats the gate, as opposed to injection that merely asks for a tool.
- Discord: answering someone who is not paired, acting as another user, or
  climbing from the user role to admin without a pairing code.
- A provider API key, bot token, or `MININARU_API_KEY` appearing in logs,
  terminal output, an API response, or an error message.
- Any data from one API request surfacing in another. `serve` is stateless and
  keeps no conversation between requests, so leakage across them is a bug.

## Not vulnerabilities

These are documented, deliberate behaviour. A report about them will be closed
with a pointer here.

- **`bash_exec` runs shell commands and `file_write` writes files.** That is
  what the tools are. In the chat client each dangerous call is gated by an
  approval prompt; `--allow-dangerous-tools` removes that gate on purpose, and
  `-p` denies them outright because nobody is there to approve.
- **Provider keys and bot tokens are stored unencrypted** in `provider.json`
  and `bot.json`, written at mode `0600` inside a `0700` data directory. They
  are masked in list output, not protected at rest. Anyone who can read your
  home directory can read them.
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
