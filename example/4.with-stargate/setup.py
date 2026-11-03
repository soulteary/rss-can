#!/usr/bin/env python3
"""Render a fail-closed connection to an existing Stargate/Warden deployment."""
import argparse
import ipaddress
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import subprocess
import sys
from urllib.parse import urlsplit

HERE = Path(__file__).resolve().parent
DIGEST = re.compile(r"(?:docker\.io/library/)?nginx@sha256:[0-9a-f]{64}\Z")
DOMAIN = re.compile(r"(?=.{1,253}\Z)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\Z")


def command(args, **kwargs):
    result = subprocess.run(args, capture_output=True, text=True, **kwargs)
    if result.returncode:
        # Do not echo subprocess output or command values: configuration and
        # secrets must not end up in setup logs.
        raise ValueError(f"{args[0]} failed; check its installation and the supplied files")
    return result.stdout


def validate_settings(s):
    for key in ("auth_origin", "stargate_upstream", "warden_url"):
        if any(c.isspace() or ord(c) < 33 or ord(c) == 127 for c in s[key]):
            raise ValueError("service URLs must not contain whitespace or control characters")
    if not DOMAIN.fullmatch(s["domain"]):
        raise ValueError("domain must be a lowercase DNS name without a port")
    auth = urlsplit(s["auth_origin"])
    if auth.scheme != "https" or not auth.hostname or auth.username or auth.password or auth.path or auth.query or auth.fragment or auth.port is not None:
        raise ValueError("auth-origin must be an HTTPS origin on port 443")
    if not DOMAIN.fullmatch(auth.hostname) or auth.hostname == s["domain"]:
        raise ValueError("auth-origin must use a separate valid DNS name")
    upstream = urlsplit(s["stargate_upstream"])
    if upstream.scheme != "http" or not upstream.hostname or upstream.username or upstream.password or upstream.path or upstream.query or upstream.fragment:
        raise ValueError("stargate-upstream must be an internal HTTP origin")
    if not re.fullmatch(r"[a-zA-Z0-9._-]+", upstream.hostname) or not upstream.port:
        raise ValueError("stargate-upstream requires an explicit container/service port")
    warden = urlsplit(s["warden_url"])
    if warden.scheme not in ("http", "https") or not warden.hostname or warden.username or warden.password or warden.path or warden.query or warden.fragment:
        raise ValueError("warden-url must be an HTTP(S) origin without credentials")
    if warden.port is not None and not 1 <= warden.port <= 65535:
        raise ValueError("warden-url has an invalid port")
    if not re.fullmatch(r"[A-Za-z0-9_.-]+", s["auth_network"]):
        raise ValueError("auth-network must be an existing Docker network name")
    backend = ipaddress.ip_network(s["backend_subnet"])
    network = ipaddress.ip_network(s["auth_network_cidr"])
    address = ipaddress.ip_address(s["auth_gateway_ip"])
    if backend.version != 4 or backend.prefixlen != 28 or not backend.is_private:
        raise ValueError("backend-subnet must be a private IPv4 /28")
    if network.version != 4 or address not in network or address in (network.network_address, network.broadcast_address) or network.overlaps(backend):
        raise ValueError("auth gateway IP/subnet is invalid or overlaps the private backend")
    if not DIGEST.fullmatch(s["nginx_image"]):
        raise ValueError("nginx-image must pin the official image by sha256 digest")
    s["backend_gateway_ip"] = str(backend.network_address + 2)
    s["backend_app_ip"] = str(backend.network_address + 3)
    s["auth_host"] = auth.hostname
    s["stargate_address"] = upstream.netloc
    return s


def render_nginx(settings):
    result = (HERE / "nginx.conf.template").read_text()
    for key in ("domain", "auth_host", "auth_origin", "stargate_address", "backend_app_ip"):
        token = "RSS_DOMAIN" if key == "domain" else key.upper()
        result = result.replace("@@" + token + "@@", settings[key])
    if "@@" in result:
        raise ValueError("unexpanded Nginx placeholder")
    return result


