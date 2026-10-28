#!/usr/bin/env python3
"""Verify all release archives before publication or after downloading a Release."""
import argparse
import hashlib
import pathlib
import re
import subprocess
import tarfile
import tempfile
import zipfile


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
    for name, (system, arch, _) in archives.items():
        path = directory / name
        if name.endswith(".zip"):
            with zipfile.ZipFile(path) as package:
                names = package.namelist()
                binary = package.read("rssc")
        else:
            with tarfile.open(path) as package:
                members = package.getmembers()
                if any(member.issym() or member.islnk() for member in members):
                    raise ValueError(f"links are not allowed in {name}")
                names = [member.name for member in members]
                binary = package.extractfile("rssc").read()
        if len(set(names)) != len(names) or any(".." in pathlib.PurePosixPath(entry).parts or entry.startswith("/") for entry in names):
            raise ValueError(f"unsafe or duplicate archive entries in {name}")
        missing = required - set(names)
        if missing or not any(entry.startswith("rules/") and entry.endswith(".js") for entry in names):
            raise ValueError(f"archive missing runtime files in {name}: {missing}")
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
