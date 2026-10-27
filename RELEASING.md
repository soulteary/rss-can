# Releasing RSS Can

Stable releases use an annotated `vMAJOR.MINOR.PATCH` tag on a tested source
commit. Do not move an existing published tag or rebuild its images in place.
The v0.3.11 recovery workflow is deliberately restricted to its audited source
and is not the normal release path.

## Prepare

1. Update the version in both READMEs and all RSS Can Compose examples. Add
   bilingual notes at `docs/releases/<tag>.md` and an entry in `CHANGELOG.md`.
2. Commit the source, documentation and release configuration. Push `main` and
   wait for Coverage, CodeQL and Build Artifacts to pass. Branch builds validate
   all four GoReleaser targets without publishing.
3. Create and push the annotated tag on that exact commit. The tag workflow,
   rather than a manual upload, builds and publishes its artifacts.

## Publication gates

GoReleaser v2.18.3 is fixed in `.github/workflows/build.yml`. Linux uses a pinned
Ubuntu 24.04 runner with explicit AMD64 and ARM64 CGO compilers. macOS builds
both architectures. All executables are named `rssc`; their version, full source
commit and build timestamp are injected at link time.

GoReleaser only stages archives (`--skip=publish`). The three CI artifacts
`release-linux`, `release-darwin-amd64`, and `release-darwin-arm64` contain four
archives and three checksum files. Archives include rules, release notes,
documentation and third-party license notices.

Native AMD64 and ARM64 runners build the source Dockerfile, inject the same tag
and source commit, and push architecture tags to Docker Hub. The
manifest job combines the platforms into the version tag, pulls both platforms
from Docker Hub, checks health and rule-list responses, and verifies
embedded version/commit metadata and OCI labels. Only verified manifests are
promoted to `latest`. Publishing jobs refuse to overwrite an already published
GitHub Release.

After images pass, `scripts/verify_release_assets.py` verifies all seven assets,
SHA-256 checksums, archive contents and each binary's Go/platform/tag/commit
metadata. A GitHub draft is created, populated and compared with GitHub's asset
digests before publication. This avoids incomplete public releases and remains
compatible with immutable-release settings. Retry failed jobs only while a
release remains unpublished; do not rerun publishing for an existing public
version.

The existing report workflow runs on the tag. A final dispatch reuses a completed
successful report if available, preserving the report writeback rules described
in `.github/GO-REPORTS.md`.

## Post-release verification

Download all assets into an otherwise empty directory and run the verifier from
the current default branch against the immutable release source:

```sh
python3 scripts/verify_release_assets.py /tmp/release-assets v0.3.12 "$(git rev-parse v0.3.12^{commit})"
```

Confirm Docker Hub version tags contain `linux/amd64` and `linux/arm64`,
and check the corresponding `latest` tags. Pull and run the released images,
not just local builds. Published image names are:

- `soulteary/rss-can:<tag>` and `soulteary/rss-can:latest`

Archives and image registries are independent distributions from the same
source commit; binaries inside the Debian-built images need not be byte-identical
to the standalone Ubuntu/macOS archives. Each distribution is verified against
its recorded checksums and source identity.

## Completing an interrupted publication

`complete-release.yml` is a manual, audited completion path. It verifies the
original tag, source commit, tag-triggered Build Artifacts run, all binary/image
build jobs, and the exact failure step before downloading the original artifacts.
It uses the corrected verifier from the current workflow source; it does not move
the release tag or rebuild any archive. Published assets are checksum-verified
without replacement. This handles the Go string-quoted linker metadata validation
failure encountered during v0.3.12 publication.
