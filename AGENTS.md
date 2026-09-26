# Overview
- This is a golang web framework based on Gin, Templ, and GopherJS.
- It is loosely inspired by [Laravel](https://laravel.com/framework/docs/structure)

## Layout
- The root package is the public framework API (package sheidan). The demo app is in cmd/sheidan.
- test/hello is a nested module with its own go.mod (replace ../..). The root's ./... patterns skip it. Run its vet, staticcheck, and tests from its directory.
- tools.go pins tool dependencies for go mod tidy. Its tools build tag excludes it from normal builds, so it coexists with the root package in one directory.

## Dev Tips
- Compare your knowledge snapshot of dependencies to the current version of dependencies. e.g.,
    - Before wrapping templ and gin types, verify the signatures of `templ.Handler`, `templ.Component`, and `gin.WrapH` in the module cache at the exact versions in go.mod.
- Follow Go conventions: [Effective Go](https://go.dev/doc/effective_go), [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments), and [Go Proverbs](https://go-proverbs.github.io/). Fetch all of these pages before writing code. Be concise, declarative, and factual.
- Query the codebase using the `codegraph` tool. Prefer using existing API and programming patterns over implementing new code.
- When implementing, do not rely on memory. Instead, look up the tool's documentation for the version being implemented or use online resources to ensure accuracy and reliability.
- Read structs and types before calling with the `codegraph` tool e.g., `codegraph node structName`.
- Use unit tests to answer questions, verify code, and test functionality.
- Do not put secrets or personal data in content files.

## Tooling
- templ: The CLI version must match the templ library version in go.mod. The CLI is pinned in mise.toml via the `go:` backend. Run `templ generate` after editing a `.templ` file, and commit the generated `*_templ.go` files.
- DCDC: A Rust CLI, not a Go module, and not in the standard mise/aqua registry. Install it via its install script or a local mise plugin.
- The gorm sqlite driver is cgo (mattn/go-sqlite3).
- GORM: `internal/db.Open` handles `file:` (local) and `libsql://`, `http(s)://`, `ws(s)://` (remote) DSNs via the morelj/gorm-sqlite-libsql driver, a fork of the official GORM SQLite driver.
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
