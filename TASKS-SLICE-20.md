# Tasks Slice 20

Make the stack fast: response compression and the response-caching
layer. The cache layer occupies ordinal 20 of the chain as a
composite slot housing three collaborating pieces: the cache store
adapter around github.com/gin-contrib/cache, the cache tag index, and
the per-route policy engine that sets cache headers.

Carry-forward notes: the composite slot joins the chain at its
announced position; the stack builder remains the adjustment surface
for the global default stack from an end-user developer's Sheidan app
(disabling the composite slot disables all three pieces at once); and
any member is suppressible per route group via
`sheidan.Chain(base).Without("cache.response")`, the natural move for
personalized pages that must never be shared between visitors.

## Background: what the library actually offers

Consult `.refs/md/cache.md` and the source checkout under
`.refs/src/cache/`. The module centers on `cache.CachePage(store, ttl,
handler)`, a factory that wraps a specific handler function with
fetch-on-miss semantics. It is not a `gin.HandlerFunc` suitable for
`Use()`, and its stores (`persistence.NewInMemoryStore(ttl)`,
`NewRedisCacheWithURL(...)`, memcached variants) expose Get/Set/Delete/
Flush but no tag concepts and no per-route negotiation. The work in
this slice is therefore an adapter, not a bolt-on: Sheidan grows a
thin compatibility layer so the library's stores and serializers plug
into the middleware chain, and the gaps (tags, per-route policy,
conditional requests) get filled with small internal pieces.

## Component: gzip (github.com/gin-contrib/gzip)

Compresses response bodies for clients offering gzip, shrinking
transfer volume for the JSON API and the HTML shells alike.

- Slot: `resp.compress`, ordinal 5, the wrapper lane. Sitting outside
  the cache lane, it compresses what crosses the wire while the store
  keeps storing pristine uncompressed payloads; the division is
  deliberate and documented in the slot's doc comment.
- Logical default: engaged for all responses, activated only when the
  client advertises gzip in `Accept-Encoding` (the module's inherent
  behavior). Minimum-length threshold of 2048 bytes skips the ceremony
  for tiny replies, per the module's own sizing advice.
- Required parameters: compression level (default: the module's
  `DefaultCompression`), the min-length threshold, excluded
  extensions (bitmapped assets: png, jpg, svg, woff2, mp4, pdf), and
  excluded paths (nothing by default; the demo adds none, because its
  payloads are text).
