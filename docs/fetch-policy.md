# Fetch policy and execution boundaries

**Availability:** these policies require the current source build. Published `v0.4.0` ignores `RSS_FETCH_POLICY` and `RSS_ACCESS_MODE`. Follow the [source deployment and verification guide](secure-deployment.md#build-and-verify-current-source) before relying on them.

The default `RSS_FETCH_POLICY=trusted` preserves existing operator-owned rules,
including private HTTP origins, proxies and remote browsers. Do not expose rule
creation in this mode to untrusted users. `RSS_FETCH_POLICY=public` is the safer
profile for shared deployments. Team access mode always selects public policy,
even if the environment also requests trusted policy.

Public policy applies to every HTTP request used by SSR, pagination and detail
extraction. It rejects credentials in URLs, non-HTTP(S) URLs, non-public IP
ranges, and explicit ports other than 80 or 443. DNS results are all checked and
the transport dials only those checked addresses. Redirects are checked on each
hop, with at most ten redirects. Environment HTTP proxies and rule-configured
proxies cannot bypass this policy: public mode disables the former and rejects
the latter. Browser/Headless overrides in rules are also rejected.

Each feed generation has one shared budget of 64 HTTP requests and 64 MiB of
HTTP response body reads, including redirects and failed requests. Existing
per-page decoded HTML, feed output, item, page, timeout and concurrency limits
continue to apply. Cache hits do not consume network budget. Different policy
profiles use separate document cache keys and in-flight request keys; targets
and cached URL metadata are checked before cached content is returned.

HTTP fetch cookie credentials are scoped to the exact scheme, hostname and
effective port of the original source in both profiles. A redirect or detail URL on a different
port or scheme does not receive them. Existing host-only and path restrictions
still apply. This intentionally tightens legacy HTTP cross-port behavior. Trusted browser
mode retains Chrome cookie semantics and does not claim exact-origin cookie
isolation; it is not available in public/team mode.

## Browser behavior

Public mode refuses CSR, MIX, and inspector browser work **before cache lookup
or browser allocation**. It does not silently fall back to an unrestricted
browser. Use SSR sources in team mode until a separately isolated execution
service and its network policy have been validated. A new browser context,
Chrome DevTools Protocol request interception, or an `internal` Docker network
alone is not a complete malicious-page isolation boundary.

Trusted deployments retain existing CSR/MIX behavior. Their browsers must be
operator-controlled and must not have access to application credentials,
control-plane networks or host files. A production browser executor needs
network enforcement covering new targets, workers, service workers, WebSockets
and every other connection, independently of page-level interception. The
current implementation does not claim to provide untrusted script hosting.

## JavaScript workers

Deno is started without application credentials or inherited proxy/Node/Deno
permission overrides. Only temporary-directory and Windows-system location
variables are retained, together with disabled update checks and color output.
Each worker receives a private temporary `DENO_DIR`, removed on process exit;
it does not inherit the parent cache directory or require a passwd/home entry.
The embedded runner remains noninteractive and receives no read, write, network,
environment or process-execution permissions. Its V8 old-space limit is 128 MiB;
worker exit and existing timeout handling discard a failed process before reuse.
This heap limit does not cap all native or external allocations. Deploy the
service with container CPU, total memory and PID limits, a read-only filesystem,
a non-root user and no privilege escalation. The VM context is state separation,
not a promise that arbitrary third-party JavaScript is safe to host.

## Verification

Policy regression tests cover prohibited addresses and ports, mixed DNS answers,
validated-IP dialing, redirect/proxy policy, exact-origin cookies, shared request
and body budgets, cache namespaces and browser refusal before allocation.
Run these with the repository's pinned Go and Deno versions:

```sh
go test -race ./internal/network ./internal/feedservice ./internal/parser ./internal/jssdk ./internal/application
```

Browser integration tests still require the separately provisioned test browser
specified in `docs/testing.md`. Tests use local fixtures; they do not probe live
cloud metadata services.
