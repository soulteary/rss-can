#!/usr/bin/env python3
"""Publish verified archives; retries never replace a public Release."""
import argparse
import hashlib
import json
import pathlib
import subprocess

from verify_release_assets import archive_names, go_version_from_mod, verify


def gh(*args):
    return subprocess.check_output(['gh', *args], text=True).strip()


def validate_assets(release, files):
    assets = {asset['name']: asset for asset in release['assets']}
    if len(assets) != len(release['assets']) or set(assets) != set(files):
        raise ValueError('Release asset set differs from verified files')
    for name, path in files.items():
        asset = assets[name]
        digest = 'sha256:' + hashlib.sha256(path.read_bytes()).hexdigest()
        if asset['state'] != 'uploaded' or asset['size'] != path.stat().st_size or asset.get('digest') != digest:
            raise ValueError(f'Release asset differs from verified file: {name}')


def publish(directory, tag, source, repository, notes):
    files = {p.name: p for p in pathlib.Path(directory).iterdir() if p.is_file()}
    archives = archive_names(tag)
    if set(files) != set(archives) | {v[2] for v in archives.values()}:
        raise ValueError('unexpected publication asset set')
    notes = pathlib.Path(notes)
    if not notes.is_file() or not notes.read_text().strip():
        raise ValueError(f'missing release notes: {notes}')

    def same_source():
        commit = json.loads(gh('api', f'repos/{repository}/commits/{tag}'))
        if commit['sha'] != source:
            raise ValueError('remote release tag no longer matches verified source')

    # Listing includes drafts. API/network/permission failures must abort, never
    # masquerade as a missing release and trigger a second creation attempt.
    same_source()
    pages = json.loads(gh('api', f'repos/{repository}/releases', '--paginate', '--slurp'))
    matches = [r for page in pages for r in page if r['tag_name'] == tag]
    if len(matches) > 1:
        raise ValueError('multiple Releases for the same tag')
    if not matches:
        gh('release', 'create', tag, '--repo', repository, '--verify-tag', '--draft', '--title', tag, '--notes-file', str(notes))
        api = json.loads(gh('release', 'view', tag, '--repo', repository, '--json', 'apiUrl'))['apiUrl']
    else:
        api = matches[0]['url']

    def current():
        release = json.loads(gh('api', api))
        if release['tag_name'] != tag:
            raise ValueError('Release identity changed')
        return release

    release = current()
    identity = release['id']
    if not release['draft']:
        validate_assets(release, files)
        return  # An identical public release is a successful, read-only retry.
    # Re-upload only absent/different files, preserving successful uploads on retry.
    existing = {a['name']: a for a in release['assets']}
    if set(existing) - set(files):
        raise ValueError('draft contains unexpected assets; refusing publication')
    for name, path in sorted(files.items()):
        asset = existing.get(name)
        if asset:
            try:
                validate_assets({'assets': [asset]}, {name: path})
                continue
            except ValueError:
                pass
        same_source()
        latest = current()
        if latest['id'] != identity or not latest['draft']:
            raise ValueError('Release is no longer the original draft')
        gh('release', 'upload', tag, str(path), '--repo', repository, '--clobber')
    same_source()
    latest = current()
    if latest['id'] != identity:
        raise ValueError('Release identity changed')
    validate_assets(latest, files)
    if latest['draft']:
        gh('release', 'edit', tag, '--repo', repository, '--draft=false', '--latest', '--title', tag, '--notes-file', str(notes))
    published = current()
    if published['draft'] or published['id'] != identity:
        raise ValueError('Release was not published')
    validate_assets(published, files)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('directory', type=pathlib.Path)
    parser.add_argument('tag')
    parser.add_argument('source')
    parser.add_argument('--repo', required=True)
    parser.add_argument('--notes', required=True, type=pathlib.Path)
    parser.add_argument('--go-mod', type=pathlib.Path, default=pathlib.Path('go.mod'))
    parser.add_argument('--cgo-enabled', choices=('0', '1'), default='0')
    args = parser.parse_args()
    verify(args.directory, args.tag, args.source, go_version_from_mod(args.go_mod), args.cgo_enabled)
    publish(args.directory, args.tag, args.source, args.repo, args.notes)
