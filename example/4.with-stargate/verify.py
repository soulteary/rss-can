#!/usr/bin/env python3
"""Validate generated policy without printing expanded Compose secrets."""
import argparse
import json
from pathlib import Path
import re
import subprocess
import sys

import setup


def read_env(path):
    result = {}
    for line in path.read_text().splitlines():
        key, sep, value = line.partition("=")
        if not sep or key in result:
            raise ValueError("invalid or duplicate generated environment setting")
        result[key] = value
    return result


def validate_files(directory):
    runtime = directory / ".runtime"
    settings = setup.validate_settings(json.loads((runtime / "settings.json").read_text()))
    if runtime.stat().st_mode & 0o077:
        raise ValueError(".runtime must have private directory permissions")
    for path in [directory / ".env", *runtime.iterdir()]:
        if path.is_symlink() or (path.is_file() and path.stat().st_mode & 0o077):
            raise ValueError("generated configuration and private keys must remain private regular files")
    if (runtime / "nginx.conf").read_text() != setup.render_nginx(settings):
        raise ValueError("Nginx configuration differs from the audited template; regenerate/review it")
    env = read_env(runtime / "rss.env")
    required = {
        "RSS_ACCESS_MODE": "team", "RSS_FETCH_POLICY": "public", "RSS_REDIS": "false",
        "RSS_MEMORY": "true", "RSS_ACCESS_STORE": "/var/lib/rss-can/access.json",
        "RSS_PUBLIC_ORIGIN": "https://" + settings["domain"],
        "RSS_TRUSTED_PROXIES": settings["backend_gateway_ip"] + "/32",
        "RSS_WARDEN_URL": settings["warden_url"], "RSS_HTTP_FEED_PATH": "/feed",
        "RSS_HOST": "0.0.0.0", "RSS_PORT": "8080", "RSS_RULE": "/rules",
    }
    if set(env) != set(required) | {"RSS_ADMIN_TOKEN", "RSS_WARDEN_API_KEY"}:
        raise ValueError("unexpected backend environment override")
    for key, value in required.items():
        if env.get(key) != value:
            raise ValueError("unsafe backend setting: " + key)
    if not re.fullmatch(r"[0-9a-f]{64}", env["RSS_ADMIN_TOKEN"]):
        raise ValueError("admin token is not the generated 256-bit value")
    if len(env["RSS_WARDEN_API_KEY"]) < 16:
        raise ValueError("Warden credential is missing or too short")
    expected = 'proxy_set_header Authorization "Bearer ' + env["RSS_ADMIN_TOKEN"] + '";\n'
    if (runtime / "admin-token.conf").read_text() != expected:
        raise ValueError("operator token does not match the backend")
    compose_env = read_env(directory / ".env")
    for key in ("nginx_image", "backend_subnet", "backend_gateway_ip", "backend_app_ip", "auth_network", "auth_gateway_ip"):
        if compose_env.get(key.upper()) != settings[key]:
            raise ValueError("Compose network/image setting does not match the generated policy")
    return settings, env


def validate_compose(document, settings):
    services = document.get("services", {})
    if set(services) != {"permissions", "rsscan", "gateway"}:
        raise ValueError("unexpected service in minimal deployment")
    for name, service in services.items():
        if service.get("privileged") or service.get("network_mode") == "host":
            raise ValueError("privileged or host networking is not allowed")
        if name != "gateway" and service.get("ports"):
            raise ValueError("backend services must not publish ports")
        if not service.get("read_only"):
            raise ValueError("service root filesystems must be read-only")
        for volume in service.get("volumes", []):
            if "docker.sock" in volume.get("source", ""):
                raise ValueError("Docker control socket must not be mounted")
    gateway, backend = services["gateway"], services["rsscan"]
    if gateway["image"] != settings["nginx_image"]:
        raise ValueError("gateway must use the pinned image")
    ports = {(str(p.get("published")), p.get("target"), p.get("host_ip", "")) for p in gateway.get("ports", [])}
    if ports != {("443", 8443, ""), ("8444", 9443, "127.0.0.1")}:
        raise ValueError("only HTTPS and loopback operator ports may be published")
    if backend.get("user") != "65532:65532" or not backend.get("build"):
        raise ValueError("RSS Can must use the local non-root source build")
    env = backend.get("environment", {})
    if env.get("RSS_ACCESS_MODE") != "team" or env.get("RSS_FETCH_POLICY") != "public" or env.get("RSS_TRUSTED_PROXIES") != settings["backend_gateway_ip"] + "/32":
        raise ValueError("expanded backend configuration weakened policy")
    if not document.get("networks", {}).get("backend", {}).get("internal"):
        raise ValueError("backend network must remain internal")
    if not document.get("networks", {}).get("auth", {}).get("external"):
        raise ValueError("auth network must reuse the existing identity stack")


def compose(directory, args):
    return setup.command(["docker", "compose", "--project-directory", str(directory), "--env-file", str(directory / ".env"), "-f", str(directory / "compose.yml"), *args])


def live_checks(directory, settings):
    compose(directory, ["exec", "-T", "gateway", "nginx", "-t"])
    domain = settings["domain"]
    certificate = directory / ".runtime" / "server.crt"
    def request(path, port=443, headers=(), identity=False):
        command = ["curl", "--silent", "--show-error", "--noproxy", "*", "--max-time", "10", "--cacert", str(certificate), "--resolve", f"{domain}:{port}:127.0.0.1", "-o", "/dev/null", "-w", "%{http_code}"]
        for header in headers:
            command += ["-H", header]
        if identity:
            command += ["--cert", str(directory / ".runtime/operator.crt"), "--key", str(directory / ".runtime/operator.key")]
        command.append(f"https://{domain}:{port}{path}")
        result = subprocess.run(command, capture_output=True, text=True)
        return result.stdout.strip() if result.returncode == 0 else "000"
    for path in ("/api/access/principals", "/config/rules/fresh", "/inspector", "/_stargate_auth"):
        if request(path) != "404":
            raise ValueError("operator/internal route exposed on public listener")
    if request("/api/sources", headers=("X-Auth-User: spoofed", "X-Forwarded-User: authenticated")) != "302":
        raise ValueError("spoofed identity was accepted or request failed unexpectedly")
    fake = "/s/" + "A" * 22 + "/" + "B" * 43 + "/rss"
    if request(fake) != "401":
        raise ValueError("invalid machine credential must return 401 without an SSO redirect")
    if request("/_/metrics/", 8444) not in ("000", "400", "401", "403"):
        raise ValueError("operator listener accepted a request without its client certificate")
    if request("/_/metrics/", 8444, identity=True) != "200":
        raise ValueError("authenticated operator metrics request failed")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--live", action="store_true", help="also check a running stack and its TLS routing")
    args = parser.parse_args()
    try:
        settings, _ = validate_files(setup.HERE)
        document = json.loads(compose(setup.HERE, ["config", "--format", "json"]))
        validate_compose(document, settings)
        runtime = setup.HERE / ".runtime"
        setup.command(["openssl", "verify", "-CAfile", str(runtime / "operator-ca.crt"), str(runtime / "operator.crt")])
        if args.live:
            live_checks(setup.HERE, settings)
        print("Deployment policy checks passed" + (" (including live deny-path checks)." if args.live else "."))
    except (ValueError, OSError, json.JSONDecodeError) as err:
        print("Verification failed: " + str(err), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
