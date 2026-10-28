# Changelog

## Unreleased

- Reject incomplete feeds when configured detail links, upstream requests or selectors fail; keep inline metadata overrides and empty-element content fallback.
- Apply rule cookies/proxies to SSR listings, pagination and details, scope cookies to the original host, and isolate document caches by request options.
- Apply proxies to isolated remote Chrome contexts and include the effective global proxy in browser cache identity.
- Add ordered, cancelable detail extraction (4 per page, 16 process-wide) and an opt-in Deno worker pool (`RSS_JS_WORKERS`, default 1, maximum 16).
- Resolve a portable sibling Deno executable, package a bilingual upgrade guide, remove obsolete Rod help, and check runtime pins in release tooling.
- Derive mutable startup configuration from existing default constants to avoid conflicting defaults.

- Use a single persistent Deno worker by default, with fresh request contexts and timeout recovery.
- Build release binaries without CGO; include Deno 2.9.8 in Docker images and CI. Standalone installations require Deno on PATH or RSS_CAN_DENO.
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
