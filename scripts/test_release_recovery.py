import copy
import pathlib
import tempfile
import unittest

from audit_release_run import audit
from release_preflight import preflight
from verify_release_assets import go_version_from_mod


class RecoveryTests(unittest.TestCase):
    tag, source = 'v0.3.13', 'a' * 40

    def fixture(self):
        run = dict(head_sha=self.source, head_branch=self.tag, event='push',
                   path='.github/workflows/build.yml', status='completed')
        jobs = [dict(name=name, conclusion='success') for name in
                ['release_tooling', 'darwin', 'linux', 'Publish image (amd64)',
                 'Publish image (arm64)', 'Verify published multi-platform images']]
        jobs += [dict(name='Verify archives and publish Release', conclusion='failure',
                      steps=[dict(name='Upload and publish verified assets', conclusion='failure')]),
                 dict(name='Dispatch verified release reports', conclusion='skipped')]
        return dict(sha=self.source), run, jobs

    def test_later_version_upload_failure_can_be_completed(self):
        audit(self.tag, self.source, *self.fixture())

    def test_build_image_unrelated_failure_or_wrong_tag_cannot_be_completed(self):
        for change in ['binary', 'image', 'step', 'tag', 'source', 'event', 'running']:
            commit, run, jobs = copy.deepcopy(self.fixture())
            if change == 'binary': jobs[1]['conclusion'] = 'failure'
            if change == 'image': jobs[3]['conclusion'] = 'failure'
            if change == 'step': jobs[-2]['steps'][0]['name'] = 'Run actions/checkout@v7'
            if change == 'tag': run['head_branch'] = 'main'
            if change == 'source': commit['sha'] = 'b' * 40
            if change == 'event': run['event'] = 'workflow_dispatch'
            if change == 'running': run['status'] = 'in_progress'
            with self.subTest(change=change), self.assertRaises(ValueError):
                audit(self.tag, self.source, commit, run, jobs)

    def test_verification_failure_and_already_published_run_can_be_rechecked(self):
        commit, run, jobs = self.fixture()
        jobs[-2]['steps'][0]['name'] = 'Verify checksums, payloads and exact source metadata'
        audit(self.tag, self.source, commit, run, jobs)
        jobs[-2]['conclusion'] = 'success'
        jobs[-1]['conclusion'] = 'success'
        audit(self.tag, self.source, commit, run, jobs)


class PreflightTests(unittest.TestCase):
    def test_later_go_version_and_original_source_toolchain(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = pathlib.Path(temporary) / 'go.mod'
            for contents, expected in [('module example\ngo 1.28.1\n', 'go1.28.1'),
                                       ('go 1.27.3\ntoolchain go1.28.2\n', 'go1.28.2'),
                                       ('go 1.28.2\ntoolchain go1.27.3\n', 'go1.28.2')]:
                path.write_text(contents)
                self.assertEqual(go_version_from_mod(path), expected)
            for invalid in ['module example\n', 'module example\ngo 1.28\n']:
                path.write_text(invalid)
                with self.assertRaises(ValueError): go_version_from_mod(path)

    def test_missing_notes_and_mismatched_docker_go_fail_before_publication(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            (root / 'go.mod').write_text('go 1.28.1\n')
            (root / 'docker').mkdir()
            (root / 'docker/Dockerfile').write_text('FROM golang:1.28.1-bookworm AS Builder\n')
            notes = root / 'docs/releases/v0.3.13.md'
            notes.parent.mkdir(parents=True)
            with self.assertRaisesRegex(ValueError, 'notes'): preflight('v0.3.13', root)
            notes.write_text('Release v0.3.13\n')
            preflight('v0.3.13', root)
            (root / 'docker/Dockerfile').write_text('FROM golang:1.27.3-bookworm\n')
            with self.assertRaisesRegex(ValueError, 'same Go'): preflight('v0.3.13', root)
            with self.assertRaisesRegex(ValueError, 'stable'): preflight('v0.3.13-rc1', root)


if __name__ == '__main__':
    unittest.main()
