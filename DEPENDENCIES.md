# Dependency maintenance

The current dependency graph is defined by `go.mod` and `go.sum`; release
builds use Go 1.27.3, CGO disabled, and Deno 2.9.8. Historical migration evidence
is preserved in Git history and `docs/benchmarks/`, rather than mixed with the
current installation requirements.

## Runtime and deployment

Deno is the sole JavaScript backend. `.deno-version` records the runtime pin;
`scripts/check_runtime_versions.py` checks Docker, CI and installation docs
against it before platform builds. Update all pins together and run the worker,
sandbox and rule configuration regressions. Standalone installations supply
Deno through `RSS_CAN_DENO`, a sibling executable or PATH. Docker includes it.
See [upgrading](docs/upgrading.md) and [runtime lifecycle](docs/deno-prototype.md).

Browser automation uses chromedp and cdproto with a separately supplied
Chrome/Chromium debugging endpoint. Changes must preserve isolated contexts,
cookies, per-context proxies, inspector destination restrictions and cleanup.
The source-built container uses a Debian slim runtime with certificates,
timezone data and libstdc++6 for Deno.

## Go dependencies and maintained forks

Application imports use module paths directly; there are no historical local
`replace` copies. `go.sum` verifies downloaded module contents. The project uses
maintained forks of goquery, cascadia, feeds and html-to-markdown, plus cache-kit
and redis-kit. Fork upgrades require compatibility tests and increase the set
of upstream releases that the maintainer must track.

- goquery/cascadia: selectors and SSR extraction. See [performance notes](docs/goquery-performance.md).
- feeds: RSS, Atom and JSON Feed output, including IDs, authors, timestamps and namespaces. See [compatibility/performance notes](docs/feeds-performance.md).
- html-to-markdown: Unicode, headings, emphasis, links, images, lists and code. See [performance notes](docs/html-to-markdown-performance.md).
- cache-kit/redis-kit and go-redis: TTLs, misses versus empty values, cancellation, fallback and connection lifecycle.
- Fiber: routes, status codes, timeouts and management authorization.

Preserve third-party notices under `third_party/licenses/`. Do not interpret a
module-level vulnerability database match as proof of an application-reachable
vulnerability; examine imported packages and reachable symbols with a fresh scan.

## Embedded JavaScript and UI

The SDK embeds jQuery 4.0.0 and Moment 2.31.0. Package provenance and script
checksums are recorded in `internal/jssdk/js/dependencies.json`; adjacent LICENSE
files must accompany archives and images. Browser jQuery uses private aliases
and must not replace a target page's globals. Go statement coverage does not
measure the embedded JavaScript; keep sandbox and real-browser tests.

Compiled Tailwind/daisyUI assets and the element picker require separate visual
verification when updated. Do not replace generated assets without their build
and compatibility checks.

## Upgrade validation

Use the toolchain from `go.mod`, pinned Deno, isolated Redis and an isolated
Chrome debugging endpoint. See [testing](docs/testing.md) for coverage budgets.

```sh
python3 scripts/check_runtime_versions.py
python3 -m unittest discover -s scripts -p 'test_*.py'
RSS_SERVER=127.0.0.1:6379 RSS_TEST_BROWSER_URL=http://127.0.0.1:9222 RSS_TEST_BROWSER_REQUIRED=1 go test -race ./... -count=1
go vet ./...
go mod verify
go mod tidy -diff
CGO_ENABLED=0 go build -o /tmp/rssc .
```

Run a current vulnerability scan during dependency upgrades, then verify actual
release archives and source-built images using `RELEASING.md`. Results from a
previous revision are not validation of a new dependency graph.
