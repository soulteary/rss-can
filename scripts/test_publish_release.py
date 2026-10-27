import copy
import hashlib
import json
import pathlib
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import publish_release as publisher
from verify_release_assets import archive_names


class FakeGitHub:
    """Stateful API fake: drafts and uploaded assets survive failed invocations."""
    def __init__(self, files, tag, source):
        self.files, self.tag, self.source = files, tag, source
        self.release = None
        self.mutations = []
        self.fail_upload = None
        self.fail_list = False

    def asset(self, path):
        return dict(name=path.name, state='uploaded', size=path.stat().st_size,
                    digest='sha256:' + hashlib.sha256(path.read_bytes()).hexdigest())

    def __call__(self, *args):
        if args[0] == 'api':
            endpoint = args[1]
            if '/commits/' in endpoint:
                return json.dumps({'sha': self.source})
            if endpoint.endswith('/releases'):
                if self.fail_list:
                    raise subprocess.CalledProcessError(1, ['gh', *args])
                return json.dumps([[self.release] if self.release else []])
            if endpoint == 'https://api.github.com/releases/1':
                return json.dumps(self.release)
        if args[:2] == ('release', 'view'):
            return json.dumps({'apiUrl': self.release['url']})
        if args[:2] == ('release', 'create'):
            if self.release:
                raise AssertionError('attempted duplicate release creation')
            self.mutations.append('create')
            self.release = dict(id=1, url='https://api.github.com/releases/1',
                                tag_name=self.tag, draft=True, assets=[])
            return ''
        if args[:2] == ('release', 'upload'):
            path = pathlib.Path(args[3])
            if path.name == self.fail_upload:
                raise subprocess.CalledProcessError(1, ['gh', *args])
            if not self.release['draft']:
                raise AssertionError('attempted upload to public release')
            self.mutations.append('upload:' + path.name)
            self.release['assets'] = [a for a in self.release['assets'] if a['name'] != path.name] + [self.asset(path)]
            return ''
        if args[:2] == ('release', 'edit'):
            self.mutations.append('publish')
            self.release['draft'] = False
            return ''
        raise AssertionError(args)


class PublishTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        root = pathlib.Path(self.temporary.name)
        self.root = root / 'assets'
        self.root.mkdir()
        self.tag, self.source = 'v0.3.13', 'a' * 40
        archives = archive_names(self.tag)
        for name in set(archives) | {v[2] for v in archives.values()}:
            (self.root / name).write_text(name)
        self.notes = root / 'notes.md'
        self.notes.write_text('Release v0.3.13\n')
        self.api = FakeGitHub(self.root, self.tag, self.source)
        self.patcher = patch.object(publisher, 'gh', self.api)
        self.patcher.start()
        self.addCleanup(self.patcher.stop)

    def publish(self):
        publisher.publish(self.root, self.tag, self.source, 'owner/repo', self.notes)

    def test_first_publication_uploads_all_files_before_publishing(self):
        self.publish()
        self.assertFalse(self.api.release['draft'])
        self.assertEqual(self.api.mutations[0], 'create')
        self.assertEqual(len(self.api.release['assets']), 7)
        self.assertEqual(self.api.mutations[-1], 'publish')

    def test_interrupted_upload_resumes_same_draft_without_replacing_valid_assets(self):
        self.api.fail_upload = sorted(p.name for p in self.root.iterdir())[2]
        with self.assertRaises(subprocess.CalledProcessError):
            self.publish()
        self.assertTrue(self.api.release['draft'])
        self.assertEqual(len(self.api.release['assets']), 2)
        self.api.fail_upload = None
        self.publish()
        self.assertEqual(self.api.mutations.count('create'), 1)
        uploads = [a for a in self.api.mutations if a.startswith('upload:')]
        self.assertEqual(len(uploads), 7)
        self.assertEqual(len(set(uploads)), 7)
        self.assertFalse(self.api.release['draft'])

    def test_public_retry_is_read_only(self):
        self.publish()
        before = copy.deepcopy(self.api.release)
        self.api.mutations.clear()
        self.publish()
        self.assertEqual(self.api.release, before)
        self.assertEqual(self.api.mutations, [])

    def test_public_asset_mismatch_never_overwrites(self):
        self.publish()
        self.api.release['assets'][0]['digest'] = 'sha256:' + '0' * 64
        self.api.mutations.clear()
        with self.assertRaisesRegex(ValueError, 'differs'):
            self.publish()
        self.assertEqual(self.api.mutations, [])

    def test_api_failure_does_not_create_another_release(self):
        self.api.fail_list = True
        with self.assertRaises(subprocess.CalledProcessError):
            self.publish()
        self.assertEqual(self.api.mutations, [])

    def test_wrong_source_and_missing_notes_fail_before_mutations(self):
        self.api.source = 'b' * 40
        with self.assertRaisesRegex(ValueError, 'source'):
            self.publish()
        self.api.source = self.source
        self.notes.unlink()
        with self.assertRaisesRegex(ValueError, 'notes'):
            self.publish()
        self.assertEqual(self.api.mutations, [])

    def test_unexpected_draft_asset_prevents_publication(self):
        self.api.fail_upload = sorted(p.name for p in self.root.iterdir())[0]
        with self.assertRaises(subprocess.CalledProcessError):
            self.publish()
        self.api.release['assets'].append(dict(name='unexpected.zip'))
        self.api.fail_upload = None
        self.api.mutations.clear()
        with self.assertRaisesRegex(ValueError, 'unexpected assets'):
            self.publish()
        self.assertEqual(self.api.mutations, [])

    def test_concurrent_publication_is_detected_before_upload(self):
        original = self.api.__call__
        reads = 0
        def raced(*args):
            nonlocal reads
            if args == ('api', 'https://api.github.com/releases/1'):
                reads += 1
                if reads == 2:
                    self.api.release['draft'] = False
            return original(*args)
        with patch.object(publisher, 'gh', side_effect=raced):
            with self.assertRaisesRegex(ValueError, 'original draft'):
                self.publish()
        self.assertEqual(self.api.mutations, ['create'])

    def test_corrupt_draft_asset_is_reuploaded_before_publication(self):
        self.publish()
        self.api.release['draft'] = True
        self.api.release['assets'][0]['digest'] = None
        self.api.mutations.clear()
        self.publish()
        self.assertEqual(len(self.api.mutations), 2)  # One repair, then publication.
        self.assertEqual(self.api.mutations[-1], 'publish')


if __name__ == '__main__':
    unittest.main()
