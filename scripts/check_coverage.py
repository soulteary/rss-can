#!/usr/bin/env python3
"""Gate statement coverage for runtime packages, excluding entry points and tools."""
import argparse
import collections
import pathlib
import sys

MINIMUMS = {'application': 85, 'cacher': 80, 'cmd': 95, 'coalesce': 95, 'define': 95, 'feedservice': 80, 'fn': 95,
            'generator': 85, 'jssdk': 80, 'logger': 95, 'network': 75,
            'limits': 95, 'workscope': 95, 'parser': 70, 'rule': 90, 'server': 75, 'telemetry': 95}


def summarize(text):
    lines = text.splitlines()
    if not lines or lines[0] not in ('mode: atomic', 'mode: count', 'mode: set'):
        raise ValueError('missing or unsupported coverage mode')
    blocks = {}
    for line in lines[1:]:
        location, statements, count = line.split()
        statements, count = int(statements), int(count)
        if statements < 0 or count < 0:
            raise ValueError('negative statement count')
        previous = blocks.get(location)
        if previous and previous[0] != statements:
            raise ValueError('inconsistent duplicate coverage block')
        blocks[location] = (statements, bool(count) or bool(previous and previous[1]))
    packages = collections.defaultdict(lambda: [0, 0])
    for location, (statements, covered) in blocks.items():
        file = location.rsplit(':', 1)[0]
        if '/internal/' not in file:
            continue
        package = file.split('/internal/', 1)[1].rsplit('/', 1)[0]
        packages[package][1] += statements
        if covered:
            packages[package][0] += statements
    if not packages or not sum(n for _, n in packages.values()):
        raise ValueError('profile has no runtime statements')
    return dict(packages)


def check(packages, minimum=80, minimums=None):
    minimums = MINIMUMS if minimums is None else minimums
    failures = []
    covered, total = map(sum, zip(*packages.values()))
    if 100 * covered < minimum * total:
        failures.append(f'runtime total: {100 * covered / total:.2f}% < {minimum}%')
    for name, threshold in minimums.items():
        if name not in packages or packages[name][1] == 0:
            failures.append(f'{name}: no coverage evidence')
        elif 100 * packages[name][0] < threshold * packages[name][1]:
            failures.append(f'{name}: {100 * packages[name][0] / packages[name][1]:.2f}% < {threshold}%')
    return failures


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('profile', type=pathlib.Path)
    args = parser.parse_args()
    try:
        packages = summarize(args.profile.read_text())
    except (OSError, ValueError) as error:
        print(f'Invalid coverage profile: {error}', file=sys.stderr)
        return 1
    for name, (covered, total) in sorted(packages.items()):
        print(f'internal/{name}: {100 * covered / total:.2f}%' if total else f'internal/{name}: no statements')
    covered, total = map(sum, zip(*packages.values()))
    print(f'Runtime statement coverage: {100 * covered / total:.2f}% ({covered}/{total})')
    failures = check(packages)
    for failure in failures:
        print(f'FAIL: {failure}', file=sys.stderr)
    return bool(failures)

if __name__ == '__main__':
    sys.exit(main())
