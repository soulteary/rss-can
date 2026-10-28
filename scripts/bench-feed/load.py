#!/usr/bin/env python3
"""Exercise the actual HTTP feed server against a deterministic local upstream."""
import argparse
import concurrent.futures
import http.server
import json
import os
import pathlib
import socket
import statistics
import subprocess
import tempfile
import threading
import time
import urllib.error
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True)
    parser.add_argument('--requests', type=int, default=256)
    parser.add_argument('--concurrency', type=int, default=16)
    parser.add_argument('--rules', type=int, default=1)
    parser.add_argument('--cold', action='store_true')
    parser.add_argument('--dates', action='store_true')
    parser.add_argument('--config-cache-ttl', type=int, default=0)
    args = parser.parse_args()
    if min(args.requests, args.concurrency, args.rules) < 1:
        parser.error('counts must be positive')
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    counts = {'upstream': 0}
    lock = threading.Lock()

    class Upstream(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            with lock:
                counts['upstream'] += 1
            time.sleep(.025)
            body = ('<html><body>' + ''.join(
                f'<article><h2>Item {i}</h2><a href="/post/{i}">read</a><time>2026-10-28 12:00:00</time></article>'
                for i in range(5)) + '</body></html>').encode()
            self.send_response(200)
            self.send_header('Content-Length', str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        def log_message(self, *_):
            pass

    upstream = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Upstream)
    thread = threading.Thread(target=upstream.serve_forever, daemon=True)
    thread.start()
    process = None
    with tempfile.TemporaryDirectory(prefix='rss-can-load-') as tmp:
        root = pathlib.Path(tmp)
        rules = root / 'rules'
        for i in range(args.rules):
            directory = rules / f'feed{i}'
            directory.mkdir(parents=True)
            config = {'Name': 'Load fixture', 'URL': f'http://127.0.0.1:{upstream.server_port}/news/{i}',
                      'Mode': 'ssr', 'Title': 'h2', 'Link': 'a', 'Expire': 600}
            if args.dates:
                config['DateTime'] = 'time'
            (directory / 'feed.js').write_text('potted.SetScope("article").SetConfig(' + json.dumps(config) + ');')
        with socket.socket() as sock:
            sock.bind(('127.0.0.1', 0))
            port = sock.getsockname()[1]
        base = f'http://127.0.0.1:{port}'
        env = dict(os.environ, RSS_HOST='127.0.0.1', RSS_PORT=str(port), RSS_RULE=str(rules),
                   RSS_REDIS='false', RSS_MEMORY='true', RSS_CONFIG_CACHE_TTL=str(args.config_cache_ttl))
        with (root / 'server.log').open('w+') as log:
            try:
                process = subprocess.Popen([args.binary], env=env, stdout=log, stderr=log)
                deadline = time.monotonic() + 15
                while True:
                    try:
                        opener.open(base + '/_/health/', timeout=.5).close()
                        break
                    except (OSError, urllib.error.URLError):
                        if process.poll() is not None or time.monotonic() > deadline:
                            log.seek(0)
                            raise RuntimeError(log.read())
                        time.sleep(.05)
                if not args.cold:
                    for i in range(args.rules):
                        opener.open(base + f'/feed/feed{i}/rss/', timeout=10).close()
                with lock:
                    counts['upstream'] = 0
                def request(i):
                    start = time.perf_counter()
                    try:
                        with opener.open(base + f'/feed/feed{i % args.rules}/rss/', timeout=12) as response:
                            body = response.read()
                            ok = response.status == 200 and body.count(b'<item>') == 5
                    except (OSError, urllib.error.URLError):
                        ok = False
                    return (time.perf_counter() - start) * 1000, ok
                start = time.perf_counter()
                with concurrent.futures.ThreadPoolExecutor(max_workers=args.concurrency) as pool:
                    results = list(pool.map(request, range(args.requests)))
                elapsed = time.perf_counter() - start
                latencies = sorted(row[0] for row in results)
                try:
                    with opener.open(base + '/_/metrics/', timeout=2) as response:
                        metrics = response.read().decode()
                except (OSError, urllib.error.URLError):
                    metrics = None
                print(json.dumps({'requests': args.requests, 'concurrency': args.concurrency,
                    'rules': args.rules, 'cold': args.cold, 'dates': args.dates,
                    'config_cache_ttl_s': args.config_cache_ttl, 'wall_s': elapsed,
                    'requests_per_s': args.requests / elapsed, 'mean_ms': statistics.mean(latencies),
                    'p50_ms': latencies[(len(latencies)-1)//2], 'p95_ms': latencies[max(0, int(len(latencies)*.95)-1)],
                    'errors': sum(not row[1] for row in results), 'upstream_requests': counts['upstream'],
                    'metrics': metrics}, ensure_ascii=False))
            finally:
                if process is not None:
                    process.terminate()
                    try:
                        process.wait(timeout=5)
                    except subprocess.TimeoutExpired:
                        process.kill()
                        process.wait()
                upstream.shutdown()
                upstream.server_close()
                thread.join()

if __name__ == '__main__':
    main()
