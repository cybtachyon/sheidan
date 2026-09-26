# Overview
- This is a golang web framework based on Gin, Templ, and GopherJS.
- It is loosely inspired by [Laravel](https://laravel.com/framework/docs/structure)

## Layout
- The root package is the public framework API (package sheidan): New, Wrap, and Bind. The db/ package is the public GORM connection layer (Open, WithAuthToken). The demo app and its CLI are in cmd/sheidan.
- cmd/sheidan is the demo app and its CLI. With no argument it serves the demo app; the web subcommand transpiles the GopherJS client, and test-web runs the client's tests. It installs as the sheidan command.
- webbuild/ is the self-bootstrapping GopherJS toolchain, reusable by any app with a GopherJS client. Build(dir, out) transpiles the client in dir to out, provisioning a Go 1.21 SDK (in the user cache) and a gopherjs CLI on first use. Test(dir) runs the client's tests.
- test/hello is a nested module with its own go.mod (replace ../..). The root's ./... patterns skip it. Run its vet, staticcheck, and tests from its directory.
- cmd/sheidan/web is a nested GopherJS client module (go 1.21, the version GopherJS 1.21.0 requires). The root's ./... patterns skip it. Build with `make web`, test with `make test-web`.
- tools.go pins tool dependencies for go mod tidy. Its tools build tag excludes it from normal builds, so it coexists with the root package in one directory.

## Dev Tips
- Compare your knowledge snapshot of dependencies to the current version of dependencies. e.g.,
    - Before wrapping templ and gin types, verify the signatures of `templ.Handler`, `templ.Component`, and `gin.WrapH` in the module cache at the exact versions in go.mod.
- Follow Go conventions: [Effective Go](https://go.dev/doc/effective_go), [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments), and [Go Proverbs](https://go-proverbs.github.io/). Fetch all of these pages before writing code. Be concise, declarative, and factual.
- Query the codebase using the `codegraph` tool, e.g., `codegraph node structName` to read a type before calling it. Prefer using existing API and programming patterns over implementing new code.
- When implementing, do not rely on memory. Instead, look up the tool's documentation for the version being implemented or use online resources to ensure accuracy and reliability.
- Use unit tests to answer questions, verify code, and test functionality.
- GopherJS client code: the js package (v1.21.0) has no js.Value interface; use *js.Object. The stdlib http client rejects relative URLs, so resolve paths against location.origin. Under Node, Go 1.21 disables the fetch transport (go.dev/issue/57613) and the net package is a fake network, so HTTP requests only complete in a real browser; test response handling separately. A *js.Object holding JavaScript null compares equal to nil (the package has no IsNull), so check a querySelector result with == nil. js.Object.Set and Get take string keys, so build indexed JS objects with string indices (list.Set("0", el), list.Set("length", n)); querySelectorAll returns a static list, so removing elements while iterating is safe.
- Do not put secrets or personal data in content files.

## Tooling
- templ: The CLI version must match the templ library version in go.mod. The CLI is pinned in mise.toml via the `go:` backend. Run `templ generate` after editing a `.templ` file, and commit the generated `*_templ.go` files. String interpolation uses `{ expr }`. The `@` prefix is for element expressions like `@list()`, not strings. Imports in a `.templ` file go after the `package` line; they are emitted as Go code nodes. An expression attribute on `<a href>` is sanitized through `templ.URL` (a scheme like `javascript:` fails sanitization) and HTML-escaped, so a plain path like `/note/1` passes through unchanged.
- DCDC: A Rust CLI, not a Go module, and not in the standard mise/aqua registry. Install it via its install script or a local mise plugin.
- GopherJS: The compiler requires a Go 1.21 GOROOT, and the CLI must be built with Go 1.21-1.26 (a CLI built with 1.27+ panics when compiling the js package). webbuild bootstraps this automatically: when the user's Go is not 1.21 it downloads the Go 1.21.13 SDK to the user cache dir, builds the gopherjs CLI with the user's Go (1.21-1.26) or the SDK's Go (1.27+), and transpiles. The transpiled output web/web.js is gitignored; the demo app serves it at /web/web.js. The gopherjs pin in tools.go tracks master; if a future release accepts a modern GOROOT, the SDK download disappears and the setup collapses to `go install github.com/gopherjs/gopherjs@vX`.
- The gorm sqlite driver is cgo (mattn/go-sqlite3).
- GORM: `db.Open` handles `file:` (local) and `libsql://`, `http(s)://`, `ws(s)://` (remote) DSNs via the morelj/gorm-sqlite-libsql driver, a fork of the official GORM SQLite driver. `ErrRecordNotFound` is a sentinel error; check it with `errors.Is`, which also matches errors GORM wraps with `%w`.
- The libsql driver rejects query parameters in the DSN. Pass the auth token with `db.WithAuthToken`.

# Code Style
- Use a declarative and explicit coding style. Ensure single sources of truth and use language mechanics for deterministic behavior. Use tools like Structs, Receiver Functions, and Interfaces if they fit the problem.
- Organize code by dependency, grouping by constructor and component usage. Prefer `internal/` packages to keep code DRY and avoid reinventing the wheel.
- Never use grammatical shortcuts like emdash. Use separate sentences first or commas and semicolons if necessary. Avoid pronouns and adverbs as much as possible.
- **Doc comments** start with the name and state what it does. Structure: `// [Name] [verb]s [what]. [Optional: when/why to use it].` Put useful information where users make decisions (usually the constructor, not methods).
- **Inline comments** are terse. Prefer end-of-line when short enough.
- **Explain why, not what.** The code shows what it does; comments should explain reasoning, non-obvious decisions, or edge cases.
- **Wrap comments** at ~80 characters, continuing naturally at word boundaries.
- **Naming matters.** Before proposing a name, stop and review existing names in the file. Ask: what would someone assume from this name? Does it fit with how similar things are named? A good name is accurate on its own and consistent in context. Do not use single letter variable names besides the common `i, j, k, n, x, y, z` in loops.
- **Comment content:**
    - Add information beyond what the code shows (not tautologies)
    - State directly: "Returns X" (not "Note that this returns X")
    - Drop filler: "basically", "actually", "really" add nothing

## Conventions
- None yet, we will add them as we go.

## Docs
- [Dagger Cookbook](https://docs.dagger.io/0.21/cookbook) Fetch this page for dagger code examples.

## Testing Instructions
- Run `go vet ./...`, and `staticcheck ./...` to catch errors. If they're not available, install them.
- Run `go test ./...` to run all tests.
- Run `go test -bench .` to run benchmarks.

## Ongoing Development
- Design next steps as the smallest unit of actionable items with a concrete goal, then write them into [TASKS-SLICE-##.md](TASKS-SLICE-##.md). Add, commit separately, then remove any task slices that have been completed, as they will be retained via Git History.
- Track progress in [STATUS.md](STATUS.md). Compact it after every update.
- As the last step, update [AGENTS.md](AGENTS.md) with any learnings and garbage collect (delete) wasteful verbiage and tokens to keep the file lean.
