#!/usr/bin/env python3
"""Reject drift between the source Deno pin, Docker, CI and installation docs."""
import pathlib
import re


def check(root):
    root = pathlib.Path(root)
    version = (root / '.deno-version').read_text().strip()
    if not re.fullmatch(r'\d+\.\d+\.\d+', version):
        raise ValueError('.deno-version must pin a full release version')
    dockerfile = (root / 'docker/Dockerfile').read_text()
    if re.findall(r'FROM denoland/deno:bin-([\d.]+)\b', dockerfile) != [version]:
        raise ValueError('Docker Deno version differs from .deno-version')
    workflows = list((root / '.github/workflows').glob('*.yml'))
    setups = 0
    for path in workflows:
        text = path.read_text()
        count = len(re.findall(r'uses:\s*denoland/setup-deno@', text))
        if not count:
            continue
        setups += count
        pins = re.findall(r'deno-version:\s*[\'"]?v?([\d.]+)', text)
        if pins != [version] * count:
            raise ValueError(f'{path.name}: each Deno setup must pin {version}')
    if not setups:
        raise ValueError('no pinned Deno CI setup found')
    for name in ('README.md', 'README_CN.md', 'docs/upgrading.md', 'docs/deno-prototype.md'):
        if version not in (root / name).read_text():
            raise ValueError(f'{name}: missing Deno version {version}')
    return version


if __name__ == '__main__':
    print(f'Deno runtime pins agree: {check(pathlib.Path(__file__).resolve().parent.parent)}')
