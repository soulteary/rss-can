#!/usr/bin/env python3
"""macOS sustained mixed-workload process-tree RSS sampling; run from repo root."""
import argparse
import ctypes
import json
import statistics
import struct
import subprocess
import time

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--binary', required=True)
parser.add_argument('--label', required=True)
parser.add_argument('--duration', type=int, default=180)
parser.add_argument('--idle', type=int, default=30)
parser.add_argument('--interval', default='5ms')
parser.add_argument('--output', required=True)
args = parser.parse_args()
if args.duration < 60 or args.idle < 10:
    parser.error('duration must be >=60s and idle >=10s for stable summary windows')
lib = ctypes.CDLL('/usr/lib/libproc.dylib')


def tree(pid):
    buffer = ctypes.create_string_buffer(96)
    size = lib.proc_pidinfo(pid, 4, 0, buffer, 96)
    own = struct.unpack_from('Q', buffer.raw, 8)[0] / 1024**2 if size == 96 else 0
    children = (ctypes.c_int * 128)()
    count = lib.proc_listchildpids(pid, children, ctypes.sizeof(children))
    descendants = [children[i] for i in range(max(0, count)) if children[i] > 0]
    return own, sum(sum(tree(child)[:2]) for child in descendants), descendants


process = subprocess.Popen([args.binary, '-workload', 'mixed', '-duration', f'{args.duration}s', '-idle', f'{args.idle}s', '-interval', args.interval, '-warmup', '7'], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
start = time.monotonic()
samples = []
next_progress = 30
while process.poll() is None:
    own, children, pids = tree(process.pid)
    samples.append({'s': time.monotonic()-start, 'go_rss_mib': own, 'child_rss_mib': children, 'tree_rss_mib': own+children, 'child_pids': pids})
    if samples[-1]['s'] >= next_progress:
        print(f"{args.label}: {samples[-1]['s']:.0f}s, tree RSS {own+children:.2f} MiB", flush=True)
        next_progress += 30
    time.sleep(.1)
stdout, stderr = process.communicate()
if process.returncode:
    raise RuntimeError(stderr)
result = json.loads(stdout)
# Exclude startup/exit transients from plateau windows; timings are approximate
# because process launch/7 warmup calls precede the sustained-loop timer.
windows = {}
for name, lower, upper in [('early_active', 10, 40), ('late_active', args.duration-30, args.duration-1), ('late_idle', args.duration+5, args.duration+args.idle-1)]:
    values = [s['tree_rss_mib'] for s in samples if lower <= s['s'] <= upper]
    windows[name] = {'mean_mib': statistics.mean(values), 'min_mib': min(values), 'max_mib': max(values)}
summary = {'peak_tree_rss_mib': max(s['tree_rss_mib'] for s in samples), 'windows': windows, 'worker_pids': sorted({pid for sample in samples for pid in sample['child_pids']})}
with open(args.output, 'w') as output:
    json.dump({'label': args.label, 'duration_s': args.duration, 'idle_s': args.idle, 'sample_interval_s': .1, 'result': result, 'summary': summary, 'samples': samples}, output, indent=2)
    output.write('\n')
print(json.dumps({'label': args.label, 'result': result, 'summary': summary}))
