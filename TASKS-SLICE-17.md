# Tasks Slice 17

Stand up the session and cookie layer: the sessions middleware, cookie
encryption, deferred cookie flushing, and flash-error sharing. These
occupy ordinals 6 (the cookiefinalize wrapper) and 11-12 and 19 of the
default chain, and they supply the identity that slices 18-19 consume.

Carry-forward notes: the slots join the chain at their published
ordinals; the stack builder remains the lever for adjusting the global
default stack from an end-user developer's Sheidan app; and each slot is
suppressible per route group through `sheidan.Chain(base).Without(...)`,
so a stateless API branch can shed `session.*` wholesale while the
browser branch keeps them.

## Middleware: sessions (github.com/gin-contrib/sessions)

Restores a named session per request and saves it when mutated. Consult
`.refs/md/sessions.md` and the fuller checkout under `.refs/src/sessions/`
for the store matrix.

- Slot: `session.restore`, ordinal 11, ahead of every consumer
  (authentication, CSRF, flash errors, authorization subject derivation).
- Logical default: all HTTP requests. Restoration is cheap and
  idempotent, so blanket scope beats picking routes; the interesting
  scoping question is the backend, not the surface.
- Storage default: the GORM-backed store
  (`github.com/gin-contrib/sessions/gorm`,
  `NewStore(db, rememberMe, keyPairs...)`) riding the same `*gorm.DB`
  that `db.Open` hands the app. Sessions live next to the domain data,
  migrations ride the existing `AutoMigrate` call, and no extra service
  appears in the demo. The cookie-backed store stays available as the
  zero-infrastructure alternative for apps that prefer it; the builder
  chooses between them by configuration.
- Required parameters: the session name (default `sheidan`), the
  keypairs for signing (two keys: one hashes, one encrypts; drawn from
  `SHEIDAN_SESSION_KEYS`, falling back to an ephemeral random pair in
  development with a loud startup warning that sessions will not
  survive restarts), and the remember-me flag controlling durable
  cookies.
- Edge cases: the GORM store migrates its own table; run that migration
  before the first request, not lazily, or cold starts race. Saving is
  lazy (mutation-triggered), so a read-only request never writes. The
  cookie fallback caps payload size (browser cookie limits), which the
  GORM store avoids; document the ceiling so nobody parks megabytes in
  cookie sessions.
- Work: add the dependency, wire the slot, extend the demo's migration
  list, and test restore/save/isolation across two concurrent sessions
  against the SQLite file.

## Middleware: EncryptCookies

Seals outbound cookie values with an authenticated cipher (AES-GCM) so
clients and intermediaries cannot read or forge them; decryption is the
inverse on ingress. This is the anti-tamper complement to the sessions
signature: sessions signs, this seals.

- Slot: `resp.cookiefinalize`, ordinal 6, the wrapper lane. It brackets
  `c.Next()` and rewrites every `Set-Cookie` value belonging to a
  protected class before the response leaves the app; on ingress the
  companion decoder unwraps protected values before any middleware
  reads them. Bracketing from ordinal 6 guarantees it sees cookies set
  by every later participant, including the session saver at 11.
- Logical default: engagement is per cookie class, defaulting to the
  session cookie and any cookie whose name carries the `enc.` prefix.
  Ordinary anonymous cookies pass through unsealed, so the demo behaves
  identically until an app adopts the class.
- Required parameters: the sealing key (drawn from
  `SHEIDAN_COOKIE_KEY`; ephemeral in development with the usual
  restart-warning) and the protected-name classifier.
- Edge cases: sealing inflates values (nonce and ciphertext), pushing
  cookie-backed setups nearer browser limits, which is one more vote
  for the GORM default; keys must rotate without orphaning live
  cookies, so the classifier accepts a primary plus retired keys tried
  in order; values that fail to unwrap degrade to absence (the cookie
  is treated as unseen), never a crash, and the degradation is logged.
