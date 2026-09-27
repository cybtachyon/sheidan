# Tasks Slice 18

Give the stack a notion of who is talking: session authentication,
CSRF token validation, HTTP-message signature validation, and the two
account-state gates (redirect-if-authenticated, ensure-email-verified).
These fill ordinals 12-14 of the chain plus two per-route attachments
that ride on top of the chain wherever an app places them.

Carry-forward notes: ordinals 12-14 join the default chain; the stack
builder remains the way an end-user developer tunes the global default
stack from their Sheidan app; and each slot admits group-level
exclusion via `sheidan.Chain(base).Without(...)`, so API-only branches
(sharing tokens instead of session cookies) commonly shed
`safety.csrf` while keeping everything else.

## Middleware: AuthenticateSession

Invalidates sessions that have gone stale: expired lifetimes, revoked
identifiers, or principals whose credential state drifted underneath
them (a password change bumps a generation counter stored in the
session; a mismatch voids the session).

- Slot: `session.authenticate`, ordinal 12, directly on the restored
  session. Success stamps the principal (identifier, generation,
  roles-as-data) into the request context for the authorization lanes
  ahead; failure destroys the session, clears the cookie through the
  queue wrapper, and answers 401 on JSON routes or a redirect to the
  login route on HTML routes.
- Logical default: engaged but tolerant by default. With no
  authentication requirements registered, the slot passes anonymous
  traffic straight through and merely validates session freshness when
  a session is present. It becomes enforcing when an app registers
  predicate hooks (expiry horizon, generation check, revocation list),
  so adopting it costs nothing and switching it on is a configuration
  act, not a refactor.
- Required parameters: none mandatory. Predicates register through a
  small hook table: each hook names a condition, a violation response,
  and whether it applies to anonymous traffic (generation and
  revocation hooks decline to judge anonymous visitors; expiry judges
  anyone holding a ticket).
- Edge cases: destruction must funnel through the cookie queue, or the
  clearing `Set-Cookie` races the session saver; a session destroyed
  mid-exchange must cancel any flash parking (nothing to deliver to a
  stranger); predicate hooks run in registration order and the first
  verdict wins, keeping behavior predictable.
- Work: the hook table, the context stamp, the dual-answer failure
  path, and tests covering expiry, generation drift, revocation, and
  the anonymous passthrough.

## Middleware: ValidateCsrfToken

Validates that state-changing browser requests carry a token matching
the session, defeating forged cross-site submissions.

- Slot: `safety.csrf`, ordinal 13, after authentication (tokens are
  minted into the session, so the issuer must be established first).
- Scheme: synchronizer tokens. The session holds a random nonce,
  mirrored into a cookie for form templates to read; on unsafe methods
  (POST, PATCH, PUT, DELETE) the submitted token (form field or header)
  must match the session nonce. Matching uses constant-time comparison.
- Applicability: enforced on unsafe-method requests originating from
  browsers (cookie-carried). Machine clients presenting bearer or
  signature credentials are exempt, since CSRF presupposes ambient
  cookie authentication that machines do not use. Exemption is judged
  by credential presence, never by origin guessing.
- Required parameters: the token field name (default `_csrf`) and the
  header synonym (default `X-CSRF-Token`), both overridable. Rotation
  policy: regenerate on privilege transitions (login, password
  change), delivered through the cookie queue.
- Edge cases: token minting must occur during session restore, not on
  demand, or the first unsafe request 403s before a token ever existed
  (mint-if-absent solves this); SPA front ends posting to same-origin
  AJAX routes read the mirror cookie and send the header, which the
  demo's client wires up in its fetch layer; duplicated or replayed
  tokens defeat nothing, because the nonce is bound to the session and
  rotated on privilege change, not burned per use.
- Work: mint/check/rotate primitives, the applicability discriminator,
  and tests covering happy path, forgery, missing token, machine
  exemption, and rotation-on-login.

## Middleware: httpsign (github.com/gin-contrib/httpsign)

Verifies RFC-style `Signature` headers (the Camage HTTP Signatures
draft) computed with symmetric keys, protecting webhook and partner
feeds from tampered replays. Consult `.refs/md/httpsign.md` and the
source checkout under `.refs/src/httpsign/`.

- Slot: `safety.signature`, ordinal 14, last of the forgery lane so a
  signed request has already survived CSRF triage (signatures and CSRF
  protect different transports; order between them is immaterial but
  fixed for determinism).
