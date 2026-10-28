# Changelog

## Unreleased

- Use a single persistent Deno worker by default, with fresh request contexts and timeout recovery.
- Build release binaries without CGO; include Deno 2.9.8 in Docker images and CI. Standalone installations require Deno on PATH or RSS_CAN_DENO.
- Retain the previous execution backend through an explicit `-tags v8go` build.

## v0.3.12 — 2026-10-27

See the [bilingual release notes](docs/releases/v0.3.12.md) for changes, upgrade requirements and container commands.

- Fix feed/cache parsing, configuration initialization, hot reloads, upstream error handling and resource cleanup.
- Upgrade upstream Go modules and embedded JavaScript libraries; preserve licenses and remove stale nested manifests.
- Restrict management access and inspector destinations; retain private browser-rule jQuery aliases.
- Package rules and licenses with four binary archives and publish verified dual-architecture Docker Hub images.

[Compare v0.3.11…v0.3.12](https://github.com/soulteary/rss-can/compare/v0.3.11...v0.3.12).

## Earlier releases

See [GitHub Releases](https://github.com/soulteary/rss-can/releases) for v0.3.11 and older versions.
