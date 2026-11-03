# Secure deployment and administrative actions

Source builds run as UID/GID 65532. The example Compose files bind only to
127.0.0.1, mount rules read-only, drop Linux capabilities, disable privilege
escalation, and provide a bounded writable temporary directory. These are
single-user examples, not an authorization boundary for multi-user hosting.
The temporary mount uses standard sticky mode `1777`, allowing the numeric
service user to create directories without engine-specific tmpfs `uid`/`gid`
options. The companion runtime-hardening change gives each Deno worker a private
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
separate access-control change; do not exempt legacy feed paths from access
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

Run `docker compose config` for the chosen example, build the source image, then
start it with a read-only root filesystem and a writable /tmp as shown in the
examples. Confirm UID 65532, readiness, a feed request, shutdown, and denied
administrative GET. Verify backend/control ports are inaccessible externally.
Pin the complete tested release set and image digests in your deployment lock
file. The examples use a release tag for readability, not a supply-chain lock.
The historical browser example remains a compatibility example for trusted
sources; its CDP endpoint must remain private. Neither a Docker internal network
nor page request interception alone proves malicious-browser isolation.
