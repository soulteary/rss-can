# Authorized feed snapshots

v0.5.0 and later, and current source builds, keep a bounded, process-local cache of complete feed representations.
The HTTP authentication/authorization middleware still runs on **every** request,
including requests carrying `If-None-Match`. A snapshot is content, never a cached
permission decision. Do not expose the feed handler outside that middleware.

## Configuration

- `RSS_SNAPSHOT_TTL=30`: freshness in seconds (0 disables snapshots; maximum 3600).
- `RSS_SNAPSHOT_MAX_STALE=3600`: additional seconds in which the last successful
  snapshot may be returned while a refresh runs (0 requires a successful refresh
  immediately after freshness expires; maximum 86400).
- An explicit rule `Expire` shorter than the global freshness setting wins.
- A rule's `DisableCache` bypasses snapshots and request coalescing, preserving
  its independent, live-fetch behavior. It does not emit snapshot validators.

Direct embedders opt in with `feedservice.NewSnapshots` and must call `Close`.
`application.App` owns this lifecycle automatically. Snapshot errors and initial
generation are bounded by the existing complete-generation timeout. No work is
scheduled after closing, and close waits for active refreshes before dependencies
are released.

The store permits at most 256 keys and 64 MiB of retained body bytes by default,
and never evicts in-flight work. Empty/error entries are bounded by the key limit.
It is deliberately separate from Redis: cache eviction, connection failures or
flushes cannot change the current authorization state.

## Identity and refresh behavior

Keys hash the source ID, output format, captured rule bytes, evaluated complete
configuration and the authenticated access scope/epoch. This separates different
owners, credential changes, rule changes, formats and authorization changes. The
key does not contain a raw download token. Existing in-flight work can publish only
to the version it captured, never to a newer version's entry.

A fresh request reads the saved representation. A stale request returns the saved
representation and starts at most one refresh. A request without an eligible saved
representation waits for a single shared generation. Canceling one waiting reader
does not cancel other readers; canceling the last cold reader stops the work.
Failures preserve the previous value until the configured stale deadline and use
an exponential retry backoff starting at approximately five seconds, with stable
per-key jitter and a capped exponent. After that deadline, errors are returned rather than
presenting old content as current. Background refresh is demand-driven: unused
feeds do not continue fetching after readers stop requesting them.

Refresh work uses a bounded round-robin owner queue: eight tasks
globally, two active tasks per owner, 32 queued globally and eight queued per owner.
Configuration resolution uses a separate four-task queue so slow outbound work
does not consume the slots needed to read an existing snapshot.
Private owners are grouped by stable subject rather than by download token; public
sources are independently grouped. Limits apply within one application process,
not across multiple replicas. No claim of distributed rate limiting is made.

The serializer reuses the previous feed-level creation timestamp when checking an
updated source. If representation bytes are unchanged, `ETag` and `Last-Modified`
remain unchanged. Changed bytes receive a new creation timestamp. Relative item
dates retain the existing parser semantics and may themselves change the output.

## HTTP contract

Successful responses use `Cache-Control: private, no-cache` unless an upstream
middleware already specified a stricter policy. Intermediaries must not share
private bodies. A weak `ETag` hashes the exact uncompressed representation so it
can safely be revalidated across HTTP compression variants.
`If-None-Match` has priority over `If-Modified-Since`, supports weak comparison
for GET and returns a bodyless 304 after authorization. Last-Modified has HTTP's
one-second precision; ETag is preferable for rapid changes. Stale responses also
carry `Warning: 110 - "Response is stale"`. Per-owner queue limits use 429;
global capacity failures use 503. Both include `Retry-After: 5`.

## Verification

Snapshot tests cover scope/epoch/configuration isolation, single flight, independent
reader cancellation, last-reader cleanup, stable validators, changed output,
refresh failures, backoff, stale deadlines, capacity and shutdown. HTTP tests cover
authorization before 304, conditional-header precedence and private cache policy.
Fair-admission tests cover owner flooding, round-robin progress and cancellation.

Run the focused suite with `go test -race ./internal/feedservice ./internal/server
./internal/application ./internal/generator ./internal/telemetry`, providing the
documented Redis/Deno/browser fixtures for existing integration tests. Measure
warm and cold fixture generation with `go test ./internal/feedservice -run '^$'
-bench 'BenchmarkSnapshot(Warm|Cold)$' -benchmem`. These synthetic benchmarks
measure service overhead; they are not upstream network performance or a product
latency guarantee.

On 2026-11-03, the fixture benchmark on Apple M5 / macOS arm64 with
`GOMAXPROCS=2` (three one-second samples) measured warm reads at 17.9–27.3 µs/op
(median 20.0 µs, 3,793 B/op, 17 allocations) and cold reads at 126.0–136.8 µs/op
(median 126.3 µs, approximately 15,853 B/op, 88 allocations). These results include
configuration fingerprinting but no network or browser work; concurrent builds
were active on the host. Reproduce on deployment hardware before sizing capacity.

### Explicit refresh and document caches

An authorized manual refresh bypasses upstream document cache reads and replaces
successful entries under the existing keys. Later ordinary reads and snapshot
regeneration therefore use the refreshed page. Failed refreshes preserve the
previous document cache and last successful representation. Concurrent older
fetches cannot overwrite a newer successful refresh. A manual request arriving
during an ordinary generation waits for it, then performs an explicit refresh;
it does not report an ordinary cached page as freshly fetched. Admission, retry
backoff, request deadlines and shutdown cancellation still apply.

Managed source document caches also include the captured source revision. Editing
or rolling back a source cannot reuse a page from a previous revision. Revision
hashes contain no source content or credentials. Legacy rule keys are unchanged.
`DisableCache` continues to disable both cache reads and writes.
