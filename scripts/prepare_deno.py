#!/usr/bin/env python3
"""Stage checksum-pinned official Deno runtimes for portable release archives."""
import argparse
import hashlib
import io
import json
import pathlib
import urllib.request
import zipfile


def load_manifest(root):
    root = pathlib.Path(root)
    manifest = json.loads((root / 'build/deno-runtime.json').read_text())
    if manifest['version'] != (root / '.deno-version').read_text().strip():
        raise ValueError('Deno distribution manifest differs from .deno-version')
    if set(manifest['platforms']) != {'linux_amd64', 'linux_arm64', 'darwin_amd64', 'darwin_arm64'}:
        raise ValueError('Deno distribution must cover all four release platforms')
    return manifest


def stage(root, system):
    root = pathlib.Path(root)
    manifest = load_manifest(root)
    for platform, asset in manifest['platforms'].items():
        if not platform.startswith(system + '_'):
            continue
        expected = f"https://github.com/denoland/deno/releases/download/v{manifest['version']}/{asset['name']}"
        if asset['url'] != expected:
            raise ValueError('Deno download must use the pinned official release')
        with urllib.request.urlopen(asset['url'], timeout=120) as response:
            data = response.read()
        if hashlib.sha256(data).hexdigest() != asset['sha256']:
            raise ValueError(f'Deno ZIP checksum mismatch: {platform}')
        with zipfile.ZipFile(io.BytesIO(data)) as package:
            if package.namelist() != ['deno']:
                raise ValueError('unexpected Deno ZIP payload')
            binary = package.read('deno')
        if hashlib.sha256(binary).hexdigest() != asset['binary_sha256']:
            raise ValueError(f'Deno binary checksum mismatch: {platform}')
        target = root / 'build/runtime' / platform / 'deno'
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(binary)
        target.chmod(0o755)
        print(f'Staged Deno {manifest["version"]}: {platform}')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('system', choices=('linux', 'darwin'))
    args = parser.parse_args()
    stage(pathlib.Path(__file__).resolve().parent.parent, args.system)
