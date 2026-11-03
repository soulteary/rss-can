# Create and maintain a personal feed

**Requires current source:** published `v0.4.0` has none of the team access, public fetch policy, source wizard or `doctor` features described here. It silently ignores the new configuration variables. [Build and verify the current source image](secure-deployment.md#build-and-verify-current-source) first; do not add team settings to the historical standalone image.

After enabling [team access](stargate-access.md), open `/sources` through the
authenticated gateway. A first successful Warden-backed login provisions a
local principal; it never re-enables an operator-disabled account.

## The source wizard

1. Paste a public website URL and choose a name. A blank name uses the hostname.
2. Start with the article or blog template. Open the advanced controls only if
   the page uses different list, title or link selectors.
3. Preview the extracted titles and links. The server rejects incomplete items,
   unsupported protocols, internal destinations and executable configuration.
4. Save. The server performs its own bounded preview again, creates a private
   source and offers a dedicated reader credential.
5. Copy the URL into the reader. Create another named credential for another
   device; revoke an individual credential from the list when it is no longer
   needed.

The wizard supports server-rendered pages. It does not request a browser worker,
accept JavaScript, or ask the user for a proxy, browser endpoint or raw cookie.
Sites requiring dynamic rendering or authentication remain an operator-managed
integration; the wizard does not silently relax network policy for them.

The advanced controls accept single simple CSS selectors, not JavaScript
expressions. Tags, classes, IDs and their compounds are supported, such as
`article.card`, `.title`, `#news` and `a.link`. Combinators, selector lists,
pseudo-classes, attributes and CSS escapes are rejected. This avoids allowing
user-supplied selectors to trigger unbounded backtracking in the shared process.
The
source format is deliberately small:

```json
{
  "name": "Example news",
  "url": "https://example.org/news",
  "item_selector": "article",
  "title_selector": "h2",
  "link_selector": "a",
  "date_selector": "time",
  "description_selector": "p",
  "max_pages": 1,
  "refresh_seconds": 600
}
```

Refresh intervals are 60–86400 seconds; page budgets are 1–20. The source's
refresh interval controls upstream document freshness. The final-output
snapshot cache has its own deployment-level freshness policy. HTML descriptions
from these sources are escaped, and non-HTTP(S) entry links fail generation.

Changes retain the last five configurations. Editing requires the current
revision number so concurrent tabs cannot silently overwrite one another.
“Restore previous version” restores a known configuration; it does not promise
that a remote website still serves the same page. All stored sources and
versions remain private unless an operator explicitly changes access.

## API for the same workflow

Browser write requests require a trusted identity, the exact configured Origin,
and `X-RSS-Admin-Action: 1`. They share a per-subject write budget: a burst of
eight requests, refilling one request every five seconds. The limiter tracks at
most 4096 subjects and returns `429` with `Retry-After` when saturated. Rotating
reader credentials cannot reset it. Two previews may run concurrently.

- `POST /api/sources/preview` with `{"spec": ...}` returns up to five preview
  entries. It does not save anything.
- `POST /api/sources` with `{"spec": ...}` validates and previews server-side,
  then saves a private source owned by the current user.
- `POST /api/sources/:id` with `{"spec": ..., "revision": 1}` updates an owned
  managed source; a stale revision returns `409`.
- `POST /api/sources/:id/rollback` with `{"revision": 2}` restores its preceding
  configuration.
- `POST /api/sources/:id/delete` with the current `revision` deletes an owned
  managed source and all its reader credentials. Old URLs stop authorizing
  immediately. The wizard asks for confirmation first.
- `POST /api/sources/:id/refresh` requests an owned source's RSS snapshot through
  the bounded refresh service. It returns `fetched_at`, `changed_at` and `stale`
  metadata, not the feed body. The wizard displays the result of this manual
  check; it does not poll every source or invent a last-success timestamp.
- `GET /api/sources` lists readable sources; editing details and history are
  returned only to their owner.
- `GET /api/grants` lists the current user's active credentials with labels and
  expiry, never their secrets. Creation/revocation endpoints are documented in
  [team access](stargate-access.md).

Managed pages are limited to 1 MiB, 10,000 DOM nodes, 48 levels of nesting and
100 extracted entries across pages. These limits supplement the global request
and execution budgets; the wizard does not bypass them.

Each user may create at most 100 managed sources, with a 256 KiB serialized
budget including their version histories. All managed sources share a 4 MiB
budget, reserving authorization-store capacity for other records. Permission
storage also has global record and byte limits. This is a single-process store: use an explicit
shared storage design before running multiple replicas.

Manual refresh rebuilds the final representation under the existing upstream
document-cache policy. It does not bypass target-domain budgets, retry backoff,
or the source's document freshness interval. A failed refresh leaves an existing
successful snapshot available subject to its configured maximum stale age.

## Read-only startup diagnostics

Run the same binary with the same environment as the service:

```sh
rssc doctor
```

It checks basic server configuration, fetch-policy/proxy compatibility, rule-directory readability, team identity
configuration, private authorization-store permissions and issuer binding,
JavaScript runtime readiness, and a usable cache backend. It reports actionable
messages without printing URLs, tokens, passwords or raw dependency errors.

The command starts no HTTP listener, acquires no authorization writer lock, and
does not create or rewrite the permission database. It may start a temporary
Deno worker and establish a cache health-check connection. An unsuccessful
required check produces a nonzero exit code. Deployment-level checks—such as
whether a host port is published to the Internet—still require verification in
the deployment environment.

If an environment value fails before configuration can be assembled, the
process exits without running these checks. It does not echo invalid values;
check the documented variable names and constraints when startup reports a
configuration failure.

Team installations may start with an empty, readable legacy rule directory;
the wizard then creates the first source. Standalone installations retain the
existing requirement for valid legacy rules.
