# HTTP feed load and coverage measurements

Measured on 2026-10-28, macOS arm64, Go 1.27.3 and Deno 2.9.8. The before binary was built from `082355bc76b340c7005220c356874663b332c447`; the after binary includes request sharing, per-feed date deduplication, optional config caching and telemetry. These are short local fixture measurements, not a production capacity or long-running memory guarantee.

## Method

`scripts/bench-feed/load.py` launches the actual compiled service on a loopback port against a deterministic local HTTP upstream with a 25 ms delay. Each page contains five RSS entries. Warm scenarios preload documents; cold scenarios start with empty caches. The date scenario repeats the same date string across all five entries. There are six scenarios: warm concurrency 1/8/32, cold concurrency 32, dates concurrency 16, and seven rules at concurrency 16.

Each trial sends 512 requests with a closed-loop Python HTTP client. Three trials alternate before, after with config caching disabled, and after with `RSS_CONFIG_CACHE_TTL=30`, using the same one-worker runtime and memory document cache. Reported throughput and p95 are the medians of the three trials, not a pooled percentile. The client opens new HTTP connections, so it contributes overhead. All 27,648 measured requests succeeded.

## Findings

- Warm, concurrency 32: before 3,813 requests/s and p95 9.91 ms; default after 4,097 requests/s and p95 9.16 ms; opt-in config cache 7,294 requests/s and p95 6.54 ms.
- Cold, concurrency 32: before 32 upstream fetches per trial; both after modes made one. End-to-end p95 stayed around 31 ms because the shared upstream still takes 25 ms. Default throughput was 3,261 requests/s versus 3,362 before; this scenario demonstrates reduced upstream work, not lower latency.
- Repeated dates, concurrency 16: before 339 requests/s and p95 57.01 ms; default after 1,208 requests/s and p95 15.73 ms; opt-in config cache 1,503 requests/s and p95 14.46 ms. This result specifically depends on repeated date strings; distinct dates still require separate JS calls.
- Seven warm rules, concurrency 16: before 3,188 requests/s and p95 8.61 ms; default after 3,510 requests/s and p95 7.07 ms; opt-in config cache 5,570 requests/s and p95 4.90 ms.

These results support reducing duplicate work before adding processes. Config caching is opt-in because arbitrary JS configuration can be dynamic. The worker remains single-process; use the new queue metrics with real rules and a target load/SLO to decide whether a worker pool is justified.

Raw individual trials, numeric metrics snapshots and all scenarios: [2026-10-28-feed-load.json](2026-10-28-feed-load.json).

## Coverage results

Coverage counts Go statements, not branches or embedded JS. The Go suite combines unit, local fixture and integration tests. Profiles use standard per-package instrumentation, `-race`, atomic counters and `-count=1`; runtime totals are weighted statement counts for `internal/`, and all-package totals additionally include the main entry point and benchmark CLI, which have no direct tests.

- Default baseline, live browser skipped: 65.81% runtime, 61.11% all packages; 115 passing top-level tests and one skipped top-level test.
- Baseline with real Chrome required: 68.73% runtime, 63.83% all packages.
- Final default (browser skipped): 83.19% runtime, 78.09% all packages. Final browser-required results, exact package statement counts and test outcomes are recorded in [2026-10-28-coverage.json](2026-10-28-coverage.json). With real Chrome required, runtime coverage is 87.06% and all-package coverage is 81.72%, with no failing or skipped tests.
- Core final packages: rule 95.06%, network 80.00%, parser 77.83%, server 84.03%, generator 88.89%, jssdk 84.77%; request sharing 97.14% and telemetry 100%.

The new CI gate requires 80% runtime coverage plus individual package floors. The coverage workflow starts isolated Chrome and requires browser integration instead of silently skipping it. Pure coalescing, metric, configuration and readiness-policy unit tests complement actual Deno, Redis, HTTP and browser regressions. The coverage increase includes newly tested existing paths, not only the added packages.

The largest remaining gaps are browser inspector fulfillment and selected transport/proxy/certificate failures, and process entry/shutdown orchestration. This is not a claim of complete behavioral coverage. See [testing](../testing.md) for commands and policy, and [observability](../observability.md) for metric definitions.

## Reproduce a load scenario

```sh
CGO_ENABLED=0 go build -o /tmp/rss-can-load .
RSS_CAN_DENO=/path/to/deno python3 scripts/bench-feed/load.py --binary /tmp/rss-can-load --requests 512 --concurrency 32 --cold
RSS_CAN_DENO=/path/to/deno python3 scripts/bench-feed/load.py --binary /tmp/rss-can-load --requests 512 --concurrency 32 --config-cache-ttl 30
```
