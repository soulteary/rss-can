#!/usr/bin/env python3
"""Verify all release archives before publication or after downloading a Release."""
import argparse
import hashlib
import json
import pathlib
import re
import subprocess
import tarfile
import tempfile
import zipfile


# The v0.5.0 user workflows must remain usable after extracting an archive.
# Keep this list explicit: broad docs/example globs can package local secrets.
TEAM_RELEASE_FILES = frozenset({
    '.github/coverage.svg',
    '.github/goreportcard.svg',
    '.github/go-test-report.md',
    '.github/goreportcard-report.md',
    '.github/images/hp.jpg',
    '.github/images/feeds.jpg',
    'docs/secure-deployment.md',
    'docs/stargate-access.md',
    'docs/stargate-setup.md',
    'docs/fetch-policy.md',
    'docs/feed-content-safety.md',
    'docs/feed-snapshots.md',
    'docs/observability.md',
    'docs/testing.md',
    'docs/feeds-performance.md',
    'docs/goquery-performance.md',
    'docs/html-to-markdown-performance.md',
    'docs/benchmarks/feed-load.md',
    'docs/benchmarks/README.md',
    'docs/benchmarks/deno-v8go.md',
    'docs/benchmarks/deno-worker.md',
    'docs/benchmarks/2026-10-28-coverage.json',
    'docs/benchmarks/2026-10-28-deno-v8go.json',
    'docs/benchmarks/2026-10-28-deno-worker-soak.json',
    'docs/benchmarks/2026-10-28-deno-worker.json',
    'docs/benchmarks/2026-10-28-feed-load.json',
    'docs/benchmarks/2026-10-28-v8go-soak.json',
    'example/README.md',
    'example/1.standalone/docker-compose.yml',
    'example/2.with-redis-cache/docker-compose.yml',
    'example/3.enable-csr-render/docker-compose.yml',
    'example/4.with-stargate/README.md',
    'example/4.with-stargate/compose.yml',
    'example/4.with-stargate/nginx.conf.template',
    'example/4.with-stargate/setup.py',
    'example/4.with-stargate/verify.py',
    'example/4.with-stargate/rss_operator.py',
    'example/4.with-stargate/.gitignore',
})


def private_release_path(name):
    parts = pathlib.PurePosixPath(name).parts
    return any(part == ".runtime" or part == ".env" or
               (part.startswith(".env.") and part != ".env.example")
               for part in parts) or "stargate-integration-plan.zh-CN.md" in parts


def archive_names(tag):
    if not re.fullmatch(r"v\d+\.\d+\.\d+", tag):
        raise ValueError("expected a stable version tag such as v0.3.12")
    version = tag[1:]
    return {
        f"RSS-Can_{version}_linux_x86_64.tar.gz": ("linux", "amd64", "checksums.txt"),
        f"RSS-Can_{version}_linux_arm64.tar.gz": ("linux", "arm64", "checksums.txt"),
        **{f"rssc_{version}_darwin_{arch}.zip": ("darwin", arch, f"rssc_{version}_darwin_{arch}_checksums.txt")
           for arch in ("amd64", "arm64")},
    }


def go_version_from_mod(path):
    """Match setup-go's go-version-file selection of go/toolchain directives."""
    versions = re.findall(r"^(?:go\s+|toolchain\s+go)([0-9]+\.[0-9]+(?:\.[0-9]+)?)(?:\s|$)", pathlib.Path(path).read_text(), re.MULTILINE)
    if not versions:
        raise ValueError(f"missing Go version in {path}")
    version = max(versions, key=lambda v: tuple(int(n) for n in v.split('.')))
    if len(version.split('.')) != 3:
        raise ValueError("release Go version must pin the patch level")
    return "go" + version


def validate_metadata(metadata, system, arch, tag, commit, go_version, cgo_enabled="0"):
    # go version -m quotes -ldflags using Go string escapes. Match token
    # boundaries after unescaping quotes, rather than accepting version prefixes.
    metadata = metadata.replace(r'\"', '"')
    expected = [f"GOOS={system}", f"GOARCH={arch}", f"CGO_ENABLED={cgo_enabled}",
                f"Version={tag}", f"Commit={commit}"]
    for value in expected:
        if not re.search(re.escape(value) + r"(?=[\s\"']|$)", metadata):
            raise ValueError(f"binary metadata missing {value}")
    if not re.search(r"\b" + re.escape(go_version) + r"\b", metadata.splitlines()[0]):
        raise ValueError(f"binary was not built with {go_version}")


def inspect_binary(binary, system, arch, tag, commit, go_version, cgo_enabled="0"):
    with tempfile.TemporaryDirectory() as directory:
        path = pathlib.Path(directory) / "rssc"
        path.write_bytes(binary)
        metadata = subprocess.check_output(["go", "version", "-m", str(path)], text=True)
        validate_metadata(metadata, system, arch, tag, commit, go_version, cgo_enabled)


