import copy
import fnmatch
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
import unittest
import uuid

import setup
import verify


class DeploymentTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.directory = Path(self.tmp.name)
        self.cert = self.directory / "input.crt"
        self.key = self.directory / "input.key"
        setup.command(["openssl", "req", "-x509", "-newkey", "ec", "-pkeyopt", "ec_paramgen_curve:prime256v1", "-nodes", "-keyout", str(self.key), "-out", str(self.cert), "-days", "1", "-subj", "/CN=rss.test.example", "-addext", "subjectAltName=DNS:rss.test.example"])
        self.key.chmod(0o600)
        self.settings = {
            "domain": "rss.test.example", "auth_origin": "https://auth.test.example",
            "stargate_upstream": "http://stargate:8080", "warden_url": "http://warden:8081",
            "auth_network": "existing-auth", "backend_subnet": "172.30.253.0/28",
            "auth_network_cidr": "172.29.0.0/24", "auth_gateway_ip": "172.29.0.250",
            # Configuration fixture, not a downloadable image claim.
            "nginx_image": "nginx@sha256:" + "a" * 64,
        }
        shutil.copyfile(setup.HERE / "compose.yml", self.directory / "compose.yml")

    def render(self):
        return setup.render(self.settings, self.cert, self.key, "test-only-warden-secret", self.directory)

    def test_render_and_real_compose_config(self):
        self.render()
        settings, env = verify.validate_files(self.directory)
        self.assertEqual(len(env["RSS_ADMIN_TOKEN"]), 64)
        self.assertEqual(env["RSS_FETCH_POLICY"], "public")
        output = verify.compose(self.directory, ["config", "--format", "json"])
        document = json.loads(output)
        verify.validate_compose(document, settings)
        conf = (self.directory / ".runtime/nginx.conf").read_text()
        public, operator = conf.split("listen 9443 ssl;")
        self.assertNotIn("include /etc/rss-admin-token.conf", public)
        self.assertIn("ssl_verify_client on", operator)
        self.assertNotIn(env["RSS_ADMIN_TOKEN"], conf)
        self.assertIn("proxy_pass_request_headers off", public)
        self.assertIn("proxy_set_header X-RSS-Admin-Action $http_x_rss_admin_action", public)
        self.assertNotIn("proxy_set_header X-RSS-Admin-Action 1", public)
        self.assertIn("limit_req_zone $binary_remote_addr", public)
        self.assertIn("limit_conn rss_connections 32", public)
        self.assertIn("limit_req_status 429", public)
        self.assertIn("burst=100", public)
        self.assertNotIn("test-only-warden-secret", (self.directory / ".runtime/settings.json").read_text())

    def test_rejects_security_downgrades(self):
        self.render()
        settings, _ = verify.validate_files(self.directory)
        document = json.loads(verify.compose(self.directory, ["config", "--format", "json"]))
        changed = copy.deepcopy(document)
        changed["services"]["rsscan"]["ports"] = [{"target": 8080, "published": "8080"}]
        with self.assertRaises(ValueError):
            verify.validate_compose(changed, settings)
        changed = copy.deepcopy(document)
        changed["services"]["gateway"]["ports"][1]["host_ip"] = "0.0.0.0"
        with self.assertRaises(ValueError):
            verify.validate_compose(changed, settings)
        env = self.directory / ".runtime/rss.env"
        env.write_text(env.read_text().replace("RSS_ACCESS_MODE=team", "RSS_ACCESS_MODE=standalone"))
        with self.assertRaises(ValueError):
            verify.validate_files(self.directory)

    def test_rejects_tampering_permissions_and_overwrite(self):
        self.render()
        with self.assertRaises(ValueError):
            self.render()
        conf = self.directory / ".runtime/nginx.conf"
        original = conf.read_text()
        conf.write_text(original.replace("ssl_verify_client on", "ssl_verify_client off"))
        with self.assertRaises(ValueError):
            verify.validate_files(self.directory)
        conf.write_text(original)
        (self.directory / ".runtime/rss.env").chmod(0o644)
        with self.assertRaises(ValueError):
            verify.validate_files(self.directory)

    def test_rejects_injection_unpinned_images_and_overlap(self):
        for key, value in (
            ("domain", "rss.test.example; return 200"), ("auth_origin", "http://auth.test.example"),
            ("auth_origin", "https://rss.test.example"), ("stargate_upstream", "http://stargate:8080/evil"),
            ("nginx_image", "nginx:latest"), ("auth_network", "auth\nunsafe"),
            ("warden_url", "http://warden:8081\nRSS_ACCESS_MODE=standalone"),
            ("auth_gateway_ip", "172.29.1.250"), ("backend_subnet", "172.29.0.0/28"),
        ):
            with self.subTest(key=key, value=value):
                changed = dict(self.settings)
                changed[key] = value
                with self.assertRaises(ValueError):
                    setup.validate_settings(changed)

    def test_documented_cli_help_from_example_directory(self):
        # Importing stdlib operator must never resolve to a local operator.py.
        # Use a fresh interpreter because this process has already imported it.
        for script in ("setup.py", "rss_operator.py"):
            with self.subTest(script=script):
                result = subprocess.run([sys.executable, script, "--help"],
                                        cwd=setup.HERE, capture_output=True,
                                        text=True, timeout=10, check=True)
                self.assertIn("usage:", result.stdout)

    def test_builder_context_excludes_deployment_secrets(self):
        patterns = (setup.HERE.parent.parent / ".dockerignore").read_text().splitlines()
        def excluded(path):
            value = False
            for pattern in patterns:
                if not pattern or pattern.startswith("#"):
                    continue
                negative = pattern.startswith("!")
                if fnmatch.fnmatchcase(path, pattern.lstrip("!")):
                    value = not negative
            return value
        for path in (".env", ".env.team", ".env.production", ".runtime/server.key", "example/4.with-stargate/.env", "example/4.with-stargate/.runtime/operator-ca.key", "example/4.with-stargate/.env.production", "example/4.with-stargate/.env.team"):
            with self.subTest(path=path):
                self.assertTrue(excluded(path), "private deployment files would enter the build context")
        self.assertFalse(excluded("example/4.with-stargate/.env.example"))
        self.assertFalse(excluded("example/4.with-stargate/README.md"))

    def test_manual_team_credentials_are_git_ignored(self):
        root = setup.HERE.parent.parent
        paths = [".env.team", "example/4.with-stargate/.env.team"]
        result = subprocess.run(["git", "check-ignore", "--no-index", *paths],
                                cwd=root, capture_output=True, text=True, check=True)
        self.assertEqual(set(result.stdout.splitlines()), set(paths))

    @unittest.skipUnless(os.environ.get("RSS_TEST_DOCKER") == "1", "set RSS_TEST_DOCKER=1 for the real Docker context check")
    def test_real_builder_context_excludes_deployment_secrets(self):
        context = self.directory / "context"
        sample = context / "example/4.with-stargate"
        (sample / ".runtime").mkdir(parents=True)
        (sample / ".runtime/test-secret").write_text("harmless exclusion marker")
        (sample / ".env").write_text("harmless exclusion marker")
        (sample / ".env.team").write_text("harmless exclusion marker")
        (context / ".env.team").write_text("harmless exclusion marker")
        (sample / "README.md").write_text("public control file")
        shutil.copyfile(setup.HERE.parent.parent / ".dockerignore", context / ".dockerignore")
        (context / "Dockerfile").write_text("FROM scratch\nCOPY . /context\n")
        tag = "rss-can-context-check:" + uuid.uuid4().hex
        container = None
        try:
            setup.command(["docker", "build", "-t", tag, str(context)])
            container = setup.command(["docker", "create", tag, "/not-executed"]).strip()
            archive = subprocess.run(["docker", "export", container], capture_output=True, check=True).stdout
            with tarfile.open(fileobj=io.BytesIO(archive)) as tar:
                names = set(tar.getnames())
            self.assertIn("context/example/4.with-stargate/README.md", names)
            self.assertNotIn("context/example/4.with-stargate/.env", names)
            self.assertNotIn("context/.env.team", names)
            self.assertNotIn("context/example/4.with-stargate/.env.team", names)
            self.assertFalse(any("/.runtime" in name for name in names))
        finally:
            if container:
                subprocess.run(["docker", "rm", container], capture_output=True)
            subprocess.run(["docker", "image", "rm", tag], capture_output=True)

    @unittest.skipUnless(os.environ.get("RSS_TEST_NGINX_IMAGE"), "set RSS_TEST_NGINX_IMAGE to a verified official digest for live proxy tests")
    def test_live_nginx_identity_operator_and_rate_boundaries(self):
        self.settings["nginx_image"] = os.environ["RSS_TEST_NGINX_IMAGE"]
        self.settings["stargate_upstream"] = "http://127.0.0.1:8080"
        self.render()
        runtime = self.directory / ".runtime"
        # Stub only the upstream contracts. The gateway, TLS, mTLS, header
        # allowlist and rate/connection policies are the real Nginx template.
        conf = (runtime / "nginx.conf").read_text().replace("server 172.30.253.3:8080;", "server 127.0.0.1:8081;")
        stub = '''
        server { listen 127.0.0.1:8080;
          location /_auth {
            if ($http_accept != "application/json") { return 400; }
            if ($http_x_auth_user != "") { return 403; }
            if ($http_stargate_password != "") { return 403; }
            if ($http_cookie != "session=valid") { return 401; }
            add_header X-Auth-User alice;
            return 204;
          }
        }
        server { listen 127.0.0.1:8081;
          location /s/ { return 401; }
          location / {
            default_type application/json;
            return 200 '{"subject":"$http_x_auth_user","authorization":"$http_authorization","action":"$http_x_rss_admin_action"}';
          }
        }
        '''
        (runtime / "nginx.conf").write_text(conf.rsplit("}", 1)[0] + stub + "}\n")
        mounts = {"nginx.conf": "/etc/nginx/nginx.conf", "admin-token.conf": "/etc/rss-admin-token.conf", "server.crt": "/etc/rss-server.crt", "server.key": "/etc/rss-server.key", "operator-ca.crt": "/etc/rss-operator-ca.crt"}
        args = ["docker", "run", "-d", "--rm", "--read-only", "--cap-drop", "ALL", "--tmpfs", "/tmp:rw,noexec,nosuid,mode=1777,size=32m", "--security-opt", "no-new-privileges:true", "-p", "127.0.0.1::8443", "-p", "127.0.0.1::9443", "--entrypoint", "nginx"]
        for cap in ("CHOWN", "SETUID", "SETGID", "DAC_OVERRIDE"):
            args += ["--cap-add", cap]
        for source, target in mounts.items():
            args += ["-v", f"{runtime / source}:{target}:ro"]
        args += [self.settings["nginx_image"], "-g", "daemon off;"]
        container = setup.command(args).strip()
        try:
            info = json.loads(setup.command(["docker", "inspect", container]))[0]
            ports = info["NetworkSettings"]["Ports"]
            public = ports["8443/tcp"][0]["HostPort"]
            operator = ports["9443/tcp"][0]["HostPort"]
            def request(path, headers=(), port=public, identity=False):
                args = ["curl", "--silent", "--show-error", "--noproxy", "*", "--max-time", "3", "--cacert", str(runtime / "server.crt"), "--resolve", f"rss.test.example:{port}:127.0.0.1", "-w", "\n%{http_code}"]
                if identity:
                    args += ["--cert", str(runtime / "operator.crt"), "--key", str(runtime / "operator.key")]
                for header in headers:
                    args += ["-H", header]
                result = subprocess.run(args + [f"https://rss.test.example:{port}{path}"], capture_output=True, text=True)
                if result.returncode:
                    return "000", ""
                body, _, code = result.stdout.rpartition("\n")
                return code, body
            deadline = time.monotonic() + 10
            while request("/config/rules/fresh")[0] == "000" and time.monotonic() < deadline:
                time.sleep(.1)
            self.assertEqual(request("/config/rules/fresh")[0], "404")
            self.assertEqual(request("/api/access/principals")[0], "404")
            self.assertEqual(request("/_stargate_auth")[0], "404")
            self.assertEqual(request("/api/sources", ("X-Auth-User: forged", "Stargate-Password: shortcut"))[0], "302")
            code, body = request("/api/sources", ("Cookie: session=valid", "X-Auth-User: forged", "Authorization: Bearer forged", "X-RSS-Admin-Action: 1"))
            self.assertEqual(code, "200")
            received = json.loads(body)
            self.assertEqual(received, {"subject": "alice", "authorization": "", "action": "1"})
            fake = "/s/" + "A" * 22 + "/" + "B" * 43 + "/rss"
            self.assertEqual(request(fake)[0], "401")
            self.assertIn(request("/api/access/principals", port=operator)[0], ("000", "400", "401", "403"))
            code, body = request("/api/access/principals", port=operator, identity=True)
            self.assertEqual(code, "200")
            received = json.loads(body)
            token = verify.read_env(runtime / "rss.env")["RSS_ADMIN_TOKEN"]
            self.assertTrue(received["authorization"] == "Bearer " + token, "operator token injection failed")
            codes = [request("/api/sources", ("Cookie: session=valid",))[0] for _ in range(60)]
            self.assertIn("429", codes)
        finally:
            subprocess.run(["docker", "stop", container], capture_output=True)


if __name__ == "__main__":
    os.umask(0o077)
    unittest.main()
