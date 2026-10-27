import hashlib
import io
import pathlib
import tarfile
import tempfile
import unittest
import zipfile
from unittest.mock import patch

import verify_release_assets as release


class VerifyReleaseTests(unittest.TestCase):
    tag = "v0.3.12"
    commit = "a" * 40

    def fixture(self, directory, omit=None):
        files = {name: b"fixture" for name in ["rssc", "LICENSE", "README.md", "README_CN.md", "CREDITS.md", "DEPENDENCIES.md", "CHANGELOG.md",
                 "docs/releases/v0.3.12.md", "rules/example/feed.js", "internal/jssdk/js/jquery.LICENSE", "internal/jssdk/js/moment.LICENSE"]}
        files.update({f"third_party/licenses/{name}.txt": b"license" for name in ("cache2go", "cascadia", "feeds", "goquery", "html-to-markdown")})
        if omit:
            del files[omit]
        checksums = {}
        for name, (_, _, checksum) in release.archive_names(self.tag).items():
            path = directory / name
            if name.endswith(".zip"):
                with zipfile.ZipFile(path, "w") as package:
                    for entry, body in files.items():
                        package.writestr(entry, body)
            else:
                with tarfile.open(path, "w:gz") as package:
                    for entry, body in files.items():
                        member = tarfile.TarInfo(entry)
                        member.size = len(body)
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
        for missing in ["rules/example/feed.js", "third_party/licenses/feeds.txt"]:
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

    def test_binary_identity(self):
        metadata = f"/tmp/rssc: go1.27.3\n build CGO_ENABLED=1\n build GOOS=linux\n build GOARCH=amd64\n build -ldflags=\"-X x.Version={self.tag} -X x.Commit={self.commit}\"\n"
        release.validate_metadata(metadata, "linux", "amd64", self.tag, self.commit, "go1.27.3")
        for replacement in [metadata.replace(self.tag, "v0.3.120"), metadata.replace("amd64", "arm64"), metadata.replace(self.commit, "b" * 40)]:
            with self.assertRaises(ValueError):
                release.validate_metadata(replacement, "linux", "amd64", self.tag, self.commit, "go1.27.3")


if __name__ == "__main__":
    unittest.main()
