# RSS Can behind an existing Stargate and Warden

This example adds a locally built RSS Can and an HTTPS Nginx gateway to an
**existing** identity stack. It does not recreate users, deploy a second Warden,
or require Herald when your existing login method does not use it. Use a
Stargate login that produces a stable Warden user ID; shared-password-only
sessions cannot represent distinct RSS Can users.

The ordinary flow is: open `/sources`, sign in through Stargate, add a source,
then copy its reader URL. Private feed downloads use their own revocable token;
they do not depend on browser cookies or redirect a reader to HTML login.

## Prerequisites

- Docker Compose 2.30+ (the example uses raw env files), Python 3.9+, OpenSSL and
  curl. Run the setup from this directory in a checkout containing the new team
  access/source features (v0.5.0 or later). This example deliberately builds the
  checkout locally instead of using a published RSS Can image. Historical
  v0.4.0 ignores team configuration and must not be substituted.
- An existing Docker network shared by reachable Stargate and Warden services.
  Do not attach untrusted workloads to this identity network.
- A DNS name for RSS Can and a valid TLS certificate/private key for that name.
  Point DNS at this host. The key and Warden API-key file must be private (0600).
- The public HTTPS origin of your existing Stargate login, its internal HTTP
  service address, and a dedicated Warden API key. Preserve the existing
  Stargate/Warden data, session storage and authentication settings.

Binary release archives include these deployment helpers, but not the complete
Go source needed by the Compose build. If you started from an archive, obtain
the matching source checkout first:

```sh
git clone --branch v0.5.1 --depth 1 https://github.com/soulteary/rss-can.git
cd rss-can/example/4.with-stargate
```

Only port 443 is public. The separate operator port binds to `127.0.0.1:8444`
and requires a generated client certificate. RSS Can has no published port;
there is no Redis or browser/CDP service in this SSR-only team deployment.

## 1. Generate the configuration

Prepare the Warden key in a private file without pasting it into command-line
arguments or source control. Then run:

```sh
python3 setup.py \
  --domain rss.example.com \
  --auth-origin https://auth.example.com \
  --stargate-upstream http://stargate:8080 \
  --warden-url http://warden:8081 \
  --warden-key-file /secure/rss-warden.key \
  --network stargate-traefik \
  --cert /secure/rss-fullchain.pem \
  --key /secure/rss-privkey.pem
```

The script inspects that existing network and chooses a currently unused static
gateway address. It pulls the official `nginx:stable-alpine` image and records
the resolved `nginx@sha256:...` digest; Compose never runs an unpinned Nginx tag.
To render offline, supply a verified `--nginx-image nginx@sha256:...`,
`--auth-network-cidr ...` and reserved `--auth-gateway-ip ...` together. Offline
rendering does not prove that the address is free or that the upstream is live.
If the private `172.30.253.0/28` backend conflicts with a host network, choose a
different private `/28` using `--backend-subnet`.

The script generates an independent 256-bit RSS admin token, an operator client
CA/certificate/key, and private files under `.runtime/`. It prints no secret.
It refuses to overwrite an existing configuration. Back up this directory like
credentials; it is ignored by Git. Compose's expanded configuration contains
credentials, so use the verifier instead of printing `docker compose config`.
The backend receives credentials as environment variables; Docker-host operators
can inspect them and are part of this deployment's trusted administration boundary.
The repository's `.dockerignore` excludes `.runtime/` and real `.env` files from
the build context, so these credentials cannot enter intermediate build layers.

## 2. Connect the existing identity stack

The setup output names two additions to make to your existing Stargate config:

- Add the RSS DNS name to `CALLBACK_ALLOWED_HOSTS`.
- Add the selected gateway IP as an exact `/32` to `TRUSTED_PROXIES`.

**Merge these values with existing entries; do not replace other applications'
allowlists.** The gateway address on the auth network is different from the
private backend address trusted by RSS Can. Restart/reload your existing
Stargate using its normal procedure. Keep its `AUTH_HOST`, Warden integration,
session exchange secret and persistent session configuration intact. Broad
cross-subdomain cookies are not required: `/_session_exchange` is proxied to
Stargate so it can validate a ticket and establish the RSS-domain session.

The template sends `GET /_auth` with `Accept: application/json` and the original
session cookie. A successful response's `X-Auth-User` becomes the sole backend
identity. Incoming request headers are disabled and only named headers are
forwarded. Client-supplied `X-Auth-*`, role/scopes, `X-User-*`, forwarded identities
and `Stargate-Password` cannot reach the auth checker or RSS backend. The original
`X-RSS-Admin-Action` is preserved on user writes, never automatically supplied.
The gateway also limits HTTP requests by the actual socket peer address to
10/second with a 30-request burst and 32 concurrent requests per IP; static assets
have a 100-request burst. It returns 429 with Retry-After for public limit failures.
These are single-gateway defaults, not a distributed DDoS defense. They do not
trust X-Forwarded-For; evaluate the limits for large shared-NAT deployments.