def write_private(path, value):
    path.write_text(value)
    path.chmod(0o600)


def discover_network(name):
    data = json.loads(command(["docker", "network", "inspect", name]))[0]
    candidates = [ipaddress.ip_network(item["Subnet"]) for item in data.get("IPAM", {}).get("Config", []) if ":" not in item.get("Subnet", "")]
    if not candidates:
        raise ValueError("existing auth network needs an IPv4 subnet")
    network = candidates[0]
    used = {ipaddress.ip_address(item["IPv4Address"].split("/")[0]) for item in (data.get("Containers") or {}).values() if item.get("IPv4Address")}
    used.update(ipaddress.ip_address(item["Gateway"]) for item in data.get("IPAM", {}).get("Config", []) if item.get("Gateway") and ":" not in item["Gateway"])
    for offset in range(1, min(network.num_addresses - 1, 1024)):
        address = network.broadcast_address - offset
        if address not in used:
            return str(network), str(address)
    raise ValueError("no free gateway address found in auth network")


def pin_nginx():
    command(["docker", "pull", "nginx:stable-alpine"])
    data = json.loads(command(["docker", "image", "inspect", "nginx:stable-alpine"]))[0]
    for digest in data.get("RepoDigests", []):
        if DIGEST.fullmatch(digest):
            return digest
    raise ValueError("Docker did not return an official Nginx digest")


def create_operator_identity(runtime):
    ca_key, ca_cert = runtime / "operator-ca.key", runtime / "operator-ca.crt"
    key, csr, cert = runtime / "operator.key", runtime / "operator.csr", runtime / "operator.crt"
    command(["openssl", "ecparam", "-name", "prime256v1", "-genkey", "-noout", "-out", str(ca_key)])
    command(["openssl", "req", "-new", "-x509", "-sha256", "-days", "3650", "-key", str(ca_key), "-out", str(ca_cert), "-subj", "/CN=RSS Can operator CA", "-addext", "basicConstraints=critical,CA:TRUE", "-addext", "keyUsage=critical,keyCertSign,cRLSign"])
    command(["openssl", "ecparam", "-name", "prime256v1", "-genkey", "-noout", "-out", str(key)])
    command(["openssl", "req", "-new", "-sha256", "-key", str(key), "-out", str(csr), "-subj", "/CN=RSS Can operator"])
    extensions = runtime / "operator.ext"
    write_private(extensions, "basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=clientAuth\n")
    command(["openssl", "x509", "-req", "-sha256", "-days", "365", "-in", str(csr), "-CA", str(ca_cert), "-CAkey", str(ca_key), "-CAcreateserial", "-extfile", str(extensions), "-out", str(cert)])
    csr.unlink()
    extensions.unlink()
    for path in runtime.glob("operator*"):
        path.chmod(0o600)


