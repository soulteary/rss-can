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
   all four GoReleaser targets without publishing. Publication tooling tests run
   before platform builds, including real four-platform Go binaries, archive
   checksums, draft creation, interrupted upload recovery and public retries.
3. Create and push the annotated tag on that exact commit. The tag workflow,
   rather than a manual upload, builds and publishes its artifacts.

## Publication gates

GoReleaser v2.18.3 is fixed in `.github/workflows/build.yml`. Linux uses a pinned
Ubuntu 24.04 runner with CGO_ENABLED=0 for AMD64 and ARM64. macOS builds
both architectures. All executables are named `rssc`; their version, full source
commit and build timestamp are injected at link time.

GoReleaser only stages archives (`--skip=publish`). The three CI artifacts
`release-linux`, `release-darwin-amd64`, and `release-darwin-arm64` contain four
archives and three checksum files. Archives include rules, release notes,
documentation and third-party license notices. Standalone binaries require Deno
2.9.8 on PATH (or RSS_CAN_DENO); Docker images include the pinned Deno binary.
CI installs the same Deno version before running tests. Archives include
`docs/upgrading.md`; follow it for the v0.3.12-to-Deno transition. The release
tooling gate checks Docker and CI Deno pins against `.deno-version`.

The archive gate validates checksums, runtime files and all four binary identities
before any image is pushed. Native AMD64 and ARM64 runners then build the source
Dockerfile, inject the same tag
and source commit, and push architecture tags to Docker Hub. The
manifest job combines the platforms into the version tag, pulls both platforms
from Docker Hub, checks health and rule-list responses, and verifies
embedded version/commit metadata and OCI labels. Only verified manifests are
promoted to `latest`. Publishing jobs refuse to overwrite an already published
GitHub Release.

Before any builds on a version tag, the preflight gate requires stable tag
syntax, nonempty release notes and matching Docker/go.mod Go toolchains pinned
to a full patch version. This
prevents discovering missing notes only after Docker publication.

After images pass, `scripts/verify_release_assets.py` verifies all seven assets,
SHA-256 checksums, archive contents and each binary's Go/platform/tag/commit
metadata. The shared `scripts/publish_release.py` creates a GitHub draft, uploads
missing or invalid draft assets and compares GitHub asset digests before
publication. API failures abort instead of being treated as an absent release.
It rechecks the remote source tag and draft identity before upload and verifies
the published asset set afterward. This avoids incomplete public releases and remains
compatible with immutable-release settings. Retry failed jobs only while a
release remains unpublished; do not rerun Docker builds for an existing public
version. The publication-only completion workflow can recheck an identical public
Release without uploading, editing or replacing assets.

The existing report workflow runs on the tag. A final dispatch reuses a completed
successful report if available, preserving the report writeback rules described
in `.github/GO-REPORTS.md`.

## Post-release verification

Download all assets into an otherwise empty directory and run the verifier from
the current default branch against the immutable release source. Historical v8go archives require
`--cgo-enabled 1` (also supported by publish_release.py); completion automation
reads this setting from the original source build configuration:

```sh
git show v0.3.12:go.mod > /tmp/release.go.mod
python3 scripts/verify_release_assets.py /tmp/release-assets v0.3.12 "$(git rev-parse v0.3.12^{commit})" --go-mod /tmp/release.go.mod --cgo-enabled 1
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

Run `complete-release.yml` on the default branch, providing all three explicit
inputs: `release_tag`, the full `source_sha`, and the original tag-triggered
`build_run_id`. There are no version-specific defaults. For example:

```sh
gh workflow run complete-release.yml --ref main \
  -f release_tag=v0.3.12 \
  -f source_sha=b42afa44bac8d8b3e02118cec8874588d555a43e \
  -f build_run_id=40525576257
```

The audit requires the tag, source and original build to agree, and every binary
build and image smoke test to have succeeded. It accepts failures only in archive
verification or Release upload/publication; build failures need the original
workflow repaired/rerun. A successful original run can also be rechecked.

The completion workflow downloads the original artifacts and takes release notes
and the expected Go toolchain from the original source commit. It shares the tag
build's concurrency group and the normal publication helper. It never moves tags
or rebuilds binaries/images. Draft uploads resume without replacing valid files;
public Releases must match exactly and are only read. After verification it
requests the same reusable quality reports as normal publication.
