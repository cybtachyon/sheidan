# Tasks Slice 21

Finish the service edge: static file serving, internationalization,
trusted proxy and security-header configuration, and graceful
shutdown. Completing this slice brings every mandated middleware of the
program (slices 15-21) into the default stack and retires the last
hand-rolled corner of `sheidan.New`.

Carry-forward notes: the placements below join the chain and the
engine facade at the positions stated; the stack builder remains the
adjustment surface for the global default stack from an end-user
developer's Sheidan app; and every element stays suppressible per
route group via `sheidan.Chain(base).Without(...)`, so a pure-API
deployment sheds `lang.i18n` and the static mount while a storefront
keeps them all.

## Facility: static (github.com/gin-contrib/static)

Serves files from a rooted directory (or an embedded filesystem) under
a mount prefix, falling through to the next handler when a file is
absent so it composes with route registries and `NoRoute`.

- Placement: a mount, not a chain slot. The demo roots it at
  `/static` over `cmd/sheidan/static` (created here, holding a favicon
  and a stylesheet), and the long-standing exception for
  `/web/web.js` (today a lone `engine.StaticFile` call) migrates onto
  the same facility as a second mount, erasing the special case. The
  embedded-filesystem flavor (`EmbedFolder`) stays available for
  deployments that pack assets into the binary.
- Logical default: engaged in the demo from day one (assets are
  universal); an app that serves nothing static omits the mount, and
  omission costs nothing because nothing in the chain depends on it.
- Required parameters: the prefix and the filesystem root (directory
  path or `fs.FS`). Optional: the browse-directory switch (off by
  default; directory indexes invite surprises) and the content-sniff
  toggle (off; extensions dictate type).
- Edge cases: traversal containment is the library's job, but the
  mount prefix must not shadow a registered route, or a file wins
  over a controller unexpectedly; the structured logger (slot
  logging.slog) skips these paths as declared in slice 15, keeping
  access chatter out of the request log; immutable-looking assets benefit from the cache layer's policy
  tooling (slice 20) when an app wishes to negotiate long TTLs.
- Work: add the dependency, the two mounts, the new asset directory,
  the `NoRoute` collaboration test, and a traversal-hostility test.

## Middleware: i18n (github.com/gin-contrib/i18n)

Resolves the requester's language from `Accept-Language` (with
fallbacks) and hands translated messages to handlers through the
bundle. Consult `.refs/md/i18n.md`.

- Slot: `lang.i18n`, ordinal 17, after policy and before the
  transformers, so handlers decorate with translations without paying
  resolution cost twice; the resolution result (chosen language tag)
  lands in the request context for templates and logs alike.
- Wiring: `Localize(WithBundle(cfg))` with a bundle config naming the
  catalog root, the accepted languages, the default language, the
  fallback chain, and the marshal function. The demo ships JSON
  catalogs for English and Mandarin under `cmd/sheidan/locale`,
  English as default, English as ultimate fallback, and the catalog
  keys seed the demo's button and toast texts, proving the pipeline
  end to end through a templ component.
- Logical default: engaged with the demo catalog; apps without
  catalogs run the middleware in passthrough mode (resolution still
  records the negotiated language, message lookups return the key),
  so enabling it is preparation, not commitment.
- Required parameters: the bundle config (mandatory fields: root,
  accepted list, default). The middleware's own message API
  (`MustGetMessage`, `HasLang`, the getters) is the only surface
  handlers need; a small Sheidan shim re-exports it with the
  framework's naming so app code never imports the vendor package
  directly.
- Edge cases: an unrecognized `Accept-Language` falls through the
  chain to the default rather than erroring; a missing key in the
  chosen language descends the fallback list, and only total absence
  yields the key as its own rendering (visible, never fatal); the
  negotiated language influences cache keys (vary dimension, slice
  20), or a German speaker inherits an English cached page.
- Work: add the dependency, the shim, the demo catalogs, the vary
  linkage, and tests for negotiation, fallback descent, and
  passthrough mode.

## Configuration: secure (github.com/gin-contrib/secure)

Two distinct duties wear this name, and this slice delivers both.
Facet one normalizes proxy reporting so the application learns the
real client address, protocol, and port behind a reverse proxy or
load balancer. Facet two sets the security response headers
(strict-transport, framing, sniffer controls, content policy) that
round out the threat surface. Consult `.refs/md/secure.md`.