- Work: seal/open primitives with vector tests, the wrapper hook, the
  ingress decode, and a rotation scenario test.

## Middleware: AddQueuedCookiesToResponse

Lets code park cookies during request handling and have them land in
the final response, solving the classic lost-update problem where an
earlier `Set-Cookie` for the same name is overwritten by a later one in
the same exchange (login rotates a session id, cleanup clears a temp
marker, and the naive last-writer wins loses the rotation).

- Slot: `resp.cookiefinalize`, ordinal 6, sharing the wrapper with
  encryption: the queue drains first (materializing the surviving
  cookie set), then encryption seals the survivors. One wrapper, two
  duties, one place to reason about the final cookie set.
- Logical default: passive; the queue starts empty on every request and
  activates only when a handler or middleware pushes entries. No
  behavioral difference for apps that never queue, which keeps the slot
  safe to ship blanket-enabled.
- Required parameters: none. The queue API offers enqueue (name, value,
  attributes), supersede (same-name entries replace), and the implicit
  drain-at-response-end.
- Edge cases: a plain `c.Cookie` write after the drain is impossible
  (the response is departing), so the API steers all late cookie work
  through the queue; queue entries participate in dedupe by name, so
  rotating then deleting a cookie nets to a deletion, matching
  developer intuition; the drain runs inside the wrapper, before
  compression touches the body, so `Set-Cookie` headers are finalized
  exactly once.
- Work: the queue primitive, the drain hook, and tests covering
  supersede, dedupe, and the rotate-then-clear scenario.

## Middleware: ShareErrorsFromSession

Surfaces errors parked in the session (flash errors) onto the next
response, so a failed action explains itself on the screen the user
lands on next, then forgets them.

- Slot: `session.flash_errors`, ordinal 19, after identity and policy
  (errors addressed to strangers are meaningless) and ahead of the
  cache layer (a cached response must not replay yesterday's flash
  onto tomorrow's visitor; flashes are inherently uncachable).
- Representation: JSON responses gain a `warnings` array; HTML shells
  gain a dismissible alert region rendered by the templ components.
  Consumption is pop-semantics: the moment a flash is delivered it is
  cleared from the session, so reloading the page does not resurface it.
- Logical default: all HTTP requests, dormant until something parks an
  error. Parking happens through a small API (`Flash(c, msg)`); the
  middleware only collects and presents.
- Required parameters: none mandatory. Optional: a retention bound
  (drop undelivered flashes older than N hours, defending against
  abandoned-browser residue) and a severity tag for styling.
- Edge cases: popping must tolerate a vanished session (expired, wiped
  by authentication invalidation in slice 18) without treating it as an
  error; simultaneous writes to the flash namespace serialize through
  the session store's transaction, or two racing handlers lose one
  message.
- Work: the Flash API, the presenter for both representations, and
  tests proving single-delivery, abandonment aging, and survival across
  a redirect hop.

## Demonstrations

- Visiting a new session plants the session cookie (encrypted, so the
  raw value is opaque gibberish in the browser inspector); revisiting
  increments a demo counter stored in the session, proving GORM-backed
  persistence across processes.
- Rotating the session identifier via the queue (simulating login)
  leaves exactly one cookie for the name in the final response, not the
  stale predecessor.
- Triggering a simulated failure on `POST /notes` (invalid payload)
  parks a flash; navigating to the notes list shows the warning once,
  and a second visit shows none.
- Disabling `session.*` slots on a dedicated API group leaves that
  group free of session cookies while the browser group keeps them.

## Done when

- Slots 6, 11, and 19 hold working implementations; the sessions GORM
  table migrates through the existing app bootstrap.
- Key material draws exclusively from environment variables (ephemeral
  in development, warned), with no secrets committed to the repo.
- The four demonstrations reproduce from a fresh clone, including the
  per-group exclusion case.
- `go vet ./...`, `staticcheck ./...`, and `go test ./...` all pass.
