import pathlib
import tempfile
import unittest
from check_runtime_versions import check


class RuntimeVersionTests(unittest.TestCase):
    def fixture(self, root):
        files = {'.deno-version': '2.9.8\n',
                 'docker/Dockerfile': 'FROM denoland/deno:bin-2.9.8 AS Deno\n',
                 '.github/workflows/build.yml': 'uses: denoland/setup-deno@v2\nwith:\n  deno-version: v2.9.8\n',
                 **{name: 'Deno 2.9.8\n' for name in ('README.md', 'README_CN.md', 'docs/upgrading.md', 'docs/deno-prototype.md')}}
        for name, text in files.items():
            path = root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(text)
        return files

    def test_agreement_and_each_kind_of_drift(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            files = self.fixture(root)
            self.assertEqual(check(root), '2.9.8')
            for name, text in files.items():
                with self.subTest(name=name):
                    (root / name).write_text(text.replace('2.9.8', '2.10.0'))
                    with self.assertRaises(ValueError): check(root)
                    (root / name).write_text(text)
            workflow = root / '.github/workflows/build.yml'
            workflow.write_text('uses: denoland/setup-deno@v2\n')
            with self.assertRaises(ValueError): check(root)
            workflow.write_text('name: no runtime\n')
            with self.assertRaises(ValueError): check(root)
            (root / '.deno-version').write_text('2.9\n')
            with self.assertRaises(ValueError): check(root)

    def test_repository_pins(self):
        self.assertTrue(check(pathlib.Path(__file__).resolve().parent.parent))
