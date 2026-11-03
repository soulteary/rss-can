#!/usr/bin/env python3
"""Call a loopback-only operator route with the generated client identity."""
import argparse
import json
import subprocess
import sys

import setup


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("path", help="operator path, e.g. /api/access/principals")
    parser.add_argument("--data", help="JSON request body; sends POST when present")
    args = parser.parse_args()
    if not args.path.startswith(("/api/access/", "/config/", "/_/")) or "?" in args.path or any(c.isspace() for c in args.path):
        parser.error("only operator paths without query strings are allowed")
    runtime = setup.HERE / ".runtime"
    settings = json.loads((runtime / "settings.json").read_text())
    domain = settings["domain"]
    command = ["curl", "--silent", "--show-error", "--fail-with-body", "--noproxy", "*", "--max-time", "35", "--cacert", str(runtime / "server.crt"), "--cert", str(runtime / "operator.crt"), "--key", str(runtime / "operator.key"), "--resolve", f"{domain}:8444:127.0.0.1", "-H", "X-RSS-Admin-Action: 1"]
    if args.data is not None:
        json.loads(args.data)
        command += ["-H", "Content-Type: application/json", "--data-binary", args.data]
    command.append(f"https://{domain}:8444{args.path}")
    return subprocess.run(command).returncode


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError) as err:
        print("Operator request failed: " + str(err), file=sys.stderr)
        raise SystemExit(1)
