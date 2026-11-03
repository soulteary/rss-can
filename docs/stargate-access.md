# Stargate identity and private feed access

RSS Can has two explicit operating modes. `standalone` preserves the existing
trusted-rule deployment. `team` requires source permissions before it serves
feeds, including feeds reached through the old `/feed/:id/:type` addresses.
An unknown rule is never implicitly public in team mode.

This change implements the identity and access layer. It does not make arbitrary
JavaScript safe to upload or replace network/container isolation. Keep rule
installation restricted to operators until the fetch-policy and execution
isolation changes have also been deployed.

## Verified upstream contract

The adapter uses the stable `X-Auth-User` value returned by Stargate's
`GET /_auth`. It deliberately ignores `X-Forwarded-User`, which can be only an
`authenticated` marker, and does not use `X-Auth-Role` or `X-Auth-Scopes` to grant
RSS Can management privileges.

The contract was checked against Stargate commit
[`bc0bb17244c342f4c3ebaa799e774af5c8824fb2`](https://github.com/soulteary/stargate/blob/bc0bb17244c342f4c3ebaa799e774af5c8824fb2/src/internal/handlers/forwardauth.go).
The official [deployment guide](https://github.com/soulteary/stargate/blob/bc0bb17244c342f4c3ebaa799e774af5c8824fb2/docs/zhCN/DEPLOYMENT.md)
lists `X-Auth-User` in `authResponseHeaders`. Password-only sessions without a
stable Warden user ID cannot be used as distinct team users.

Every private browser or reader request also checks Warden directly:
`GET /user?user_id=<subject>`, with `Authorization: Bearer <API key>`. The response
must identify the exact subject and have `status: "active"`. This follows
[`warden/pkg/warden/client.go`](https://github.com/soulteary/warden/blob/af23fee285e3cb2d4c01cac56d50db86ab9338dc/pkg/warden/client.go).
These checks have a two-second ceiling, reject redirects, and fail closed when
the directory is unavailable. No Warden response is cached by this adapter.

Stargate's session refresh interval therefore does not postpone revocation of a
reader token. An account disabled in Warden cannot download a private feed on
its next request. Revocation cannot retrieve content a reader already saved.

## Team configuration

Create a private writable directory (mode `0700`) owned by the service user.
The service writes a `0600` authorization file and a persistent advisory lock
file there. The lock is automatically released by the OS on process exit; do
not delete the lock file while a process is running.

```sh
RSS_ACCESS_MODE=team
RSS_ACCESS_STORE=/var/lib/rss-can/access.json
RSS_PUBLIC_ORIGIN=https://rss.example.com
RSS_TRUSTED_PROXIES=172.25.0.10/32
RSS_WARDEN_URL=http://warden:8081
RSS_WARDEN_API_KEY=<dedicated-api-key>
RSS_ADMIN_TOKEN=<at-least-32-random-characters>
```

`RSS_PUBLIC_ORIGIN` is an HTTPS origin with no path or trailing slash. Use an
exact reverse-proxy address or narrow CIDR; `0.0.0.0/0` and `::/0` are rejected.
RSS Can derives proxy trust from the socket address, never from forwarded IP
headers. Prefer authenticated TLS for Warden across hosts; HTTP is intended
only for an isolated same-host service network.

At the edge:

1. Remove all incoming `X-Auth-*`, `X-Forwarded-User` and identity injection
   headers before processing authentication. Forward only Stargate's verified
   `X-Auth-User` response on browser routes.
2. Never publish RSS Can's port directly. Only the configured proxy may reach
   its service network. Client-controlled forwarding headers do not confer trust.
3. Protect browser UI routes with Stargate. The exact `/s/` reader route bypasses
   browser SSO but is always authenticated by RSS Can's scoped token. Do not
   forward it to an anonymous copy of the old service.
4. Do not inject the admin token for every signed-in user. Use a separate
   operator-only route/network for `/api/access/*`, `/config/*`, and inspector
   management. Keep the token out of browser JavaScript and HTML.
5. Suppress reader secrets from proxy/CDN/APM logs as well as application logs.
   Do not put reader URLs into analytics or permit shared caching.

The store is a bounded, single-process JSON database with atomic replacement,
file sync and an exclusive advisory lock. Multiple replicas must not open the
same store. Back it up like authorization data, not like disposable feed cache.
Changing source permissions or principal status advances an authorization
version that downstream output caches can include in their keys. Creating or
revoking a reader credential does not invalidate unrelated content snapshots;
the credential itself is checked freshly before every read.
The store is bound to its configured Warden URL. Repointing an existing store
at another directory issuer fails startup rather than silently rebinding user
IDs. Plan an explicit permission migration when changing directory authorities.

## Register a source

Install and review its rule through the existing operator workflow first. Obtain
its rule ID from the operator inventory. A source record must reference an
existing rule. A registration request is authenticated with the operator token:

```http
POST /api/access/sources
Authorization: Bearer <admin-token>
X-RSS-Admin-Action: 1
Content-Type: application/json

{"id":"<rule-id>","owner":"<warden-user-id>","readers":[]}
```

Private is the default. Set `"public":true` only when anonymous access to the
output is intended. Changing the owner revokes all previously issued grants for
that source. Removing a reader immediately removes the reader's grant access.
Owners and named readers are provisioned as active local principals unless an
operator has already disabled them.

`GET /api/sources` and `/feed` return only the caller's readable sources. They
require a trusted `X-Auth-User` plus an active Warden record. Administrative
access is still separate from that identity and cannot be obtained by setting a
role header.

## Create a reader credential

The signed-in user's browser sends:

```http
POST /api/sources/<rule-id>/grants
Origin: https://rss.example.com
X-RSS-Admin-Action: 1
Content-Type: application/json

{"formats":["rss"],"days":90}
```

The origin must match the configured public origin and writes require the
non-simple `X-RSS-Admin-Action: 1` header. The default lifetime is 90
days; allowed lifetimes are 1–365 days. Formats may be `rss`, `atom`, and `json`.
The response includes an ID, a token, a copyable URL, and its expiry:

```json
{
  "id": "<random-selector>",
  "token": "<random-selector>.<random-secret>",
  "url": "https://rss.example.com/s/<random-selector>/<random-secret>/rss",
  "expires_at": "<timestamp>"
}
```

Each account may hold up to 64 active reader credentials across all sources.
Issuance above that limit returns HTTP 429 with `code: "GRANT_LIMIT"`; revoke an
unused credential before creating another. A separate global ceiling of 8,192
active credentials leaves room in the bounded store for operator-managed
principals and sources. Expired and revoked records are reclaimed during
successful store mutations, including administrative changes, so inactive
credentials do not permanently occupy capacity. Existing oversized active
stores remain readable; new issuance waits until usage falls below the limits.

The secret is 256 random bits. Only its SHA-256 digest is stored. It is returned
only by this creation response and cannot be recovered later. A reader that
supports request headers can instead use:

```http
GET /feed/<rule-id>/rss
Authorization: Bearer <random-selector>.<random-secret>
```

Both forms check the same source permission, active account, requested format,
expiry and revocation before feed generation. A token never permits source
editing, inspection, account administration, or a different source. A failed
machine request returns `401` or `503`, not an HTML login redirect.

Treat the URL as a password: a reader's synchronization service will also see
it. Issue one credential per reader, do not publish it, and replace it when a
device or synchronization service is no longer trusted.

## Revoke and disable

The authenticated owner of a credential can revoke it:

```http
POST /api/grants/<grant-id>/revoke
Origin: https://rss.example.com
X-RSS-Admin-Action: 1
```

An operator can disable a local principal as an emergency control independent
of Warden availability:

```http
POST /api/access/principals
Authorization: Bearer <admin-token>
X-RSS-Admin-Action: 1
Content-Type: application/json

{"subject":"<warden-user-id>","disabled":true}
```

Disabling a principal also permanently revokes its existing grants. Setting
`disabled` back to `false` permits fresh credentials but does not resurrect old
ones. Warden's account status must also be active. An in-flight download already
authorized before revocation may finish; every new request checks fresh state.

## Migration and failure behavior

- Existing standalone deployments remain compatible until they opt into team
  mode. They should remain on a trusted private network.
- Enabling team mode starts with no public sources. Register each old rule with
  an owner or explicitly mark it public before sharing replacement reader URLs.
  Old private `/feed/` URLs without a valid identity or reader token stop working.
- Missing configuration or an invalid/private-store permission error prevents
  team startup. A missing access controller in an embedding application rejects
  requests rather than reverting to anonymous routes.
- Warden errors fail closed for private reads; public sources remain explicitly
  public. Cache fallbacks do not substitute for authorization checks.
- Do not reuse one authorization file across replicas. Shared-database support,
  delegated source editors, and automatic rule creation are separate work.

## Tests

```sh
go test -race ./internal/access ./internal/server ./internal/application -count=1
```

The access tests cover durable reload, file permissions, exclusive ownership,
corrupt schemas, reader isolation, format and expiry limits, revocation,
disable/re-enable, ownership transfer, concurrent reads and Warden identity
matching. HTTP tests cover trusted proxy identity, role-header spoofing,
directory filtering, legacy routes, CSRF origin checks, reader credentials and
fail-closed dependency behavior.
