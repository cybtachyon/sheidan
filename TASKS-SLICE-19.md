# Tasks Slice 19

Put the brakes on: rate limiting, authorization, and request timeouts,
occupying ordinals 15, 16, and 21 of the default chain. Together they
bound how much work one client can spend, who may spend it, and for
how long, and they sit close enough to the handlers that a breach
costs almost nothing.

Carry-forward notes: the three slots join the chain at their published
ordinals; the stack builder stays the adjustment surface for the
global default stack from an end-user developer's Sheidan app; and
every slot here is group-excludable via `sheidan.Chain(base).Without(...)`,
a routine move for batch-job routes (heavy but infrequent, so the
limiter and the timeout both loosen) and for administrative panels
(where the authorization policy tightens instead of shedding).

## Middleware: RateLimitRequests

Counts requests per client address inside a rolling window and answers
429 with `Retry-After` once the allotment exhausts, blunting brute-force
flooding and accidental client storms.

- Slot: `policy.rate_limit`, ordinal 15, before authorization so a
  flood pays the cheapest check first and never reaches rule
  evaluation. The client address is the normalized one (proxied-for
  respected per the engine's trusted proxy policy, finalized in slice
  21), so a load balancer does not masquerade thousands of users as one
  lucky IP.
- Logical default: engaged for all HTTP requests at a permissive pace
  (suggested floor: sixty requests per minute per address with a short
  burst allowance), tightened by app configuration. Routes that stream
  or legitimately pound (polling endpoints, websocket upgrades) opt
  into their own allowances through per-route configuration rather
  than dropping the slot.
- Mechanism: fixed-window counters with jittered boundaries (jitter
  smears the cliff at window rollover, preventing synchronized
  retries into a thundering herd), evicted on staleness to keep memory
  bounded even against spray attacks with millions of synthetic
  addresses.
- Required parameters: the window length, the allowance per window,
  the burst factor, and the eviction horizon. The 429 body names the
  exhausted dimension and the retry delay.
- Edge cases: distributed deployments shard the counters per replica,
  which understates a fleet-wide flood proportionally to replica count;
  document that honesty, and offer the upgrade path (a shared-backend
  counter) as a seam, not a surprise. Clock jumps (NTP slew) stretch
  or starve windows, so counters advance on monotonic time.
- Work: the counter core with jitter and eviction, the route-affine
  allowance table, and tests covering the cliff, the burst, the
  eviction, and the monotonic-clock immunity.

## Middleware: authz (github.com/gin-contrib/authz)

Applies rule-based authorization to admitted requests, deciding
permit or deny from the triple (subject, object, action) through a
Casbin enforcer. Clarifying the mandate: this module authorizes; it
does not authenticate. The subject it judges arrives stamped by
`session.authenticate` (ordinal 12), so the pair divides labor:
authentication knows who speaks, authorization decides what that whom
may do. Consult `.refs/md/authz.md` and the checkout under
`.refs/src/authz/`.

- Slot: `policy.authorization`, ordinal 16, the penultimate gate
  before the timeout wraps the actual work. Object is the request URI
  path; action is the HTTP method; subject is the principal identity
  from the session stamp (anonymous traffic carries the public
  pseudo-principal).
- Logical default: the slot ships open-by-default with no policy
  loaded (the enforcer declines to judge what it has no rules about),
  matching the rest of the stack's philosophy that engagement follows
  declaration. Loading a policy flips the regime; the demo ships a
  small RBAC model and policy file defining viewer/editor/curator over
  the notes resources, demonstrating the realistic middle rather than
  maximal lockdown.
- Mapping: the enforcer's subject source is a pluggable extractor
  (default: the session stamp; a bearer-token app supplies its own),
  which is the seam where "session data and credentials" feed the
  judgment, per the original mandate.
- Required parameters: the model and policy sources (embedded files in
  the repo for the demo; paths for apps), the subject extractor, and
  the denial presentation (403 with a neutral message revealing nothing about the policy model;
  verbose rule debugging goes to the log, never the client).
- Edge cases: Casbin loads policy eagerly; a malformed policy file
  fails the build, not the first request, so the loader runs at
  configuration time with a clear error. Dynamic policy edits require
  an explicit reload call; the API exposes it for ops tooling. Wildcard
  path matching in the policy must agree with Gin's route grammar, or
  a rule that looks loaded quietly governs no route: a smoke test
  walks registered routes and asserts each has a reachable verdict.
- Work: add the dependency, the extractor seam, the demo model and
  policy, the eager-loading loader, and tests walking the demo's route
  table through permit, deny, and anonymous-public cases.

## Middleware: timeout (github.com/gin-contrib/timeout)

Runs the enclosed handlers against a deadline and answers 408 when the
deadline lapses, bounding worst-case latency and converting hung
dependencies into client-visible answers. Consult `.refs/md/timeout.md`.

- Slot: `resp.timeout`, ordinal 21, the innermost lane, directly on the
  handlers. Everything outside it (guards, identity, policy,
  decorators, caching) has already spent its modest work; the deadline
  therefore prices the expensive part, which is what a timeout ought
  to price.
- Mechanics: the module wraps the writer with a buffer, runs the
  handler chain in a spawned worker on a cancellable context, and
  arbitrates completion against the deadline. Partial responses never
  reach the client: either the buffered winner flushes whole or the
  timeout answer does. Panics inside the worker unwind into the
  release-mode rethrow that gin's Recovery (still in the engine,
  outside the wrapper) absorbs.
- Logical default: engaged for all routable requests at five seconds
  (the module's own default), overridden per route where work is
  knowingly long: uploads and externally-mediated calls earn multiples
  of the base, and streaming endpoints (anything flushing incremental
  chunks) are excluded outright, because the buffering premise and
  streaming are mutually exclusive; the exclusion list is mandatory
  knowledge, documented in the route-registration docs.
- Required parameters: the base duration, the per-route multiplier
  table, and the timeout responder (default: 408 with a JSON body
  carrying the request ID, tying the incident back to the logs).
- Edge cases: workers outlive the deadline until the handler notices
  its canceled context; cooperative handlers poll the context, and the
  doc comment says so plainly so future readers stop hunting for a
  magic kill-switch. Clients that abandon a connection shorten the
  effective deadline (cancellation cascades inward), which is desired.
- Work: add the dependency, wire the slot with the multiplier table,
  the 408 responder with the request ID, and tests covering
  timely-success, lapse, panic-in-worker, and the streaming exclusion.

## Demonstrations

- Ninety rapid requests from one scripted client sail through; the
  ninety-first inside the window meets 429 with a retry delay; the same
  storm spread across forty synthetic addresses stays under the radar,
  illustrating per-address scope.
- The demo curator role patches a note and succeeds; the viewer role
  on the same route meets 403; anonymous GETs on the public list
  succeed, proving the open-by-default flip happened only where the
  policy spoke.
- A `slow` demo route sleeping eight seconds (against the five-second
  base) answers 408 promptly with the request ID; doubling the route's
  multiplier lets the identical sleep finish 200.

## Done when

- Ordinals 15, 16, and 21 hold working implementations with the
  documented defaults; the streaming exclusion list is asserted by a
  test so regressions cannot sneak a flusher inside the deadline.
- The rate limiter's memory footprint stays bounded under a million-
  address spray (eviction proven in test).
- The three demonstrations reproduce from a fresh clone, including the
  per-group override cases.
- `go vet ./...`, `staticcheck ./...`, and `go test ./...` all pass.