- Facet one (trusted proxies): today `sheidan.New` hardcodes
  `SetTrustedProxies(nil)`, declaring no proxy worthy of belief, which
  suits a direct-connected demo and blinds the app behind a real
  front door. The facade grows a setter: `sheidan.New(proxies ...)`
  accepts a list of CIDRs (parsed from
  `SHEIDAN_TRUSTED_PROXIES`, comma separated), forwarding to
  `SetTrustedProxies`. An empty list preserves the historic
  all-direct behavior, so existing apps feel nothing. With a list
  present, `ClientIP` reports the proxied-for address, and every
  address-reasoning consumer (the bad-target blocker, the rate
  limiter, the log's ip field) inherits the correction without
  knowing about it.
- Facet two (security headers): the `resp.cookiefinalize` wrapper
  (ordinal 6) gains a header-emission duty, delivering the hardened
  headers on every response it touches, guard refusals included,
  without disturbing the frozen chain. Profiles: the development
  profile sets nosniff, a permissive content policy, and the IE
  no-open header, and refrains from strict-transport and
  ssl-redirection (neither makes sense on plaintext localhost); the
  production profile approaches the library's `DefaultConfig` but
  drops the deprecated browser-xss-filter header and keeps the
  transport-security seconds configurable, so deploying behind a
  terminator that terminates TLS earlier does not produce contradictory
  directives. Profile selection is configuration, defaulting to
  development.
- Logical default: facet one inert (no proxies declared) and facet
  two on in its development profile, so every response ships sane
  headers from the first run.
- Required parameters: facet one takes the CIDR list; facet two takes
  the profile and, within it, the transport-security duration and the
  content policy string.
- Edge cases: claiming a trusted proxy for a range you do not control
  invites header forgery, so the parser rejects private ranges with a
  warning unless an explicit override acknowledges the risk (front
  doors on private networks are the exception that must confess);
  ssl-redirection enabled on a plaintext listener loops forever in
  some clients, hence its confinement to the production profile.
- Work: the facade setter, the profile table, the wrapper extension,
  and tests for address decoding under a fake proxied request, the
  private-range confession, and the header sets per profile.

## Facility: graceful (github.com/gin-contrib/graceful)

Retires the bare `engine.Run` so the process honors SIGINT and
SIGTERM: active requests finish, resources drain, and the exit is
quiet. Consult `.refs/md/graceful.md`.

- Shape: `graceful.New(engine, opts...)` wraps the prepared engine;
  the runner becomes `RunWithContext(ctx)` where the context is a
  signal-scoped context (`signal.NotifyContext` over SIGINT and
  SIGTERM). Startup options: the server timeouts (read fifteen
  seconds, write thirty, idle sixty, the module's defaults), the
  shutdown timeout (thirty seconds), a before-shutdown hook (close the
  listener to new connections, flush the structured logger), and an
  after-shutdown hook (close the database handle).
- Logical default: this is process furniture, always engaged in the
  demo and offered as the standard runner in the facade, so an app's
  `main` shrinks to "prepare the engine, prepare the database, run."
  There is no meaningful off switch; refusing graceful death is the
  defect.
- Required parameters: the signal set (fixed: INT, TERM) and the two
  hook chains; everything else carries the defaults above.
- Edge cases: a hook that hangs stretches the shutdown toward the
  timeout, so hooks receive the shutdown context and must respect its
  deadline (logger flush does; the database close does, with a
  bounded wait); a second signal during drain escalates to abrupt
  close rather than ignoring the operator, matching platform
  expectation; the run call reports `context.Canceled` as the quiet
  exit, not an error, so supervision sees a clean zero.
- Work: add the dependency, the facade runner hiding the signal
  plumbing, the two demo hooks, and tests simulating TERM mid-request
  (completion observed, listener closed, exit code zero) plus the
  stuck-hook timeout path.

## Program close-out

With this slice, the mandated roster is fully seated: fourteen
third-party modules (requestid, slog, static, timeout, secure, cors,
sessions, cache, authz, httpsign, pprof, i18n, gzip, graceful) and
seventeen built middlewares (maintenance gating, bad-target blocking,
post-size validation, header validation, string trimming, CSRF
validation, cookie encryption, cookie queuing, flash-error sharing,
session authentication, redirect-if-authenticated, email
verification, rate limiting, cache-head emission, cache tagging,
route-level caching, binding substitution), joined by the facade
settings that accompany them. The offline references in
`.refs` (distilled notes plus full checkouts of authz, cache,
httpsign, and sessions) remain the consulting desk for any follow-on
tuning, and the demo app doubles as the runnable exhibit for every
claim above.

## Demonstrations

- `/static/css/site.css` and `/static/favicon.ico` serve from the new
  directory with correct content types; a nonexistent file falls
  through to the friendly 404, and a traversal attempt meets 400.
- `Accept-Language: zh` renders the demo chrome in Mandarin; an
  unknown language lands in English; the negotiated tag appears in
  the structured log.
- Behind a mocked proxy (request carrying a forwarded address from a
  declared trusted CIDR), the bad-target blocker and the log's ip
  field report the forwarded address; without the declaration they
  report the proxy itself, proving the toggle's teeth.
- Sending SIGTERM to the running demo mid-note-edit lets the edit
  commit, watches the listener close, and observes a zero exit with
  the drain logged; a second SIGTERM during a wedged hook forces an abrupt close.

## Done when

- The demo boots through the facade runner and dies politely on both
  signals, with the hooks firing in order.
- `sheidan.New` carries no hardcoded proxy policy and no bare
  `engine.Run` remains in the demo; the static exceptions are
  consolidated under the facility.
- The header profiles emit their respective sets on every response
  class, including guard refusals.
- All four demonstrations reproduce from a fresh clone.
- `go vet ./...`, `staticcheck ./...`, and `go test ./...` all pass,
  closing the program opened in slice 15.
