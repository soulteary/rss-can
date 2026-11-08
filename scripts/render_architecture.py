#!/usr/bin/env python3
"""Render the Fireworks style-6 architecture and custom call-direction GIF.

Requires Python packages resvg-py==0.5.0 and pillow==13.0.0.
Pass --fireworks-root pointing to fireworks-tech-graph commit
1ca1ff9cfcb67893a43d32b9ef3a1b782c44a46f (or a compatible checkout).
The motion is a direction illustration, not a measured execution trace.
"""
import argparse
import io
import json
from pathlib import Path
import re
import subprocess
import sys
import xml.etree.ElementTree as ET

import resvg_py
from PIL import Image, ImageDraw


def main():
    cli = argparse.ArgumentParser(description=__doc__)
    cli.add_argument('--fireworks-root', type=Path, required=True)
    args = cli.parse_args()
    root = Path(__file__).resolve().parents[1]
    base = root / 'docs/images/rss-can-architecture'
    tool = args.fireworks_root / 'scripts/fireworks.py'
    subprocess.run([sys.executable, str(tool), 'render', 'architecture',
                    str(base.with_suffix('.json')), str(base.with_suffix('.svg'))], check=True)
    subprocess.run([sys.executable, str(tool), 'check', str(base.with_suffix('.svg'))], check=True)
    svg = base.with_suffix('.svg').read_text()
    base.with_suffix('.png').write_bytes(resvg_py.svg_to_bytes(svg_string=svg, width=1920))
    width = 1520
    background = Image.open(io.BytesIO(resvg_py.svg_to_bytes(svg_string=svg, width=width))).convert('RGB')
    edges = []
    for el in ET.fromstring(svg).iter():
        if el.get('data-graph-role') != 'edge':
            continue
        match = re.fullmatch(r'M ([\d.]+),([\d.]+) L ([\d.]+),([\d.]+)', el.get('d', ''))
        if not match:
            raise ValueError('Custom animation requires straight connectors; review new topology.')
        edges.append((el.get('id'), tuple(map(float, match.groups())), el.get('stroke')))
    frames = []
    for frame in range(120):
        im = background.copy()
        draw = ImageDraw.Draw(im)
        for index, (_, (x1, y1, x2, y2), color) in enumerate(edges):
            # The full topology stays visible; markers follow the source arrow direction.
            progress = ((frame + index * 9) % 40) / 40
            x, y = x1 + (x2-x1)*progress, y1 + (y2-y1)*progress
            if min(progress, 1-progress) > .08:
                draw.ellipse((x-4, y-4, x+4, y+4), fill=color, outline='#f8f6f3', width=1)
        frames.append(im)
    palette = background.quantize(colors=240)
    frames = [im.quantize(palette=palette, dither=Image.Dither.NONE) for im in frames]
    frames[0].save(base.with_suffix('.gif'), save_all=True, append_images=frames[1:],
                   duration=50, loop=0, optimize=True, disposal=1)
    with Image.open(base.with_suffix('.gif')) as gif:
        durations = []
        for i in range(gif.n_frames):
            gif.seek(i)
            durations.append(gif.info['duration'])
        assert gif.size == (1520, 880) and gif.n_frames == 120
        assert sum(durations) == 6000 and gif.info['loop'] == 0
    print(json.dumps({'png': [1920, 1112], 'gif': [1520, 880], 'frames': 120,
                      'duration_ms': sum(durations), 'bytes': base.with_suffix('.gif').stat().st_size}))


if __name__ == '__main__':
    main()
