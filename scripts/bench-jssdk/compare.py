#!/usr/bin/env python3
"""Compare prebuilt backend probes. Run from the repository root.

Latency runs are uninstrumented; separate macOS RSS runs sample the complete
process tree at ~1ms intervals using libproc. RSS sums include shared pages
in each process and are not unique physical memory. Short spikes may be missed.
"""
import argparse
import ctypes
import json
import os
import struct
import subprocess
import sys
import time

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--v8', required=True)
parser.add_argument('--deno', required=True)
parser.add_argument('--deno-once', help='optional preserved per-call Deno benchmark binary')
parser.add_argument('--n', type=int, default=210)
parser.add_argument('--rounds', type=int, default=3)
parser.add_argument('--output', required=True)
args = parser.parse_args()
records = []


def run(binary, workload, n, monitor=False):
    process = subprocess.Popen([binary, '-workload', workload, '-n', str(n), '-warmup', '7'], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    peak = 0
    sample_count = 0
    if monitor:
        lib = ctypes.CDLL('/usr/lib/libproc.dylib')
        def tree_rss(pid):
            buffer = ctypes.create_string_buffer(96)
            size = lib.proc_pidinfo(pid, 4, 0, buffer, 96)  # PROC_PIDTASKINFO
            own = struct.unpack_from('Q', buffer.raw, 8)[0] if size == 96 else 0
            children = (ctypes.c_int * 128)()
            child_count = lib.proc_listchildpids(pid, children, ctypes.sizeof(children))
            return own + sum(tree_rss(children[i]) for i in range(max(0, child_count)) if children[i] > 0)
        while process.poll() is None:
            peak = max(peak, tree_rss(process.pid))
            sample_count += 1
            time.sleep(.001)
    stdout, stderr = process.communicate()
    if process.returncode:
        raise RuntimeError(f'{binary} {workload}: {stderr}')
    result = json.loads(stdout)
    if monitor:
        if not peak:
            raise RuntimeError('No RSS samples: libproc access unavailable')
        result.update(tree_peak_rss_mib=peak / 1024**2, rss_samples=sample_count)
    return result


for round_number in range(args.rounds):
    # Alternate backend order to reduce systematic ordering bias.
    backends = [('v8go', args.v8), ('deno', args.deno)]
    if args.deno_once:
        backends.append(('deno-per-call', args.deno_once))
    if round_number % 2:
        backends.reverse()
    for workload in ['minimal', 'rules', 'date']:
        for name, binary in backends:
            record = run(binary, workload, args.n)
            record.update(backend=name, round=round_number + 1, kind='latency')
            records.append(record)
            print(f'{name} {workload} round {round_number+1}: {record["mean_ms"]:.3f} ms', file=sys.stderr)
    record = run(args.deno, 'deno-startup', args.n)
    record.update(backend='deno', round=round_number + 1, kind='latency')
    records.append(record)

if sys.platform == 'darwin':
    memory_backends = [('v8go', args.v8), ('deno', args.deno)]
    if args.deno_once:
        memory_backends.append(('deno-per-call', args.deno_once))
    for name, binary in memory_backends:
        for workload in ['minimal', 'rules', 'date']:
            record = run(binary, workload, 70, monitor=True)
            record.update(backend=name, kind='memory')
            records.append(record)
            print(f'{name} {workload} tree RSS: {record["tree_peak_rss_mib"]:.2f} MiB', file=sys.stderr)

with open(args.output, 'w') as output:
    json.dump({'platform': sys.platform, 'deno_path': os.environ.get('RSS_CAN_DENO', 'deno'), 'records': records}, output, indent=2)
    output.write('\n')
