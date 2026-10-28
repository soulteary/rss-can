# Readiness, metrics and request sharing

## Health endpoints

- `GET /_/health/` is a cheap liveness check and keeps the existing `{"message":"pong"}` response.
- `GET /_/ready/` probes JavaScript execution and the configured cache concurrently with a one-second overall budget. Redis gets at most 250 ms of that budget. A working memory fallback reports `ready: true`, `backend: "memory"`, `degraded: true`; a required Redis backend without fallback or an unavailable JS runtime returns HTTP 503. Disabling both cache backends is supported and reports `backend: "disabled"`.
- `GET /_/metrics/` exports Prometheus text format, using the same loopback/token restrictions as management endpoints. When `RSS_ADMIN_TOKEN` is configured, include `Authorization: Bearer <token>` even on loopback. Metrics contain no rule IDs, target URLs, cookies or credentials.

All metrics are process-local and reset on restart. Gauges describe current work; counters and duration histograms are cumulative. The histogram bounds in seconds are 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5 and 10, plus +Inf.

## Metrics and interpretation

- `rss_can_feed_requests_total`, `rss_can_feed_successes_total`, `rss_can_feed_failures_total`, `rss_can_feed_timeouts_total`, `rss_can_feed_duration_seconds`: feed HTTP outcomes, including rejected formats and unknown feeds. An overall generation deadline counts as both a failure and a timeout.
- `rss_can_fetch_requests_total`, `rss_can_fetch_failures_total`, `rss_can_fetch_duration_seconds`: actual SSR/detail HTTP document fetches, including status/body/decode failures. Cache hits and coalesced waiters do not count as upstream fetches. Browser navigation is tracked separately and is not included in this HTTP fetch failure rate.
- `rss_can_cache_hits_total`, `rss_can_cache_misses_total`, `rss_can_cache_errors_total`, `rss_can_cache_fallbacks_total`: backend reads, primary read errors, and fallback reads. A successful memory fallback can count as both a hit and a Redis error. Write/delete operations and readiness PINGs do not enter the read hit ratio.
- `rss_can_js_requests_total`, `rss_can_js_failures_total`, `rss_can_js_cancellations_total`, `rss_can_js_starts_total`: execution attempts (including readiness probes), failed calls, caller-context cancellations, and worker start attempts. A per-execution JS timeout counts as a failure. `rss_can_js_queued` and `rss_can_js_active` are gauges. `rss_can_js_queue_seconds` measures slot waiting; `rss_can_js_execution_seconds` includes any startup after acquisition, IPC and script execution.
- `rss_can_browser_requests_total`, `rss_can_browser_failures_total`, `rss_can_browser_queued`, `rss_can_browser_active`, `rss_can_browser_queue_seconds`, `rss_can_browser_duration_seconds`: browser tasks, queueing, occupied slots and duration through context cleanup. The concurrency limit remains four.
- `rss_can_config_cache_hits_total`, `rss_can_config_cache_misses_total`, `rss_can_config_shared_total`, `rss_can_fetch_shared_total`: configuration reuse and callers joining existing work.

Example PromQL for upstream document failure rate:

```promql
rate(rss_can_fetch_failures_total[5m]) / clamp_min(rate(rss_can_fetch_requests_total[5m]), 1e-9)
```

Cache read hit ratio:

```promql
rate(rss_can_cache_hits_total[5m]) / clamp_min(rate(rss_can_cache_hits_total[5m]) + rate(rss_can_cache_misses_total[5m]), 1e-9)
```

JS slot-wait p95 in seconds:

```promql
histogram_quantile(0.95, sum by (le) (rate(rss_can_js_queue_seconds_bucket[5m])))
```

## Sharing and configuration caching

When document caching is enabled, concurrent requests for the same URL, charset and expiration share in-flight work. Each caller keeps its own deadline. One caller leaving does not cancel others; the final caller leaving cancels and forgets the task. Shared tasks also have the configured server deadline. Failures are not cached. `DisableCache` or disabling both backends also disables document request sharing. Document cache keys isolate charset and expiration, so existing URL-only cache entries are not reused.

Set `RSS_CONFIG_CACHE_TTL=30` to opt into a 30-second configuration cache. The default is disabled to preserve rules whose configuration uses time, randomness or other per-request computation. Invalid/nonpositive values disable it; values above 3600 are capped at one hour. The process stores at most 256 entries and keys them by actual rule bytes, path, TTL and reload generation. Updated rule bytes change the key when rules are reloaded; reload also clears the cache. Failed evaluations are not stored. Concurrent misses share one evaluation; reload during evaluation cannot repopulate stale entries. The feed list also benefits from this cache.

Repeated identical date strings are converted once per generated feed. Conversions are never cached across requests, preserving relative-date behavior.

The implementation retains one Deno worker. Local fixture measurements establish the benefit of removing duplicate work; they are not a production capacity guarantee. Use queue histograms and active/waiting gauges under your actual rules before introducing more workers and their memory/process overhead. See [load measurements](benchmarks/feed-load.md).
