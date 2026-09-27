# Tasks Slice 16

Fill the request-entry lanes of the default stack (ordinals 3-10):
maintenance gating, bad-target blocking, POST size validation, request
header validation, string trimming, and CORS. These run before identity
and policy, so abusive or malformed traffic dies before it spends
database or session work.

House notes carried from slice 15: every slot below joins the default
chain at the published ordinal; the stack builder
(`sheidan.Stack().Disable(name)`, `.Configure(name, cfg)`) stays the way
end-user developers adjust the global default stack from their Sheidan
app; and each slot is eligible for group-level exclusion through
`sheidan.Chain(base).Without(names...)`, so a developer assigning
middleware to a group of routes can suppress any of these for that
branch (uploads routinely shed `intake.postsize` tightening, API-only
branches shed `api.cors`).

## Middleware: PreventRequestsDuringMaintenance

Rejects all traffic with 503 and a `Retry-After` header while the app is
flagged as under maintenance.

- Slot: `gate.maintenance`, ordinal 3 (behind requestid and slog, so
  rejected bursts still reach the log with their identifier).
- Logical default: all HTTP requests, switched off. Activation is
  environmental, not per route: the slot engages when
  `SHEIDAN_MAINTENANCE=1` or when a file named by
  `SHEIDAN_MAINTENANCE_FLAG` exists (deploy scripts toggle the file
  without restarting). Blanket scope is correct because maintenance
  means the whole service, not a subset of it.
- Required parameters: the activation source (env var and/or flag file)
  and the retry interval (default 60 seconds). Optional: an allowlist of
  paths that stay live during maintenance (health probes, static assets,
  a maintenance page), defaulted to `/healthz` and `/static/`.
- Edge cases: flipping the flag mid-life must not hang in-flight
  requests; the check is per request, so in-flight work completes
  naturally. Allowlisted paths must also bypass the other entry guards
  that fire on the same request, or a probe during maintenance trips
  monitoring.
- Work: implement as a small internal package with an activator
  interface (env, file) so tests can swap triggers, plus tests for the
  on/off transition and the allowlist.

## Middleware: BlockBadTargets

Blocks requests coming from listed IP addresses and networks. The task
brief frames this as blocking known malicious addresses; the
implementation loads a blocklist (single IPs and CIDR notation) from a
comma-separated env var (`SHEIDAN_BLOCK_IPS`) or a file pointed to by
`SHEIDAN_BLOCK_IPS_FILE`, parses it once at startup, and rejects matches
with 403.

- Slot: `gate.badiptarget`, ordinal 4, directly after maintenance.
  Matching uses the normalized client address, so it trusts the trusted
  proxy policy configured at the engine level (finalized in slice 21);
  behind a load balancer the proxied-for address is what gets tested.
- Logical default: all HTTP requests, engaged only when a list is
  supplied. An empty list is a no-op, so shipping the slot enabled with
  nothing in it costs one map lookup per request.
- Required parameters: the list source. Optional: a deny status choice
  (403 advertises the refusal; 404 hides it from scanners) and a reload
  cadence for file-based lists.
- Edge cases: IPv6-mapped IPv4 forms normalize to one representation
  before matching, or a blocked `::ffff:a.b.c.d` slips past an `a.b.c.d`
  entry. Private-range entries (10.*, 192.168.*) imply deployment behind
  NAT and deserve a loud startup warning, not silence.
- Work: parser with tests for mixed forms, the mapping normalization,
  and the no-op-empty-list behavior.

## Middleware: ValidatePostSize

Caps the size of incoming request bodies so oversized uploads cannot
pressure memory or disk. Mirrors the technique in
`gin-gonic/examples/upload-file/limit-bytes`: honor `Content-Length`
fast, and clamp the body reader for chunked transfers that lie about or
omit the length.

- Slot: `intake.postsize`, ordinal 8 (after CORS, so prefilter answers
  never pay the inspection; before any binding, so oversize bodies never
  reach decoders).
- Logical default: all HTTP requests bearing a body, with a default cap
  of 1 MiB for ordinary routes and an 8 MiB cap that designated upload
  routes opt into via a per-route configuration. Oversize responses are
  413 with the effective cap in the body.
- Required parameters: the global cap and the per-route override table.
  Both derive from app configuration; the demo's note-create route stays
  at the global cap, while a future file-upload route raises its own.
- Edge cases: a zero or absent `Content-Length` must not waive the check
  (chunked bodies get the clamped-reader path); the clamp surfaces EOF
  to readers as truncation, so decoder errors classify as 413, not 500;
  ranged or resumed uploads compute against the declared range, not the
  whole entity.
- Work: a body-clamping reader wrapper with truncation tests, the
  Content-Length fast path, and a per-route override plumbed through the
  route registration API.

