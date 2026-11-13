# Secure deployment and administrative actions

## Build and verify current source

For an existing Stargate/Warden stack, start with the [guided source-based
Compose deployment](../example/4.with-stargate/README.md). Its setup generates
private environment files, builds RSS Can from source, and wires the HTTPS
gateway and private backend network. The commands below are for operators
maintaining their own container wiring.

The security behavior below applies to **v0.5.0 and later**, and current source
builds, not historical `v0.4.0`. That historical image ignores `RSS_ACCESS_MODE` and
`RSS_FETCH_POLICY`; setting them does not add authentication or SSRF protection.
It also predates the POST-only administrative API and log redaction described
below. The standalone Compose examples now use v0.5.1 but do not enable team identity;
use the Stargate example for a shared deployment.

For the released image, pull the pinned version and verify that its diagnostics
complete successfully before configuring real credentials:

```sh
docker pull soulteary/rss-can:v0.5.1
docker run --rm --network none -e RSS_REDIS=false soulteary/rss-can:v0.5.1 rssc doctor
```

Alternatively, from a reviewed checkout, build and verify a separately named
source image. Do not overwrite a published release tag:

```sh
docker build -f docker/Dockerfile --build-arg COMMIT="$(git rev-parse HEAD)" -t rss-can-source:local .
docker run --rm --network none -e RSS_REDIS=false rss-can-source:local rssc doctor
```

The second command must finish successfully and print diagnostic results,
including `OK` lines; merely seeing a running HTTP server is not success.
The old release does not implement this command. Record the resulting image ID
and commit, then deploy that exact image. Do not replace it with `v0.4.0` when
following the [team configuration](stargate-access.md#team-configuration).

For a team deployment, prepare a private state directory owned by UID/GID 65532
(mode `0700`). Store the team environment from that guide in an operator-only
file `.env.team`, using unquoted `KEY=value` lines (Docker retains literal
quotes). Create it with `umask 077`, require mode `0600`, and never commit
credentials. `.env.team` is excluded from Git and the Docker build context. Set
`RSS_ACCESS_STORE=/state/access.json` and
`RSS_REDIS=false` unless a private Redis is configured. Run the verified release image (or substitute the separately verified source image):

```sh
docker run --name rss-can-team --user 65532:65532 \
  --read-only --cap-drop ALL --security-opt no-new-privileges:true \
  --pids-limit 128 --memory 512m --cpus 2 \
  --tmpfs /tmp:rw,noexec,nosuid,size=64m,mode=1777 \
  --env-file .env.team -p 127.0.0.1:8080:8080 \
  -v "$(pwd)/state:/state" -v "$(pwd)/rules:/rules:ro" \
  soulteary/rss-can:v0.5.1
```

Connect the authenticated reverse proxy through a private path and set
`RSS_TRUSTED_PROXIES` to its actual socket source address; do not expose the
backend to clients or broaden trust to compensate for a wrong address. Verify
`/_/ready/` succeeds, an unauthenticated feed request is denied, an authorized
user can access only an assigned source, and a revoked reader credential is
rejected before sending real traffic. Run `doctor` with the same environment
and mounts when diagnosing the deployed configuration; failure must be fixed,
not bypassed by switching back to the old image.

v0.5.0 and current source container images run as UID/GID 65532. The standalone Compose files bind only to
127.0.0.1, mount rules read-only, drop Linux capabilities, disable privilege
escalation, and provide a bounded writable temporary directory. These are
single-user examples, not an authorization boundary for multi-user hosting.
The temporary mount uses standard sticky mode `1777`, allowing the numeric
service user to create directories without engine-specific tmpfs `uid`/`gid`
options. Current source gives each Deno worker a private
`0700` cache subdirectory and removes it on exit; the shared `/tmp` mount is not
itself an execution-isolation boundary.
A mounted rule must be readable by UID 65532; do not make secrets world-readable
as a workaround. Use a dedicated group/mount ownership where credentials exist.

## Administrative API migration

State-changing configuration requests now require POST and the explicit header
`X-RSS-Admin-Action: 1`. GET returns 405 and never reloads rules or changes logging.
The existing admin token, Origin and Fetch Metadata checks remain mandatory.
For example, with the token supplied by your secret manager:

```sh
curl --fail --request POST \
  --header "Authorization: Bearer ${RSS_ADMIN_TOKEN}" \
  --header 'X-RSS-Admin-Action: 1' \
  http://127.0.0.1:8080/config/rules/fresh/
```

Invalid log levels return 400. Cross-site browser requests are denied. The
non-simple action header prevents ordinary cross-site forms from triggering
mutations; the service does not allow cross-origin preflight requests. Reverse
proxies must not automatically inject this action header for arbitrary callers.

## Reverse proxies and Stargate

Keep the RSS Can backend, Redis and browser control port off public interfaces.
Use HTTPS at the only public ingress. Strip incoming identity headers before
trusted authentication middleware sets them. A successful Stargate login does
not grant administrator privileges: only an explicitly authorized operations
route may inject RSS_ADMIN_TOKEN, and the token must never reach a browser.
Stargate's native integration and per-source authorization are documented in the
[team access guide](stargate-access.md); do not exempt legacy feed paths from access
checks just to make a reader follow a login redirect.

## Logging and preview privacy

Access logs record route templates rather than concrete paths and omit Referer.
Upstream failures do not log raw URLs or error strings, which can contain Cookie
query credentials or private source contents. Configure the reverse proxy, CDN,
APM and log collector to use the same policy, especially for private subscription
URLs. Retain only the minimum operational data and limit log-reader access.
Inspector responses use no-store, no-referrer and nosniff in addition to their
existing opaque-origin CSP sandbox. External image/style resources can still
contact their host: do not use the inspector for sensitive authenticated pages.

## Deployment verification

For standalone examples, run `docker compose config`, then start the verified
release or source image with a read-only root filesystem and a writable `/tmp`.
For the Stargate example, run `python3 verify.py` and then `python3 verify.py --live`
from its directory instead: expanded Compose output contains credentials and
must not be printed. Confirm UID 65532, readiness, a feed request, shutdown, and denied
administrative GET. Verify backend/control ports are inaccessible externally.
Pin the complete tested release set and image digests in your deployment lock
file. The examples use a release tag for readability, not a supply-chain lock.
The standalone browser example remains a compatibility example for trusted
sources; its CDP endpoint must remain private. Neither a Docker internal network
nor page request interception alone proves malicious-browser isolation.
