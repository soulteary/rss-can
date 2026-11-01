import hashlib
import io
import json
import pathlib
import os
import subprocess
import tarfile
import tempfile
import unittest
import zipfile
from unittest.mock import patch

import verify_release_assets as release
REAL_INSPECT_DENO = release.inspect_deno


class VerifyReleaseTests(unittest.TestCase):
    tag = "v0.3.12"
    commit = "a" * 40

    def fixture(self, directory, omit=None, binaries=None):
        files = {name: b"fixture" for name in ["rssc", "LICENSE", "README.md", "README_CN.md", "CREDITS.md", "DEPENDENCIES.md", "CHANGELOG.md",
                 f"docs/releases/{self.tag}.md", "docs/deno-prototype.md", "docs/upgrading.md", "rules/example/feed.js", "internal/jssdk/js/jquery.LICENSE", "internal/jssdk/js/moment.LICENSE"]}
        files.update({f"third_party/licenses/{name}.txt": b"license" for name in ("cache2go", "cascadia", "feeds", "goquery", "html-to-markdown", "deno")})
        if tuple(map(int, self.tag[1:].split("."))) >= (0, 4, 0):
            files.update({"deno": b"runtime", "build/deno-runtime.json": b"{}"})
        if omit:
            del files[omit]
        checksums = {}
        for name, (system, arch, checksum) in release.archive_names(self.tag).items():
            if binaries:
                files["rssc"] = binaries[(system, arch)]
            path = directory / name
            if name.endswith(".zip"):
                with zipfile.ZipFile(path, "w") as package:
                    for entry, body in files.items():
                        member = zipfile.ZipInfo(entry)
                        member.external_attr = (0o100755 if entry in ("rssc", "deno") else 0o100644) << 16
                        package.writestr(member, body)
            else:
                with tarfile.open(path, "w:gz") as package:
                    for entry, body in files.items():
                        member = tarfile.TarInfo(entry)
                        member.size = len(body)
                        member.mode = 0o755 if entry in ("rssc", "deno") else 0o644
                        package.addfile(member, io.BytesIO(body))
            checksums.setdefault(checksum, []).append(f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {name}\n")
        for name, records in checksums.items():
            (directory / name).write_text("".join(records))

    @patch.object(release, "inspect_binary")
    def test_complete_archives(self, inspect):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            self.fixture(root)
            self.assertEqual(len(release.verify(root, self.tag, self.commit, "go1.27.3")), 7)
            self.assertEqual(inspect.call_count, 4)

    @patch.object(release, "inspect_binary")
    def test_tampered_archive(self, inspect):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            self.fixture(root)
            (root / next(iter(release.archive_names(self.tag)))).write_bytes(b"tampered")
            with self.assertRaisesRegex(ValueError, "checksum mismatch"):
                release.verify(root, self.tag, self.commit, "go1.27.3")

    @patch.object(release, "inspect_binary")
    def test_missing_rules_or_license(self, inspect):
        for missing in ["rules/example/feed.js", "third_party/licenses/feeds.txt", "third_party/licenses/deno.txt", "docs/deno-prototype.md", "docs/upgrading.md"]:
            with self.subTest(missing=missing), tempfile.TemporaryDirectory() as temporary:
                root = pathlib.Path(temporary)
                self.fixture(root, omit=missing)
                with self.assertRaisesRegex(ValueError, "missing runtime files"):
                    release.verify(root, self.tag, self.commit, "go1.27.3")

    @patch.object(release, "inspect_binary")
    def test_missing_architecture(self, inspect):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            self.fixture(root)
            (root / next(iter(release.archive_names(self.tag)))).unlink()
            with self.assertRaisesRegex(ValueError, "asset set"):
                release.verify(root, self.tag, self.commit, "go1.27.3")

    def test_real_four_platform_archives_and_quoted_linker_metadata(self):
        # Exercise go build -> archive -> checksum -> go version -m together.
        # No network or external module is needed for this tiny binary.
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            (root / "main.go").write_text('package main\nvar Version, Commit string\nfunc main() {}\n')
            go_version = subprocess.check_output(["go", "env", "GOVERSION"], text=True).strip()
            binaries = {}
            flags = f'-s -w -X "main.Version={self.tag}" -X "main.Commit={self.commit}"'
            for system, arch in [("linux", "amd64"), ("linux", "arm64"), ("darwin", "amd64"), ("darwin", "arm64")]:
                binary = root / "binary"
                env = dict(os.environ, GOOS=system, GOARCH=arch, CGO_ENABLED="0", GO111MODULE="off")
                subprocess.run(["go", "build", "-ldflags", flags, "-o", str(binary), str(root / "main.go")],
                               env=env, check=True, capture_output=True)
                binaries[(system, arch)] = binary.read_bytes()
            assets = root / "assets"
            assets.mkdir()
            self.fixture(assets, binaries=binaries)
            self.assertEqual(len(release.verify(assets, self.tag, self.commit, go_version)), 7)

    def test_binary_identity(self):
        metadata = f"/tmp/rssc: go1.27.3\n build CGO_ENABLED=0\n build GOOS=linux\n build GOARCH=amd64\n build -ldflags=\"-X x.Version={self.tag} -X x.Commit={self.commit}\"\n"
        release.validate_metadata(metadata, "linux", "amd64", self.tag, self.commit, "go1.27.3")
        for replacement in [metadata.replace(self.tag, "v0.3.120"), metadata.replace("amd64", "arm64"), metadata.replace(self.commit, "b" * 40)]:
            with self.assertRaises(ValueError):
                release.validate_metadata(replacement, "linux", "amd64", self.tag, self.commit, "go1.27.3")

    def test_legacy_cgo_requires_explicit_opt_in(self):
        metadata = f'/tmp/rssc: go1.27.3\n build CGO_ENABLED=1\n build GOOS=linux\n build GOARCH=amd64\n build -ldflags="-X x.Version={self.tag} -X x.Commit={self.commit}"\n'
        with self.assertRaisesRegex(ValueError, 'CGO_ENABLED=0'):
            release.validate_metadata(metadata, "linux", "amd64", self.tag, self.commit, "go1.27.3")
        release.validate_metadata(metadata, "linux", "amd64", self.tag, self.commit, "go1.27.3", "1")

    def test_real_goreleaser_quoted_linker_flags(self):
        flags = f'-w -s -X "github.com/soulteary/RSS-Can/internal/version.Version={self.tag}" -X "github.com/soulteary/RSS-Can/internal/version.Commit={self.commit}"'
        metadata = f'/tmp/rssc: go1.27.3\n build -ldflags={json.dumps(flags)}\n build CGO_ENABLED=0\n build GOOS=darwin\n build GOARCH=arm64\n'
        release.validate_metadata(metadata, "darwin", "arm64", self.tag, self.commit, "go1.27.3")
        with self.assertRaises(ValueError):
            release.validate_metadata(metadata.replace(self.tag, self.tag+"0"), "darwin", "arm64", self.tag, self.commit, "go1.27.3")


class PortableRuntimeTests(VerifyReleaseTests):
    tag = "v0.4.0"

    def setUp(self):
        patcher = patch.object(release, 'inspect_deno')
        self.inspect_runtime = patcher.start()
        self.addCleanup(patcher.stop)

    def test_runtime_hash_rejects_untrusted_binary(self):
        # Call the real validator even though archive fixtures use fake runtimes.
        self.inspect_runtime.side_effect = None
        from prepare_deno import load_manifest
        manifest = load_manifest(pathlib.Path(__file__).resolve().parent.parent)
        self.assertEqual(manifest['version'], '2.9.8')
        self.assertEqual(len(manifest['platforms']), 4)
        with self.assertRaisesRegex(ValueError, 'Deno checksum mismatch'):
            REAL_INSPECT_DENO(b'tampered', 'linux', 'amd64')

    @patch.object(release, 'inspect_binary')
    def test_portable_runtime_is_validated_for_every_platform(self, inspect):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            self.fixture(root)
            release.verify(root, self.tag, self.commit, 'go1.27.3')
            self.assertEqual(self.inspect_runtime.call_count, 4)
            self.assertEqual({call.args[1:] for call in self.inspect_runtime.call_args_list},
                             {('linux', 'amd64'), ('linux', 'arm64'), ('darwin', 'amd64'), ('darwin', 'arm64')})


if __name__ == "__main__":
    unittest.main()