The gateway does not reinterpret an upstream failure as success. Browser 401s
redirect to the fixed login origin; unavailable auth services fail closed. All
machine routes go directly to RSS Can's token checks, with no injected identity
or admin token. The legacy `/feed/<id>/<format>` route is a machine endpoint in
this example: use a bearer token there, or copy the `/s/...` reader URL. Browser
management happens through `/sources`.

Verified contracts (pinned source references, not claims that arbitrary newer
versions are compatible):

- [Stargate `/_auth` response headers](https://github.com/soulteary/stargate/blob/bc0bb17244c342f4c3ebaa799e774af5c8824fb2/src/internal/handlers/forwardauth.go)
  and [request/401 handling](https://github.com/soulteary/stargate/blob/bc0bb17244c342f4c3ebaa799e774af5c8824fb2/src/internal/handlers/check.go).
- [Stargate deployment variables, callback routing and session requirements](https://github.com/soulteary/stargate/blob/bc0bb17244c342f4c3ebaa799e774af5c8824fb2/docs/zhCN/DEPLOYMENT.md).
- [Warden lookup and API-key headers](https://github.com/soulteary/warden/blob/af23fee285e3cb2d4c01cac56d50db86ab9338dc/pkg/warden/client.go): RSS Can independently calls
  `GET /user?user_id=...` with its dedicated bearer key and requires the same
  subject with `status: active` on every private download.

## 3. Validate, build and start

```sh
python3 verify.py
docker compose -f compose.yml up -d --build
python3 verify.py --live
```

The static verifier captures and checks Compose's JSON without printing it. It
rejects weakened auth mode, broad backend proxy trust, exposed backend ports,
an exposed operator listener, unpinned images, changed Nginx policy and unsafe
file permissions. The live check additionally tests Nginx syntax and refusal of
spoofed identities, anonymous operator access and invalid reader credentials.
It does not log in as a real user or claim to replace the full two-user test.

The backend data volume is initialized with private permissions for UID 65532.
Its one-time permissions container has no network and only the capabilities
needed to set volume ownership. Nginx's master reads private TLS files and starts
unprivileged workers; it has a read-only root and narrowly specified startup
capabilities. No Docker control socket is mounted. Nginx access/error request
logging is disabled to avoid recording reader URL secrets; use the application's
redacted logs and protected metrics for operational diagnostics.

## 4. Sign in and create a source

Open `https://rss.example.com/sources` and sign in through Stargate. The first
successful login automatically creates a local principal after Warden confirms
the account is active. Sources created in the wizard are private by default.
An operator-disabled local account stays disabled after login.

Operators may optionally pre-provision a principal, or explicitly re-enable one
that was disabled. Use the stable Warden user ID, not an email that may be
reassigned:

```sh
python3 rss_operator.py /api/access/principals \
  --data '{"subject":"YOUR_WARDEN_USER_ID","disabled":false}'
```

The account must also remain active in Warden. The wizard creates declarative
SSR sources; it does not accept JavaScript or enable browser rendering.
Operators can also register an existing reviewed JavaScript rule through the
operator channel:

```sh
python3 rss_operator.py /api/access/sources \
  --data '{"id":"geekpark","owner":"YOUR_WARDEN_USER_ID","readers":[]}'
```

`rss_operator.py` authenticates with the generated client certificate and sends the
explicit mutation header. It never reads or prints the backend admin token.
Ordinary SSO users cannot reach `/api/access/*`, `/config/*`, `/inspector` or
metrics on the public listener. Keep the operator private key and CA key out of
user browsers; anyone holding that client identity is an operator. The client
certificate expires after one year; rotate it and the CA through a reviewed
configuration change before expiry.

## Acceptance and lifecycle

Test two different Warden users and a real RSS reader: user A must not list or
read B's private source; invalid/expired/revoked reader tokens must fail without
a login redirect; public rules must be explicitly designated; disabling a
principal must deny its next private request. Stop Stargate/Warden separately
and verify private requests fail closed. Team mode currently disables CSR/MIX
and inspector browsers; it never silently weakens fetch policy to make them run.

Back up `access-data` and the private generated configuration. A normal upgrade
rebuilds this checkout and recreates the services without deleting that volume:

```sh
docker compose -f compose.yml up -d --build
```

Do not use `down -v` during upgrades. This example does not alter or back up the
existing Stargate/Warden data. To rotate exposed reader URLs use the source UI;
to rotate deployment/operator credentials, preserve the data volume and review
new generated configuration before replacing the old gateway and backend.

For offline template regression tests:

```sh
python3 -m unittest discover -s . -p 'test_deployment.py'
# Optional real builder-context exclusion check (uses an isolated scratch image):
RSS_TEST_DOCKER=1 python3 -m unittest discover -s . -p 'test_deployment.py'
# Include real Nginx TLS/mTLS/header/rate tests against isolated stub upstreams:
RSS_TEST_DOCKER=1 \
RSS_TEST_NGINX_IMAGE="$(docker image inspect nginx:stable-alpine --format '{{index .RepoDigests 0}}')" \
  python3 -m unittest discover -s . -p 'test_deployment.py'
```