def render(settings, cert, key, warden_key, directory=HERE):
    settings = validate_settings({k: settings[k] for k in ("domain", "auth_origin", "stargate_upstream", "warden_url", "auth_network", "backend_subnet", "auth_network_cidr", "auth_gateway_ip", "nginx_image")})
    if len(warden_key) < 16 or any(ord(c) < 33 or ord(c) > 126 for c in warden_key):
        raise ValueError("Warden key must contain at least 16 printable non-whitespace characters")
    runtime = directory / ".runtime"
    if runtime.exists() or (directory / ".env").exists():
        raise ValueError("configuration already exists; refuse to overwrite identity or secrets")
    command(["openssl", "x509", "-in", str(cert), "-noout", "-checkhost", settings["domain"]])
    cert_public = command(["openssl", "x509", "-in", str(cert), "-pubkey", "-noout"])
    key_public = command(["openssl", "pkey", "-in", str(key), "-pubout", "-passin", "pass:"])
    if cert_public != key_public:
        raise ValueError("TLS certificate does not match its private key")
    runtime.mkdir(mode=0o700)
    token = secrets.token_hex(32)
    try:
        shutil.copyfile(cert, runtime / "server.crt")
        shutil.copyfile(key, runtime / "server.key")
        create_operator_identity(runtime)
        write_private(runtime / "nginx.conf", render_nginx(settings))
        write_private(runtime / "admin-token.conf", f'proxy_set_header Authorization "Bearer {token}";\n')
        env = {
            "RSS_ACCESS_MODE": "team", "RSS_ACCESS_STORE": "/var/lib/rss-can/access.json",
            "RSS_PUBLIC_ORIGIN": "https://" + settings["domain"],
            "RSS_TRUSTED_PROXIES": settings["backend_gateway_ip"] + "/32",
            "RSS_WARDEN_URL": settings["warden_url"], "RSS_WARDEN_API_KEY": warden_key,
            "RSS_ADMIN_TOKEN": token, "RSS_FETCH_POLICY": "public", "RSS_REDIS": "false",
            "RSS_MEMORY": "true", "RSS_HOST": "0.0.0.0", "RSS_PORT": "8080", "RSS_HTTP_FEED_PATH": "/feed", "RSS_RULE": "/rules",
        }
        write_private(runtime / "rss.env", "".join(k + "=" + v + "\n" for k, v in env.items()))
        write_private(runtime / "settings.json", json.dumps(settings, indent=2) + "\n")
        write_private(directory / ".env", "".join(k + "=" + v + "\n" for k, v in {
            "NGINX_IMAGE": settings["nginx_image"], "BACKEND_SUBNET": settings["backend_subnet"],
            "BACKEND_GATEWAY_IP": settings["backend_gateway_ip"], "BACKEND_APP_IP": settings["backend_app_ip"],
            "AUTH_NETWORK": settings["auth_network"], "AUTH_GATEWAY_IP": settings["auth_gateway_ip"],
        }.items()))
        for path in runtime.iterdir():
            if path.is_file():
                path.chmod(0o600)
    except Exception:
        # Leave evidence for the operator; never silently retry with a new CA.
        raise ValueError("setup incomplete; inspect the private .runtime directory before retrying") from None
    return settings


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--domain", required=True)
    parser.add_argument("--auth-origin", required=True)
    parser.add_argument("--stargate-upstream", required=True)
    parser.add_argument("--warden-url", required=True)
    parser.add_argument("--warden-key-file", type=Path, required=True)
    parser.add_argument("--network", dest="auth_network", required=True)
    parser.add_argument("--cert", type=Path, required=True)
    parser.add_argument("--key", type=Path, required=True)
    parser.add_argument("--backend-subnet", default="172.30.253.0/28")
    parser.add_argument("--nginx-image", help="optional pre-resolved official nginx@sha256 digest")
    parser.add_argument("--auth-network-cidr", help="offline rendering only; specify with --auth-gateway-ip")
    parser.add_argument("--auth-gateway-ip", help="offline rendering only; operator must reserve this address")
    args = parser.parse_args()
    try:
        if bool(args.auth_network_cidr) != bool(args.auth_gateway_ip):
            raise ValueError("offline auth subnet and gateway IP must be supplied together")
        if not args.auth_gateway_ip:
            args.auth_network_cidr, args.auth_gateway_ip = discover_network(args.auth_network)
        if not args.nginx_image:
            args.nginx_image = pin_nginx()
        for path in (args.key,args.warden_key_file):
            if path.stat().st_mode & 0o077:
                raise ValueError("private key and Warden key files must not be group/world accessible")
        settings = render(vars(args), args.cert, args.key, args.warden_key_file.read_text().strip())
        print("Private configuration created. No secrets were printed.")
        print("Merge into existing Stargate: CALLBACK_ALLOWED_HOSTS += " + settings["domain"])
        print("Merge into existing Stargate: TRUSTED_PROXIES += " + settings["auth_gateway_ip"] + "/32")
        print("Then run: python3 verify.py && docker compose -f compose.yml up -d --build")
    except (ValueError, OSError, json.JSONDecodeError) as err:
        print("Setup failed: " + str(err), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    os.umask(0o077)
    raise SystemExit(main())
