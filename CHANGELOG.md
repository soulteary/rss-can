# Changelog

## v0.5.1 — 2026-11-13

See the [bilingual release notes](docs/releases/v0.5.1.md). No configuration or data migration is required from v0.5.0.

- Require Go 1.27.4 in `go.mod`, the Docker builder image and the CodeQL workflow. Go 1.27.4 contains security fixes to cgo, the go command, the compiler, the linker and `net/http` (GO-2026-6859).
- Update cache-kit/v2 to v2.1.2 and redis-kit to v1.8.2.
- Update chromedp to v0.23.0 and cdproto to v0.159.2, used by browser (CSR) rules.
- Add an architecture diagram to both READMEs, with its source under `docs/images/` and `scripts/render_architecture.py`. Refresh Go quality reports.
- Retain Deno 2.9.8, the archive layout, configuration variables and stored data formats.

[Compare v0.5.0…v0.5.1](https://github.com/soulteary/rss-can/compare/v0.5.0...v0.5.1).

## v0.5.0 — 2026-11-04

See the [bilingual release notes](docs/releases/v0.5.0.md) and [v0.4.0 upgrade steps](docs/upgrading.md#upgrade-v050).

- Add optional Stargate/Warden team identity, explicit source ACLs, bounded persistent authorization storage and revocable read-only reader credentials; authenticate every private read before serving cached content or 304 responses.
- Add a public-website source wizard with preview, revision checks, rollback, deletion and manual refresh, plus read-only `rssc doctor` diagnostics and a guided private-backend Stargate deployment.
- Enforce public SSR fetch policy in team mode, validate redirects and resolved destinations, and share 64-request/64-MiB upstream budgets across each generation. Standalone mode retains trusted operator-owned rules.
- Add bounded authorized feed snapshots, conditional responses and per-owner scheduling. Default freshness is 30 seconds with up to 3600 additional stale seconds; `RSS_SNAPSHOT_TTL=0` disables snapshots.
- Make manual refresh bypass and replace page caches, isolate managed source revisions, prevent older concurrent writes from overwriting newer successful fetches, and release preparation permits after panics.
- Sanitize HTML in every feed format, remove active/resource-loading markup, replace images with alternative text and preserve plain-text JSON summaries without inserting spaces inside inline text.
- Require POST plus an explicit action header for administrative mutations, redact sensitive request details, run containers as UID/GID 65532, and isolate Deno worker caches in private temporary directories.
- Tighten browser/selector resource limits, startup validation, source ownership checks and fail-closed proxy configuration; retain private legacy standalone operation and pinned Deno 2.9.8 packaging.

[Compare v0.4.0…v0.5.0](https://github.com/soulteary/rss-can/compare/v0.4.0...v0.5.0).

## v0.4.0 — 2026-11-02

See the [bilingual release notes](docs/releases/v0.4.0.md) and [upgrade guide](docs/upgrading.md).

- Ship checksum-pinned Deno 2.9.8 with all four Linux/macOS archives; validate executable permissions and start native archives without an external runtime.
- Adopt Fiber/chromedp, bounded application lifecycle and resource admission, immutable rule snapshots, feed telemetry and admin hardening.
- Preserve feed field, date/category, pagination and URL provenance contracts; reject incomplete detail extraction.
- Refresh cache-kit v2.1.1 and redis-kit v1.8.1 and optimized parser/Markdown/feed dependencies.

- Reject incomplete feeds when configured detail links, upstream requests or selectors fail; keep inline metadata overrides and empty-element content fallback.
- Apply rule cookies/proxies to SSR listings, pagination and details, scope cookies to the original host, and isolate document caches by request options.
- Apply proxies to isolated remote Chrome contexts and include the effective global proxy in browser cache identity.
- Add ordered, cancelable detail extraction (4 per page, 16 process-wide) and an opt-in Deno worker pool (`RSS_JS_WORKERS`, default 1, maximum 16).
- Resolve a portable sibling Deno executable, package a bilingual upgrade guide, remove obsolete Rod help, and check runtime pins in release tooling.
- Derive mutable startup configuration from existing default constants to avoid conflicting defaults.

- Use a single persistent Deno worker by default, with fresh request contexts and timeout recovery.
- Build release binaries without CGO; include Deno 2.9.8 in Docker images and CI. Portable archives include Deno beside rssc; source builds need Deno on PATH or RSS_CAN_DENO.
- Remove the legacy v8go backend and Go module dependency; Deno is the sole JavaScript runtime.

## v0.3.12 — 2026-10-27

See the [bilingual release notes](docs/releases/v0.3.12.md) for changes, upgrade requirements and container commands.

- Fix feed/cache parsing, configuration initialization, hot reloads, upstream error handling and resource cleanup.
- Upgrade upstream Go modules and embedded JavaScript libraries; preserve licenses and remove stale nested manifests.
- Restrict management access and inspector destinations; retain private browser-rule jQuery aliases.
- Package rules and licenses with four binary archives and publish verified dual-architecture Docker Hub images.

[Compare v0.3.11…v0.3.12](https://github.com/soulteary/rss-can/compare/v0.3.11...v0.3.12).

## Earlier releases

See [GitHub Releases](https://github.com/soulteary/rss-can/releases) for v0.3.11 and older versions.
