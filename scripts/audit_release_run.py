#!/usr/bin/env python3
"""Audit the immutable inputs to publication-only recovery."""
import argparse
import json
import pathlib
import re


def audit(tag, source, commit, run, jobs):
    if not re.fullmatch(r'v[0-9]+\.[0-9]+\.[0-9]+', tag) or not re.fullmatch(r'[0-9a-f]{40}', source):
        raise ValueError('expected stable tag and full source SHA')
    if commit['sha'] != source or run['head_sha'] != source or run['head_branch'] != tag:
        raise ValueError('tag/build/source mismatch')
    if run['event'] != 'push' or run['path'] != '.github/workflows/build.yml' or run['status'] != 'completed':
        raise ValueError('expected completed tag-triggered Build Artifacts run')
    required = {'darwin', 'linux', 'Publish image (amd64)', 'Publish image (arm64)', 'Verify published multi-platform images'}
    if not required <= {j['name'] for j in jobs if j['conclusion'] == 'success'}:
        raise ValueError('original builds and image verification must all succeed')
    allowed_steps = {'Verify checksums, payloads and exact source metadata', 'Upload and publish verified assets'}
    for job in jobs:
        if job['conclusion'] == 'success':
            continue
        if job['name'] == 'Dispatch verified release reports' and job['conclusion'] == 'skipped':
            continue
        failures = {s['name'] for s in job.get('steps', []) if s['conclusion'] == 'failure'}
        if job['name'] != 'Verify archives and publish Release' or job['conclusion'] != 'failure' or not failures or not failures <= allowed_steps:
            raise ValueError('only publication failures can be recovered without rebuilding')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('tag')
    parser.add_argument('source')
    parser.add_argument('directory', type=pathlib.Path)
    args = parser.parse_args()
    def read(name):
        return json.loads((args.directory / name).read_text())
    audit(args.tag, args.source, read('source.json'), read('build.json'), read('jobs.json')['jobs'])