- Scope: this is a route-affinity middleware, not a blanket one. The
  default chain carries it dormant; routes that demand signatures
  activate it with their own secret set, the idiomatic pattern being a
  webhook group (`/hooks/...`) assembled through the group API.
- Mechanics: `httpsign.NewAuthenticator(secretMap)` where the map ties
  each `KeyID` to a secret plus algorithm (HMAC SHA-256 or SHA-512);
  the authenticator parses the header, recomputes the digest over the
  covered components (date, content-digest where present), and compares
  in constant time. Date skew beyond the tolerated window fails the
  check, killing replayed captures.
- Required parameters: the secret map (values drawn from
  `SHEIDAN_SIGNATURE_<KEYID>` environment variables; never literals in
  source), the accepted algorithms, and the date-skew allowance
  (default fifteen minutes).
- Edge cases: unsigned requests to a demanding route fail closed with
  401; a request naming an unknown KeyID distinguishes misrouting from
  forgery in diagnostics (different log category, same 401 outward);
  body digests require the middleware to read the body once and refill
  it for downstream decoders, so the implementation caches the slurped
  bytes in the request context.
- Work: add the dependency, the route-affine wiring, the env-fed
  secret loader, and tests for valid, altered-body, stale-date, and
  unknown-KeyID exchanges.

## Attachment: RedirectIfAuthenticated

Steers already-authenticated principals away from the routes meant for
fresh eyes (login, register, password reset), landing them on the
dashboard instead. Attached per route group, not held in the global
chain: its meaning is intrinsically local to auth-carrying pages.

- Behavior: on the decorated routes, a valid authenticated session
  earns a 302 to the destination (default the home route; configurable
  resolver). Anonymous visits pass untouched. JSON clients get 401
  instead of a redirect, since a machine following hops into a login
  page is a bug, not a person.
- Required parameters: the destination resolver and the credential
  probe (shares the context stamp from `session.authenticate`, so this
  attachment presumes that slot is active on the branch it dresses).
- Edge cases: the probe must read the same stamp the authenticator
  wrote, not re-validate, or a session expiring mid-hop forks the two
  views; decorating the login route twice (accidentally nested groups)
  must collapse, not loop, so the attachment is idempotent per route.
- Work: the attachment, the dual-channel answer, and loop-collapse
  tests.

## Attachment: EnsureEmailIsVerified

Bars routes that presume a confirmed identity until the principal's
email address has been confirmed. Attached per group, reading the
verification state through a predicate the app supplies (typically a
column on the user model populated by a confirmation-link flow).

- Behavior: verified principals pass; unconfirmed ones meet 403 with a
  challenge hint (resend link) on JSON, or a resend panel on HTML. The
  predicate receives the stamped principal, so no second identity
  lookup invents a divergent view.
- Required parameters: the verification predicate and the challenge
  renderer; both are mandatory for the attachment, which makes sense
  because without them the gate has nothing to judge or say.
- Edge cases: the predicate must be total (answer for every shaped
  principal, including ones lacking the field), defaulting to unverified
  when unsure, so a schema migration halfway never unlocks prematurely;
  confirmation events should invalidate dependent sessions through the
  same destruction path as authentication, keeping one chokepoint.
- Work: the attachment, the total-predicate contract, and tests for
  verified, unverified, and missing-field principals.

## Demonstrations

- A session aged past its horizon (clock manipulated in test) is
  refused with 401 and a cleared cookie; changing the demo password
  busts outstanding sessions on their next request.
- Submitting the notes form without a token 403s; with the minted
  token it lands; forging the token from a second origin reproduces the
  403; a bearer-token client posting the same route sails through.
- A signed webhook post computes its `Signature` with the env-loaded
  key and is accepted; altering one body byte, or sleeping twenty
  minutes, rejects it; an unsigned post to the webhook group 401s.
- After logging in, visiting the login route bounces to home; the same
  visit before login stays put. A route group guarded by the email
  verifier admits the confirmed demo user and fences the unconfirmed
  one.

## Done when

- Ordinals 12-14 hold working implementations with the default
  tolerant/dormant postures, and the two attachments are consumable
  through the group API.
- All key and nonce material originates from the environment or the
  session; the repo holds no permanent secrets.
- The five demonstrations reproduce from a fresh clone, including the
  per-group exclusion of `safety.csrf` on an API branch.
- `go vet ./...`, `staticcheck ./...`, and `go test ./...` all pass.
