# Testing and statement coverage

The Go suite mixes pure unit tests, local HTTP/TCP fixtures, Deno process tests, Redis integration tests and Chrome integration tests. A single coverage percentage is therefore **combined Go statement coverage**, not pure-unit coverage or branch coverage. It does not measure embedded JavaScript statements. Cross-package calls are not credited to an imported package unless that package is instrumented with `-coverpkg`; the default report uses Go's standard per-package instrumentation.

## Required dependencies and full regression

Install the Go version from `go.mod`, Deno 2.9.8, Redis, and Chrome/Chromium. Run an isolated Chrome with a dedicated temporary profile and a loopback debugging port; this does not use your personal browser profile. Set `RSS_TEST_BROWSER_URL` to that endpoint. `RSS_TEST_BROWSER_REQUIRED=1` prevents silently skipping browser regressions.

```sh
RSS_SERVER=127.0.0.1:6379 \
RSS_CAN_DENO=/path/to/deno \
RSS_TEST_BROWSER_URL=http://127.0.0.1:9222 \
RSS_TEST_BROWSER_REQUIRED=1 \
go test -race ./... -count=1 -timeout=90s -covermode=atomic -coverprofile=coverage.out

go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html
python3 scripts/check_coverage.py coverage.out
python3 -m unittest discover -s scripts -p 'test_*.py'
```

The browser test currently consumes `RSS_TEST_BROWSER_URL`; it does not discover or launch Chrome from `RSS_TEST_CHROME`. Without a browser URL the live browser test is skipped unless it is required. The coverage workflow starts isolated Chrome and requires this test, while providing Redis and Deno.

Pure unit tests for request coalescing and telemetry can run without Redis, Deno or Chrome:

```sh
go test -race ./internal/coalesce ./internal/telemetry ./internal/define ./internal/fn ./internal/cmd ./internal/logger
```

## Coverage budgets

`scripts/check_coverage.py` requires at least **80% weighted runtime statement coverage** across `internal/`. It excludes the executable entry point and developer benchmark CLI from that one budget, while `go test ./...` still tests/builds all packages and the full Go profile retains them. An empty profile or a missing required runtime package fails validation. Per-package minimums prevent highly covered utility packages from hiding a weak core module:

- 95%: cmd, coalesce, define, fn, logger, telemetry.
- 90%: rule.
- 85%: generator.
- 80%: cacher, jssdk.
- 75%: network, server.
- 70%: parser.

These are regression floors, not completeness claims. Browser interception failure branches, remote certificate/proxy setup and process entry/shutdown still need dedicated scenarios. Prefer tests that assert results, cancellation, isolation and cleanup over tests written only to raise a number. The gate has its own Python regression tests for statement weighting, duplicate blocks, invalid evidence and missing packages.

The checked-in `.github/go-test-report.md` and badges can refer to older generated evidence; use a freshly generated profile or the CI artifact for the revision being reviewed. See [the measured baseline and current profile](benchmarks/feed-load.md) for this improvement's results.
