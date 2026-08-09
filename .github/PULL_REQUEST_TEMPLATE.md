## What this changes

<!-- What was wrong before, and why this is the fix. Link the issue if there is one. -->

## How it was verified

<!-- What you ran, and what it said. "make test-race" alone is fine if that is genuinely all of it. -->

- [ ] `make test-race` passes
- [ ] New behaviour has a test, or there is nothing to test

## Checklist

- [ ] Follows [docs/CONVENTION.md](../docs/CONVENTION.md): no comments, no `:=`, one `var` block per function in first-use order with `err` last, functions in the order they execute
- [ ] New `.go` files carry the two-line SPDX header
- [ ] Documentation updated if behaviour a user can see has changed
- [ ] My contribution is licensed `GPL-3.0-or-later`, matching the project

<!--
Do not include artwork. assets/ is all rights reserved and outside the GPL.
Do not report a security vulnerability here; see SECURITY.md.
-->
