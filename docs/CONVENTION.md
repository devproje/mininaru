# Code Convention

This document defines the rules to follow across the project so that
implementation and code review stay consistent.

## 1. Variable Declarations

### Do not use `:=`

Do not use short variable declarations (`:=`) inside functions. Declare every
local variable with `var` at the top of the function body, and use `=` to assign
or reassign values.

```go
// Bad
client := newClient()
result, err := client.Call(ctx)

// Good
var client *Client
var result *Result

var err    error

client = newClient()
result, err = client.Call(ctx)
```

### Declaration placement and order

- As a rule, declare local variables at the very top of the function body in a
  single `var` block.
- Order variables by **first use in the function**, top to bottom. A variable
  used earlier belongs higher in the block.
- Always put `err` last in the declaration block.
- Do not initialize at declaration time. Assign with `=` where the value is
  needed.
- Do not use `:=` anywhere, including the init statements of `for`, `if`, and
  `switch`.

```go
func ServiceCreate(ctx context.Context, input CreateInput) (*Item, error) {
    var request Request
    var item *Item

    var err error

    request = toRequest(input)
    item, err = s.repository.Create(ctx, request)
    if err != nil {
        return nil, err
    }

    return item, nil
}
```

## 2. File and Function Layout

Top-level elements in a file **must** appear in this order:

1. Type declarations such as `struct`
2. `const` declarations
3. Package-level `var` declarations
4. Function declarations

Arrange functions in **dependency order** — not alphabetically and not grouped
by role. Put a helper or callee immediately before the first function that calls
it. That makes each higher-level procedure read after the operations it composes,
with the public entry point last in the file.

If a file declares `main`, `main` **must be the final function and the final
top-level declaration in that file, without exception**. No helper, type,
constant, variable, or other declaration may appear below it.

```go
type Config struct {
    Address string
}

const defaultAddress = ":3000"

var config Config

func loadConfig() error {
    config.Address = defaultAddress
    return nil
}

func startServer() error {
    return nil
}

func main() {
    var err error

    err = loadConfig()
    if err != nil {
        panic(err)
    }

    err = startServer()
    if err != nil {
        panic(err)
    }
}
```

- Do not place type, constant, or package-level variable declarations after
  function declarations.
- Do not sort functions by name.
- When one caller invokes several independent helpers, keep those helpers in the
  same order in which the caller invokes them.
- A helper shared by several callers belongs before its earliest caller. Do not
  duplicate or move it to an unrelated role-based section.
- In a file without `main`, apply the same dependency order so that the
  package's public entry point or representative function comes last.

## 3. Comments

Do not write explanatory comments. If a block needs one to be understood, the
names are wrong or the function is doing too much. Fix that instead.

Three kinds of comment are allowed, and nothing else:

- The two-line SPDX header every `.go` file starts with. It is the per-file
  licence notice the GPL asks for, and tooling reads it.
- Compiler directives such as `//go:build` and `//go:embed`.
- A `//nolint` or equivalent pragma a tool requires to work.

```go
// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules
```

A new file that is missing the header is incomplete. Copy it from any
neighbouring file rather than retyping it.

## 4. API Paths

The externally exposed OpenAI-compatible API should follow OpenAI's request and
response format and behavior as closely as possible, but every endpoint path
**must** begin with `/api/v1`.

- Correct: `/api/v1/chat/completions`, `/api/v1/models`
- Forbidden: `/v1/chat/completions`, `/chat/completions`, `/api/chat/completions`
- Keep the path identical across router registration, API documentation,
  examples, and tests.
- Even when referring to an original OpenAI path, apply the `/api/v1` prefix to
  the actual URL the project serves.

## 5. Logging

Diagnostics go through `util.Log`, the shared `log/slog` logger. Do not use
`log.Printf`, and do not write diagnostics to `os.Stderr` with `fmt.Fprintf`.

```go
// Bad
log.Printf("mcp server %q unavailable: %v", name, err)
fmt.Fprintf(os.Stderr, "mcp server %q unavailable: %v\n", name, err)

// Good
util.Log.Error("mcp server unavailable", "server", name, "error", err)
```

- The message is a short, constant, lowercase sentence. Everything variable
  belongs in a key/value attribute, never interpolated into the message.
- Levels: `debug` for per-request detail, `info` for lifecycle events, `warn` for
  a recovered or ignored problem, `error` for a failed operation.
- Never log a secret. Log whether one was present, not what it was.
- Inside an HTTP handler use `requestLogger(ctx)` instead of `util.Log` so the
  record carries the request's `request_id`.
- `time`, `level`, `msg`, and `source` are slog's own keys. The handler renames a
  colliding attribute to `attr_<key>` rather than emitting a duplicate, but pick
  a different key in the first place.

Output written for the user — a command's results, the `-p` answer, tool progress
in `-p` mode — is not logging. It keeps using `fmt` and goes to stdout, except
`-p` tool progress and reasoning, which the README documents as stderr.

## 6. Early Return

Use early returns to reduce nested conditionals and keep the happy path easy to
read. Return immediately on any condition that prevents further processing, such
as an error, a failed input validation, or insufficient permissions.

```go
// Bad
if input.Valid() {
    item, err = service.Create(ctx, input)
    if err == nil {
        return item, nil
    }
}
return nil, err

// Good
if !input.Valid() {
    return nil, ErrInvalidInput
}

item, err = service.Create(ctx, input)
if err != nil {
    return nil, err
}

return item, nil
```

- Check failure conditions first and return right away.
- Keep the normal path as the main flow of the function, indented as little as
  possible.
- If cleanup must run before returning, use `defer` or perform the cleanup
  explicitly before the return.