def inspect_deno(binary, system, arch):
    from prepare_deno import load_manifest
    manifest = load_manifest(pathlib.Path(__file__).resolve().parent.parent)
    expected = manifest['platforms'][f'{system}_{arch}']['binary_sha256']
    if hashlib.sha256(binary).hexdigest() != expected:
        raise ValueError(f'bundled Deno checksum mismatch: {system}/{arch}')


def verify(directory, tag, commit, go_version, cgo_enabled="0"):
    if not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("expected the full source commit SHA")
    directory = pathlib.Path(directory)
    archives = archive_names(tag)
    checksum_files = {value[2] for value in archives.values()}
    expected = set(archives) | checksum_files
    actual = {path.name for path in directory.iterdir() if path.is_file()}
    if actual != expected:
        raise ValueError(f"unexpected asset set: missing={expected - actual}, extra={actual - expected}")
    for checksum in checksum_files:
        records = {}
        for line in (directory / checksum).read_text().splitlines():
            digest, name = line.split(maxsplit=1)
            name = name.lstrip("*")
            if not re.fullmatch(r"[0-9a-f]{64}", digest) or name in records:
                raise ValueError(f"invalid checksum entry: {line}")
            records[name] = digest
        covered = {name for name, value in archives.items() if value[2] == checksum}
        if set(records) != covered:
            raise ValueError(f"wrong archive coverage in {checksum}")
        for name, digest in records.items():
            if hashlib.sha256((directory / name).read_bytes()).hexdigest() != digest:
                raise ValueError(f"checksum mismatch: {name}")
    required = {"rssc", "LICENSE", "README.md", "README_CN.md", "CREDITS.md", "DEPENDENCIES.md", "CHANGELOG.md",
                f"docs/releases/{tag}.md", "internal/jssdk/js/jquery.LICENSE", "internal/jssdk/js/moment.LICENSE"}
    if cgo_enabled == "0":
        required |= {"third_party/licenses/deno.txt", "docs/deno-prototype.md", "docs/upgrading.md"}
    required |= {f"third_party/licenses/{name}.txt" for name in ("cache2go", "cascadia", "feeds", "goquery", "html-to-markdown")}
    portable = tuple(map(int, tag[1:].split("."))) >= (0, 4, 0)
    if portable:
        required |= {"deno", "build/deno-runtime.json"}
    team_release = tuple(map(int, tag[1:].split("."))) >= (0, 5, 0)
    if team_release:
        required |= TEAM_RELEASE_FILES
    for name, (system, arch, _) in archives.items():
        path = directory / name
        if name.endswith(".zip"):
            with zipfile.ZipFile(path) as package:
                names = package.namelist()
                binary = package.read("rssc")
                if portable:
                    runtime = package.read("deno")
                    executable = bool((package.getinfo("deno").external_attr >> 16) & 0o111)
        else:
            with tarfile.open(path) as package:
                members = package.getmembers()
                if any(member.issym() or member.islnk() for member in members):
                    raise ValueError(f"links are not allowed in {name}")
                names = [member.name for member in members]
                binary = package.extractfile("rssc").read()
                if portable:
                    runtime = package.extractfile("deno").read()
                    executable = bool(package.getmember("deno").mode & 0o111)
        if len(set(names)) != len(names) or any(".." in pathlib.PurePosixPath(entry).parts or entry.startswith("/") for entry in names):
            raise ValueError(f"unsafe or duplicate archive entries in {name}")
        if team_release and any(private_release_path(entry) for entry in names):
            raise ValueError(f"private deployment files in {name}")
        missing = required - set(names)
        if missing or not any(entry.startswith("rules/") and entry.endswith(".js") for entry in names):
            raise ValueError(f"archive missing runtime files in {name}: {missing}")
        if portable:
            if not executable:
                raise ValueError(f"bundled Deno is not executable: {name}")
            inspect_deno(runtime, system, arch)
        inspect_binary(binary, system, arch, tag, commit, go_version, cgo_enabled)
        print(f"Verified {name}: {system}/{arch}, {tag}, {commit}")
    return expected


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", type=pathlib.Path)
    parser.add_argument("tag")
    parser.add_argument("commit")
    parser.add_argument("--cgo-enabled", choices=("0", "1"), default="0", help="use 1 for historical v8go releases")
    parser.add_argument("--go-version")
    parser.add_argument("--go-mod", type=pathlib.Path, default=pathlib.Path("go.mod"))
    args = parser.parse_args()
    verify(args.directory, args.tag, args.commit, args.go_version or go_version_from_mod(args.go_mod), args.cgo_enabled)
