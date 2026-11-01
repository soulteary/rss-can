#!/usr/bin/env python3
"""Start a native release archive with no PATH or configured Deno dependency."""
import argparse
import json
import os
import pathlib
import platform
import socket
import subprocess
import tarfile
import tempfile
import time
import urllib.error
import urllib.request
import zipfile


def smoke(archive):
    with tempfile.TemporaryDirectory() as temporary:
        root = pathlib.Path(temporary).resolve()
        if str(archive).endswith('.zip'):
            with zipfile.ZipFile(archive) as package:
                for member in package.infolist():
                    path = root / member.filename
                    if not path.resolve().is_relative_to(root):
                        raise ValueError('unsafe archive entry')
                    if member.is_dir():
                        path.mkdir(parents=True, exist_ok=True)
                    else:
                        path.parent.mkdir(parents=True, exist_ok=True)
                        path.write_bytes(package.read(member))
                        path.chmod((member.external_attr >> 16) & 0o777)
        else:
            with tarfile.open(archive) as package:
                package.extractall(root, filter='data')
        binary, deno = root / 'rssc', root / 'deno'
        runtime = subprocess.check_output([str(deno), '--version'], text=True)
        version = json.loads((root / 'build/deno-runtime.json').read_text())['version']
        assert runtime.split()[:2] == ['deno', version], runtime
        with socket.socket() as listener:
            listener.bind(('127.0.0.1', 0))
            port = listener.getsockname()[1]
        env = {k: v for k, v in os.environ.items() if not k.startswith('RSS_')}
        env.update(PATH='', RSS_REDIS='false', RSS_PORT=str(port), DENO_NO_UPDATE_CHECK='1')
        with (root / 'smoke.log').open('w+') as log:
            process = subprocess.Popen([str(binary)], cwd=root, env=env, stdout=log, stderr=log)
            try:
                deadline = time.monotonic() + 30
                while time.monotonic() < deadline:
                    if process.poll() is not None:
                        raise RuntimeError('archive exited before readiness')
                    try:
                        with urllib.request.urlopen(f'http://127.0.0.1:{port}/_/ready/', timeout=1) as response:
                            if response.status == 200:
                                print(f'Portable archive ready with bundled {runtime.splitlines()[0]}: {archive}')
                                return
                    except (urllib.error.URLError, TimeoutError):
                        pass
                    time.sleep(.2)
                raise RuntimeError('archive did not become ready')
            except Exception:
                log.flush()
                print((root / 'smoke.log').read_text())
                raise
            finally:
                process.terminate()
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('directory', type=pathlib.Path)
    args = parser.parse_args()
    system = platform.system().lower()
    arch = {'x86_64': 'x86_64' if system == 'linux' else 'amd64', 'aarch64': 'arm64', 'arm64': 'arm64'}[platform.machine()]
    files = list(args.directory.glob(f'*_{system}_{arch}.*'))
    files = [p for p in files if p.name.endswith(('.zip', '.tar.gz'))]
    if len(files) != 1:
        raise SystemExit(f'expected one native archive, got {files}')
    smoke(files[0])
