# Tasks Slice 15

Establish the slot and lane model for the default Sheidan middleware stack,
then bring up the observability tier: requestid, slog, and pprof. Later
middleware slices (16-21) fill the remaining lanes; this slice fixes the
shared vocabulary, the ordering, and the ergonomics API that every one of
them builds on.

## Baseline decisions (binding for slices 16-21)

1. The default stack is an ordered list of named slots. Each slot has a
   stable dot-separated name, a lane number, an ordinal, a kind, and a
   parameter bag. Kinds:

   - guard: inspects the request before `c.Next()` and may abort it.
   - transform: prepares request data before `c.Next()` and lets the
     request continue.
   - state: restores or advances per-client state before `c.Next()`.
   - decorator: augments the response after `c.Next()` returns.
   - wrapper: brackets `c.Next()` and reacts to the finished exchange
     (timing, compression, draining deferred effects).

2. The canonical default chain, in order:

   ```
    1 request.id            passthrough
    2 logging.slog          wrapper
    3 gate.maintenance      guard
    4 gate.badiptarget      guard
    5 resp.compress         wrapper
    6 resp.cookiefinalize   wrapper
    7 api.cors              guard
    8 intake.postsize       guard
    9 intake.requestheaders guard
   10 intake.strings.trim   transform
   11 session.restore       state
   12 session.authenticate  guard
   13 safety.csrf           guard
   14 safety.signature      guard
   15 policy.rate_limit     guard
   16 policy.authorization  guard
   17 lang.localized        decorator
   18 binding.params.subst  transform
   19 session.flash_errors  decorator
   20 cache.response        decorator
   21 resp.timeout          wrapper
   ```

   Slot 6 hosts two cooperating mechanisms (encryption and queue draining
   of Set-Cookie values); it is filled in slice 17. Slots 3-4 arrive in
   slice 16, 7-10 in slice 16, 11 in slice 17, 12-14 in slice 18,
   15-16 and 21 in slice 19, 17-20 in slices 17 and 20, and the
   out-of-chain facilities (static mounting, pprof group, graceful server,
   trusted proxy configuration) in slices 21 and 15 respectively. Until a
   slice delivers a slot, the builder emits a no-op holder so the chain
   length and ordering stay stable.

3. Engine-level facts that are configuration, not middleware: trusted
   proxy policy (`SetTrustedProxies`), the pprof route group, and the
   graceful server wrapper travel beside the chain, owned by slice 21 and
   this slice.

## Ergonomics: adjusting the global default stack

Introduce the stack builder so an end-user developer adjusts the global
default stack from their Sheidan app without copying the chain:

- `sheidan.Stack()` returns a mutable builder preloaded with the default
  chain and permissive defaults for every slot.
- The builder offers `Enable(name)`, `Disable(name)`,
  `Configure(name, cfg)`, `Insert(afterOrBefore name, mw)`, and
  `Remove(name)`. Names are the dotted slot names above; misnamed calls
  return an error at build time, not silent no-ops.
- `Apply()` compiles the chain into a `*gin.Engine`, preserving the
  existing `sheidan.New()` signature as a thin convenience that applies
  stock defaults.
- Unit tests assert the compiled order equals the canonical list, that
  disabling a slot spares its neighbors, and that inserted middleware
  lands at the requested position.

## Group-level exclusion

When a developer assigns middleware to a group of routes (a
`RouterGroup.Use` chain derived from the default stack), the group takes a
filtered copy of the current chain: every default slot is inherited except
those named in the group's exclusion list, and any group-specific
middleware is appended. Express exclusion as a negative selector, e.g.
`sheidan.Chain(base).Without("safety.csrf", "intake.postsize")`.
Document that excluded slots never run on that branch, that the filter is
copied per group (later branches keep the originals), and that wrappers
removed from a branch lose their bracket effect on that branch only.
Later slices repeat this note with their own slot names.

## Middleware: requestid (github.com/gin-contrib/requestid)

Adds an `X-Request-ID` response header to every request, propagating a
caller-supplied `X-Request-ID` unchanged and generating a UUID v4 when
absent. Retrieve the value in handlers and log decorators through
`requestid.Get(c)`.

- Slot: `request.id`, ordinal 1. Earliest in the chain so every later
  consumer (slog records, rate limiter rejection logs, timeout responses,
  cache tags) correlates on the same identifier.
