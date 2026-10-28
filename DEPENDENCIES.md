# Dependency maintenance

Reviewed on 2026-10-27 with Go 1.27.3.

## Go modules

The application now uses upstream modules instead of the five historical local
source copies. Merely increasing their `require` versions previously left the
old implementations active because of `replace` directives. The copies and their
nested manifests have been removed; Go's module checksum verification now covers
all five dependencies. The corresponding upstream license notices are preserved
in `third_party/licenses/`.

This upgrade moves goquery to v1.13.0, replaces cascadia v1.3.5 with the
performance-focused `github.com/soulteary/cascadia` v1.0.0 fork, Redis from v8.11.5 to
`github.com/redis/go-redis/v9` v9.23.0, HTML conversion from the local v1 copy to
`github.com/JohannesKaufmann/html-to-markdown/v2` v2.5.2, and Sonic from v1.15.4
to v1.15.5. Feeds v1.2.0 and cache2go commit `518229cd8021` are now the actual
upstream implementations rather than older local copies bearing those version
labels. Redis address/password/database environment variables and cache APIs
retain their existing names.

The cascadia fork is selected with a module-level `replace` directive because
goquery imports the original module path internally. This ensures both direct
and transitive cascadia users resolve to the same fork implementation.

The upstream feeds implementation emits JSON Feed 1.1, including the `authors`
array and the legacy `author` field. RSS and Atom entry IDs, content and links
are covered by regression tests. Markdown conversion has focused tests for
headings, Unicode text, emphasis, links, images, ordered lists and code.

Browser automation now uses chromedp v0.21.0 and generated cdproto v0.157.8.
The migration removes Rod and its pinned fetchup/got/goob/gson/leakless dependency
chain. Remote Chrome discovery, isolated browser contexts, request filtering,
cookies and deadline-bounded WebSocket connections remain covered by parser tests.

- Gin, gzip, Zap, chromedp, v8go and the selected `golang.org/x/*` versions were already
  at their latest stable releases when checked. v8go includes native V8 libraries;
  changing to another fork is a separate runtime/build migration, not a version bump.

Dependabot's 12 open alerts observed before this upgrade were all associated
with `golang.org/x/net` v0.22.0 in three deleted nested manifests. The application
already selected v0.61.0. Removing those manifests eliminates the stale dependency
sources; alert closure still depends on GitHub rescanning a pushed commit.

## Embedded browser libraries

jQuery is upgraded from 1.12.4 to 4.0.0 and Moment from 2.29.4 to 2.31.0. Assets
were extracted from the exact npm registry release packages after SHA-512 package
integrity verification. Source URLs and script checksums are recorded in
`internal/jssdk/js/dependencies.json`; license text is stored beside each script.

jQuery runs inside the SDK injection scope and calls `noConflict(true)` before
rule execution. Both `$` and `jQuery` remain available as private aliases for
rule compatibility. Browser integration tests verify extraction and repeated
injection without replacing a target page's existing `$` or `jQuery` globals.

The compiled Tailwind/daisyUI stylesheets and element-picker remain unchanged.
Updating those UI libraries requires a separate template/CSS migration and visual
verification; no newer release was substituted without that work.

## Validation

Use a remote-debuggable Chrome/Chromium endpoint and a test Redis instance:

```sh
RSS_SERVER=127.0.0.1:6379 RSS_TEST_BROWSER_URL=http://127.0.0.1:9222 RSS_TEST_BROWSER_REQUIRED=1 go test -race ./... -count=1
go vet ./...
go mod verify
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
go build -o /tmp/rssc .
```

`RSS_TEST_BROWSER_URL` may be an HTTP discovery endpoint or a complete browser
WebSocket debugger URL. Redis client compatibility is also tested against Redis 6, matching the existing macOS CI
configuration. Browser integration tests must execute, rather than skip, during
upgrade verification.

`govulncheck` can report GO-2026-5932 at module level because `x/crypto` contains
retired OpenPGP packages. The application uses SHA-3 through the HTTP validator,
not OpenPGP. Distinguish module-level database matches from imported packages and
reachable vulnerable symbols; do not label all database records as live findings.

## Source-built container

`docker/Dockerfile` now uses the published `golang:1.27.3-bookworm` builder and
`debian:bookworm-slim` runtime. The previous `1.27.3-bullseye` builder tag cannot
be pulled. Runtime certificates, timezone data and `libstdc++6` are explicit CGO
requirements. UPX is no longer needed; Go strips debug metadata at build time.
Module manifests are copied before application source so module downloads can
be cached now that local `replace` directives are gone.

## Verification results for this upgrade

- Full `go test -race ./... -count=1` passed with Chrome integration tests required and Redis 8 available.
- Cache tests passed separately against Redis 6.
- `go vet ./...`, `go mod verify` and `go mod tidy -diff` passed.
- `govulncheck` reported zero reachable vulnerabilities and zero vulnerabilities in imported packages; the unused OpenPGP module-level advisory remains unchanged.
- macOS ARM64 and AMD64 builds passed. Linux AMD64 also compiled successfully under QEMU and the resulting binary executed its help command.
- The source-built Linux ARM64 Docker image built successfully; its health endpoint and dynamic rule list both returned HTTP 200.
