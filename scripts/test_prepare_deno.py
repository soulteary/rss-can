import hashlib
import io
import json
import pathlib
import tempfile
import unittest
import zipfile
from unittest.mock import patch

from prepare_deno import load_manifest, stage


class PrepareDenoTests(unittest.TestCase):
    def fixture(self, root):
        data = io.BytesIO()
        with zipfile.ZipFile(data, 'w') as package:
            package.writestr('deno', b'runtime')
        archive = data.getvalue()
        platforms = {}
        for platform in ('linux_amd64', 'linux_arm64', 'darwin_amd64', 'darwin_arm64'):
            name = platform + '.zip'
            platforms[platform] = {'name': name, 'url': f'https://github.com/denoland/deno/releases/download/v2.9.8/{name}',
                                   'sha256': hashlib.sha256(archive).hexdigest(), 'binary_sha256': hashlib.sha256(b'runtime').hexdigest()}
        (root / 'build').mkdir()
        (root / '.deno-version').write_text('2.9.8\n')
        (root / 'build/deno-runtime.json').write_text(json.dumps({'version': '2.9.8', 'platforms': platforms}))
        return archive

    def test_verified_staging_and_executable_permissions(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            data = self.fixture(root)
            with patch('urllib.request.urlopen', side_effect=lambda *args, **kw: io.BytesIO(data)):
                stage(root, 'linux')
            self.assertEqual((root / 'build/runtime/linux_amd64/deno').read_bytes(), b'runtime')
            self.assertTrue((root / 'build/runtime/linux_arm64/deno').stat().st_mode & 0o111)
            self.assertFalse((root / 'build/runtime/darwin_arm64').exists())

    def test_tampered_download_fails_before_writing_runtime(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            self.fixture(root)
            with patch('urllib.request.urlopen', return_value=io.BytesIO(b'tampered')):
                with self.assertRaisesRegex(ValueError, 'ZIP checksum mismatch'):
                    stage(root, 'linux')
            self.assertFalse((root / 'build/runtime').exists())

    def test_version_drift_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            self.fixture(root)
            (root / '.deno-version').write_text('2.9.9\n')
            with self.assertRaisesRegex(ValueError, 'differs'):
                load_manifest(root)