## Middleware: ValidateRequestHeaders

Bounds the request header block against flooding and smuggling: a maximum
header count, a maximum length per value, a maximum aggregate size, and
a ban on CR/LF sequences smuggled into values.

- Slot: `intake.requestheaders`, ordinal 9, after the size gate.
- Logical default: all HTTP requests. Caps sized for browsers and mobile
  agents: 50 headers, 8 KiB per value, 32 KiB aggregate. Breaches answer
  431 (Request Header Fields Too Large) with the offending dimension.
- Required parameters: the three caps and the banned-class check, all
  with the browser-sized defaults above and all overridable per app.
- Edge cases: Go's http server already imposes some kernel-level limits,
  so the middleware is belt-and-braces for configurations that loosen
  them; measuring the aggregate size must use header bytes as transmitted
  (keys plus values), not rounded estimates. Responses must name the
  breached dimension so operators can distinguish floods from fat
  cookies.
- Work: constant table, measurement pass, and tests that construct
  violating headers through httptest.

## Middleware: TrimStrings

Removes leading and trailing whitespace (spaces, tabs, newlines) from
string-typed input values before handlers decode them, absorbing the
padding that paste and form tooling love to add.

- Slot: `intake.strings.trim`, ordinal 10, the last transform before
  state restoration, so every downstream binder sees cleaned values.
- Logical default: all routed input channels (query parameters, form
  fields, JSON body strings). Deliberately excluded: multipart file
  parts (binary content), values flagged as verbatim through a
  field-option, and anything after the body has been consumed by an
  earlier binder.
- Required parameters: none mandatory. Optional: an exclusion list of
  field names (passwords and API keys lead the list; stripping padding
  from a password the user chose is data corruption dressed up as
  kindness) and a recursion depth cap for nested structures.
- Edge cases: Unicode space forms trim through the unicode-aware
  trimmer, not a rune-space hack; the middleware transforms a copy of
  the parsed containers, leaving the raw body readable once for
  auditors; arrays of strings trim each element, maps walk to leaves.
- Work: transformer over the three input channels with golden tests
  (pad, tab, newline, Unicode pad, excluded field, deep nesting).

## Middleware: CORS (github.com/gin-contrib/cors)

Manages Cross-Origin Resource Sharing headers so external front ends
know whether a browser may talk to the API, and answers prefilter
(OPTIONS) exchanges without reaching application code. Consult
`.refs/md/cors.md` for the configuration surface.

- Slot: `api.cors`, ordinal 7, the first business lane, so prefilters
  settle before any heavier middleware runs.
- Logical default: all HTTP requests, restricted by default. The stock
  `cors.Default()` permits every origin, and the README warns that
  wildcard origins invalidate credentialed (cookie-carried) requests, so
  the Sheidan default starts with no origins granted (same-origin
  traffic is invisible to the middleware and always passes). Apps grant
  origins explicitly, with `AllowCredentials` paired to the grants.
- Required parameters: the origin list (empty by default), permitted
  methods (default: the REST set GET/HEAD/POST/PATCH/DELETE), permitted
  headers, the prefilter cache age (default one hour), and the
  credentials flag (false by default).
- Edge cases: granting wildcards while requesting credentials is a
  contradiction browsers refuse; the builder should reject that combo at
  compile time rather than at request time. Non-simple requests from
  same-origin pages never engage CORS, so the slot is transparent to
  the demo's own GopherJS client.
- Work: add the dependency, wire the slot with a config bag fed by app
  settings, and add tests for prefilter answering, origin reflection,
  and the wildcard-plus-credentials rejection.

## Demonstrations

- With `SHEIDAN_MAINTENANCE=1`, every route answers 503 with
  `Retry-After` while `/healthz` stays alive; clearing the env var
  restores traffic without a restart.
- A crafted 2 MiB POST to `POST /notes` answers 413; the same body to a
  hypothetical upload route (cap raised in config) proceeds.
- Fifty-five headers on one request answers 431; fifty-four pass.
- A POSTed note title arriving as `"  hello "` stores as `"hello"`; a
  password-shaped field with padding stores verbatim.
- A cross-origin OPTIONS prefilter from a foreign origin answers 204
  with the granted headers; a same-origin POST needs no CORS exchange.

## Done when

- Ordinals 3, 4, 7, 8, 9, 10 hold working implementations in the
  default chain (placeholders replaced), each honoring its defaults and
  its per-app configuration.
- The builder's `Configure` accepts each new slot's parameter bag, and
  the group exclusion list demonstrably silences any of them for one
  branch while neighboring branches keep behaving.
- The five demonstrations reproduce from a fresh clone.
- `go vet ./...`, `staticcheck ./...`, and `go test ./...` all pass.