- Edge cases: responses that flush incrementally (streams, SSE) pass
  through ungoverned, which is fortunate, since the compressor and
  incremental flushing fight over the writer; already-compressed
  content-types (image/jpeg, audio/*) are excluded by extension, not
  MIME sniffing, keeping the rule table inspectable; the module's
  server-push experiment stays unused (dead weight in modern browsers).
- Work: add the dependency, wire the slot with the parameter bag, and
  tests comparing raw versus compressed byte counts for a known JSON
  blob, plus the tiny-reply exemption.

## Component: the cache store adapter (github.com/gin-contrib/cache)

Wraps the library's `CachePage` fetch-on-miss semantics into a
middleware that the chain can carry, keyed by route plus varying
dimensions, with conditional-request support.

- Role inside the composite slot: the innermost duty. On a cacheable
  GET/HEAD, a hit answers directly with 200 (stored body) or 304
  (matching `If-None-Match`), sparing the handler entirely; a miss runs
  the handler chain, captures the winning response, stores it, and
  forwards it.
- Store default: the in-memory store (`NewInMemoryStore`) with a
  twelve-hour base TTL, living for the life of the process; the
  persistence interface stays exposed so an app swaps in Redis or
  memcached by configuration, without touching the chain.
- Key anatomy: route pattern plus the values of the varying dimensions
  (default: none, so one entry per route; personalized routes declare
  their variations). Entries carry a weak ETag derived from the stored
  bytes, powering the 304 path.
- Logical default: dormant. No route is cacheable until a policy
  (below) nominates it, so adopting the slot changes nothing until an
  app opts routes in. This keeps the demo's interactive note flow
  truthful by default while the demonstration routes prove the
  machinery.
- Edge cases: only 200 and 304 winners store; a 408 from the timeout
  lane, a 500, or an aborted request stores nothing, so stale poison
  never lodges. Capture must begin at the wrapper's writer boundary
  (ordinal 5's gzip sits outside, so the captured bytes are the
  uncompressed originals, and the stored ETag matches what a client
  saw before compression). Concurrent misses for the same key may both
  run the handler; the last writer's entry wins, which is benign for
  idempotent GETs and noted as such.
- Work: the adapter, the key builder, the ETag derivation, the
  conditional short-circuit, and tests for hit, miss, expire, 304
  roundtrip, and the no-poison rule.

## Component: CacheTag

Gives cache entries tag affiliations and purges by tag, turning
"invalidate every page featuring this dataset" from a flush-everything
blast into a surgical strike.

- Role: the indexing duty inside the composite slot. Policies (below)
  nominate tags per route; on store, the tag index records which keys
  bear which tags; `Purge(tag)` deletes the nominated keys and prunes
  the index. A convenience `PurgeAll` delegates to the store's Flush
  for the rare nuclear case.
- Memory discipline: the index maps tag to key-set; keys vanish on TTL
  expiry, sweeping their tag memberships in the same pass, so the
  index cannot outgrow the store.
- Logical default: inert until a policy names tags; the demo's sole
  tag, `feed.notes`, binds the notes list route, and deleting or
  creating a note purges it, keeping the list honest without a flush.
- Edge cases: purging under concurrency must lock per tag, not
  globally, or one purge stalls the fleet; a tag purged while a miss
  is in flight may repopulate from the dying handler, so the in-flight
  capture checks the tag's epoch before storing (epoch bumped on
  purge), a cheap two-field dance that closes the race.
- Work: the index with epochs, the purge API, and tests covering
  surgical purge, expiry sweeps, and the in-flight race.

## Component: CacheRouteResponse

Declares, per route, whether and how that route's responses cache:
enabled, TTL, varying dimensions, tag nominations, and the header
policy. This is the face an app talks to; the other two components are
its organs.

- Declaration: routes register policies through the route API
  (a `Cache(policy)` call on the route builder, or a policy table in
  app configuration for mass application to a group). The demo
  declares: the notes list JSON (ten-minute TTL, tag `feed.notes`,
  public), a heavy demo route (five-minute TTL, immutable-style), and
  the note-detail route (private, short TTL, varied by auth state).
- Header emission: the companion duty sets `Cache-Control`
  (public/private, max-age, stale-while-revalidate where the policy
  allows), `Expires`, the weak `ETag`, and `Surrogate-Control` hints
  for intermediary caches, computed from the same policy struct, so
  one declaration drives both store behavior and negotiator headers.
- Logical default: nothing cached, nothing hinted; the headers speak
  only where a policy exists, keeping unspecified routes honest.
- Edge cases: stale-while-revalidate serves the stale copy instantly
  and revalidates in the background, so the policy must name a
  revalidation grace distinct from the max-age; private and public
  policies never blend on one route (a compile-time assertion);
  authenticated variation forces `private` automatically, overriding
  a careless `public` nomination, because a public cache serving
  personalized bytes is a disclosure incident waiting for traffic.
- Work: the policy struct, the declaration API, the header emitter,
  and tests walking the demo's declarations through their expected
  header sets.

## Demonstrations

- Two successive GETs of the notes list: the first costs the handler
  (log line present), the second answers from the store (no handler
  line) with a matching weak ETag; a third GET carrying that ETag
  meets 304 with an empty body.
- Deleting a note purges `feed.notes`; the next list GET rebuilds the
  entry without the deleted row, while an untaged route's entry
  survives the purge untouched.
- `curl --compressed` on the heavy route shows the shrunk transfer;
  the same request without the header shows the raw size; a 200-byte
  reply stays uncompressed either way.
- Declaring `public` on a route with auth variation compiles with a
  correction to `private`, visible in the emitted headers.

## Done when

- The composite slot holds all three components; the store adapter
  speaks the library's persistence interface, so swapping in Redis is
  a configuration act.
- Only 200/304 winners ever store, and the in-flight epoch race is
  proven closed by test.
- The four demonstrations reproduce from a fresh clone, including the
  per-group suppression of the whole composite slot.
- `go vet ./...`, `staticcheck ./...`, and `go test ./...` all pass.
