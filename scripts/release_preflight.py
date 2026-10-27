#!/usr/bin/env python3
"""Check stable release inputs before any image or archive publication."""
import pathlib
import sys

from verify_release_assets import archive_names, go_version_from_mod


def preflight(tag, root):
    archive_names(tag)  # Restrict to the archive naming scheme we can verify.
    notes = pathlib.Path(root) / 'docs' / 'releases' / f'{tag}.md'
    if not notes.is_file() or not notes.read_text().strip():
        raise ValueError(f'missing release notes: {notes}')
    version = go_version_from_mod(pathlib.Path(root) / 'go.mod').removeprefix('go')
    dockerfile = (pathlib.Path(root) / 'docker' / 'Dockerfile').read_text()
    if f'FROM golang:{version}-' not in dockerfile:
        raise ValueError('Docker builder and go.mod must use the same Go version')


if __name__ == '__main__':
    preflight(sys.argv[1], pathlib.Path('.'))