- Logical default: all HTTP requests. Generation and propagation need no
  per-route decisions, so blanket coverage is the right scope.
- Required parameters: none mandatory. Optional: a custom generator
  (`WithGenerator`) and a renamed header key
  (`WithCustomHeaderStrKey`). Stock defaults (UUID v4, standard header
  name) are the Sheidan default; renaming the header breaks interoperation
  with tooling that expects `X-Request-ID`, so treat the rename as an
  expert knob.
- Edge cases: the propagated value is attacker controlled, so log escaping
  and length caps apply when the ID enters logs or response bodies; the
  demo's JSON and HTML representations should ignore the header entirely
  apart from echoing it in the response header.
- Work: add the dependency (pin the current release in go.mod), wrap it
  in the `request.id` slot with a configurable generator, and expose the
  ID through the slog injector and the timeout response.

## Middleware: slog (github.com/gin-contrib/slog)

Structured request logging through Go's `log/slog`. The module requires
Go 1.26+, which the repo's go.mod (1.27) satisfies. Import it aliased
(gslog) to sidestep the name collision with the standard library package.

- Slot: `logging.slog`, ordinal 2. Outermost recorder so it observes the
  outcome of the guards that follow it (their aborts get logged with the
  final status), while staying inside `request.id` so records carry the
  identifier.
- Logical default: all HTTP requests, with skips tuned for noise. Default
  skips: `/static/`, `/web/`, `/debug/pprof/`, and `OPTIONS` prefilters
  (via the skipper hook). Sensible levels: Info below 400, Warn for 4xx,
  Error for 5xx.
- Required parameters: essentially none; the defaults suffice. Provided
  knobs used by the defaults: `WithWriter(os.Stdout)` (gin's stock writer
  is stderr), `WithLogger` that grafts the request ID onto every record,
  `WithHiddenRequestHeaders` extended beyond the built-in list to cover
  CSRF-bearing headers.
- Edge cases: the recorded body size reflects whatever crosses its own
  writer boundary, and because this slot sits outside `resp.compress`
  the number reports compressed bytes; state that in the doc comment.
  Skipping by path must match registered route patterns, not guessed
  substrings.
- Work: add the dependency, wire the slot, and retire gin's plain
  `Logger` from `sheidan.New()` so the two loggers never double-print.
  Keep `Recovery` where it stands.

## Facility: pprof (github.com/gin-contrib/pprof)

`pprof.Register` and `pprof.RouteRegister` attach profiling routes
(`/debug/pprof/profile|heap|goroutine|...`); this is route registration,
not a per-request middleware, so it travels out of chain.

- Placement: a dedicated `RouterGroup` under `/debug/pprof`, mounted only
  when the operator opts in. Default: disabled. Enable with
  `SHEIDAN_DEBUG=1`; an optional token header
  (`SHEIDAN_DEBUG_TOKEN`) can gate the group when the port is reachable
  beyond the developer laptop.
- Required parameters: the prefix (stock `debug/pprof`) and the enable
  switch. The README's Authorization-header group pattern is the
  reference for gating.
- Edge cases: profiling endpoints disclose memory layouts, goroutine
  symbols, and allocation rates; leaving them exposed in a deployed
  service hands an adversary a map. The default-disabled stance plus the
  debug-env gate is the mitigation.
- Work: mount the group behind the env switch in the demo app, add a
  test asserting the routes 404 when the flag is unset, and note in the
  demo's help text how to drive `go tool pprof` against the live server.

## Demonstrations

- Any request's response carries `X-Request-ID`; a second request that
  sends the same header gets it reflected, proving propagation.
- Structured log lines for a deliberate 4xx and a 5xx show the level
  ladder and the correlated ID; a request to `/static/` produces no line.
- With `SHEIDAN_DEBUG=1`, `curl /debug/pprof/heap` answers; without the
  flag the path 404s.

## Done when

- The builder API ships with unit tests proving the canonical order,
  per-slot disable, insertion positioning, and the per-group exclusion
  copy semantics.
- `sheidan.New()` returns an engine whose chain matches the canonical
  list (holders included), with the stock Logger gone and Recovery kept.
- The three demonstrations above reproduce from a fresh clone plus
  `go run ./cmd/sheidan`.
- `go mod tidy` records the new dependencies; `go vet ./...`,
  `staticcheck ./...`, and `go test ./...` all pass.
