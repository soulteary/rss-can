# Runtime installation and upgrade guide / 运行时与升级指南

## Which distribution am I running? / 先确认版本

Published **v0.3.12** archives and images use the historical embedded runtime.
They do not require Deno. **v0.4.0**, **v0.5.0** and current source use
Deno **2.9.8**, with CGO disabled in Go release binaries. Changes under
a later `CHANGELOG.md` section do not change an existing published tag.
Never replace a published release or its images to apply these changes.

已发布的 **v0.3.12** 仍使用旧的内嵌运行时，无需 Deno。**v0.4.0**、**v0.5.0** 和当前源码
使用 Deno **2.9.8**。后续源码中的改动不会自动进入已发布版本；升级时使用新版本
或明确的源码构建镜像，不覆盖历史发布物。

Team access, public fetch policy, the source wizard and `doctor` are available in
**v0.5.0 and later**. Historical v0.4.0 silently ignores their mode settings.
Use a matching release or the [verified source-image procedure](secure-deployment.md#build-and-verify-current-source), never the old image for team traffic.

团队鉴权、公网抓取策略、来源向导和 `doctor` 从 **v0.5.0** 开始提供。
历史 v0.4.0 会静默忽略相关配置，不能承接团队流量。

<a id="upgrade-v050"></a>

## Upgrading from v0.4.0 to v0.5.0 / 从 v0.4.0 升级到 v0.5.0

v0.5.0 keeps Deno 2.9.8 and the same portable archive layout. Extract the whole
archive, retaining the executable sibling `deno`. The new team mode is optional:
without `RSS_ACCESS_MODE=team`, standalone mode continues to use trusted,
operator-installed rules. Upgrading the binary does not make a standalone feed
private. Historical v0.4.0 ignores the new access/fetch settings entirely.

Before switching traffic:

1. Back up the current image digest/archive, deployment settings and rule files.
   If upgrading a source-based team deployment, also back up its authorization
   database and private environment files while the service is stopped. Preserve
   ownership and permissions; do not copy these secrets into the image build
   context, Git repository or diagnostic output.
2. Use the v0.5.0 archive/image for your platform, or build a reviewed checkout.
   Containers now run as UID/GID **65532**. Mount rules read-only and make them
   readable to that user; provide writable private state for team mode, and a
   bounded writable `/tmp` when using a read-only root filesystem. Do not solve
   permission errors by making secrets world-readable or running as root.
3. Decide the freshness policy. Output snapshots are now enabled by default:
   `RSS_SNAPSHOT_TTL=30` and `RSS_SNAPSHOT_MAX_STALE=3600` (seconds). Readers may
   receive the last successful feed while a refresh runs or fails, within that
   bound. `RSS_SNAPSHOT_MAX_STALE=0` requires successful refresh after expiry;
   `RSS_SNAPSHOT_TTL=0` disables output snapshots but retains the document cache. A rule's `DisableCache` still
   bypasses caches. See [snapshot behavior](feed-snapshots.md).
4. Review representative RSS, Atom and JSON feeds. [HTML sanitization](feed-content-safety.md)
   now applies to **all** rules: CSS, scripts, embeds and other active markup are
   removed; images become alternative text; JSON summaries are plain text.
   This changes output for feeds relying on images or custom styling. There is
   no per-rule sanitizer bypass.
5. Update management clients: `/config/*` writes require **POST** and
   `X-RSS-Admin-Action: 1`, plus the existing authorization and same-origin checks.
   GET no longer mutates configuration. A proxy must not inject the action
   header for arbitrary callers or give every signed-in user the admin token.
6. Run `rssc doctor` with the deployment's environment and mounts, verify
   `/_/ready/`, then test the actual rules, details, cookies/proxies and reader
   workflows you use. Feed generations share a 64-HTTP-request/64-MiB upstream
   budget in addition to existing per-page and concurrency limits. Test both
   fresh and cached responses; use the same credentials to verify 304 handling.

For **team mode**, follow [team access](stargate-access.md) and the
[Stargate deployment example](../example/4.with-stargate/README.md) before moving
traffic. It enforces public SSR-only destinations, including redirect/DNS checks;
private network targets, executable user rules, browser rendering and upstream
proxy/cookie configuration are not wizard capabilities. Keep trusted legacy
integrations in a separate, private standalone deployment when needed.

Set the exact HTTPS public origin and trusted proxy socket addresses, a Warden
URL/API key and a private access-store path. The state directory must be owned
by the service user with mode `0700`; the store is `0600`. Only one service
process may write a store: stop the old instance before opening it in the new
one. Do not delete its advisory lock file while a service is running. The store
is bound to its Warden issuer; changing directory authorities needs an explicit
permission migration. An existing legacy rule is not automatically readable in
team mode: register its owner/ACL or create a source through the wizard.

Verify unauthenticated and unassigned reads fail, ordinary users cannot access
management, and revoked reader credentials and disabled Warden users lose access
on their next request. Authorization also runs before cache hits and 304s. Keep
backend, Redis and CDP ports private; expose only the authenticated gateway.

Manual refresh now bypasses and replaces upstream document caches even when
output snapshots are disabled. It retains request budgets, retry backoff and
cancellation. Source edits/rollbacks use revision-isolated page caches; a failed
refresh preserves eligible last-success content rather than publishing a partial
feed. No cache flush is needed for the new key namespace. Caches remain disposable,
whereas the authorization database contains ownership, credentials and source
history and must be preserved.

v0.5.0 延续 Deno 2.9.8 和归档布局，完整解压并保留同目录可执行的 `deno`。
团队模式是可选项；未设置 `RSS_ACCESS_MODE=team` 时仍为独立模式，仅信任管理员安装的
规则。升级程序不会自动把已有订阅变为私有；历史 v0.4.0 会忽略新的鉴权及抓取配置。

升级前备份镜像摘要/归档、环境和规则；已有源码团队部署还需停机备份权限数据库与私密
环境文件。容器以 UID/GID 65532 运行，规则只读且对该用户可读，团队状态目录权限为
`0700`、数据库为 `0600`，只读根文件系统需提供有容量限制的可写 `/tmp`。
不能通过开放秘密文件权限或改用 root 解决挂载问题。

默认输出快照保鲜 30 秒，允许额外 3600 秒旧内容；`RSS_SNAPSHOT_MAX_STALE=0` 禁止
过期快照响应，`RSS_SNAPSHOT_TTL=0` 关闭输出快照但保留页面缓存。HTML 清理作用于全部规则：删除 CSS、
脚本及嵌入内容，图片转为替代文本，JSON 摘要为纯文本，无单规则绕过开关。
管理写接口须改为 POST 并显式携带 `X-RSS-Admin-Action: 1`；代理不得为普通请求自动注入
此头或管理员凭据。运行 `rssc doctor`、检查就绪，并验证实际订阅、缓存、分页/详情与
Cookie/代理。每次生成共用 64 次 HTTP 请求、64 MiB 上游响应预算。

团队部署须配置 HTTPS 来源、精确可信代理、Warden 和私有权限文件，并保持单进程写入；
切换前停旧实例，运行中不可删除锁文件。权限库绑定 Warden，不能直接换成另一身份目录。
已有规则不会自动公开，须登记所有者和 ACL。团队模式强制公网 SSR；浏览器、内网及带
代理/Cookie 的可信规则需要独立私有部署。验收未登录/未授权拒绝、普通用户不能管理、
阅读器撤销和 Warden 禁用立即生效，以及缓存和 304 前仍鉴权。只公开网关，保持后端、
Redis 和 CDP 私有。

手动刷新现在重新抓取并更新底层页面缓存，即使已关闭输出快照；仍遵守预算、重试退避和
取消。来源修改/恢复版本采用隔离缓存，不会复用旧版本页面。缓存可重建，无需统一清空；
包含来源、历史、权限和阅读器凭据的数据库必须保留。

### Safe rollback / 安全回退

For standalone deployments, stop v0.5.0 and restore the exact prior image/archive,
settings and compatible rules. Retain the new backups separately. Do not mix
executables from different archives or flush Redis shared with other applications.

**Do not route team traffic to v0.4.0 or earlier.** Those versions ignore team
settings and cannot enforce the new ACLs or use managed sources/credentials.
Keep the team gateway closed while restoring a compatible team-capable build and
its matching private state backup. Do not overwrite or discard the newer store
merely to make an older binary start; preserve it for recovery and migrate
permissions explicitly if necessary. Restoring a database backup can restore
previously revoked grants or old ownership: reconcile revocations and re-test
access before reopening the gateway.

独立部署回退时先停新版，再恢复原镜像/完整归档、配置和兼容规则，另存新版备份。
团队流量不可切到 v0.4.0 或更旧版本：它们忽略团队配置，无法执行 ACL 或使用托管来源。
恢复兼容的团队版本和对应私密状态期间，关闭网关；不要覆盖或丢弃新权限库来强行启动旧版。
恢复备份可能带回已撤销凭据或旧所有权，重新核对撤销记录与权限并验收后再开放。

## Standalone / 独立二进制

v0.4.0 and v0.5.0 archives bundle checksum-pinned Deno 2.9.8 for the same OS/architecture as `rssc`. Extract the whole archive and keep executable `deno` beside `rssc`; do not copy only the Go binary. Runtime startup needs no network download. Linux archives require a glibc-based host (use the Debian container on musl/Alpine hosts). Source-only builds still need Deno installed. The executable lookup
order is `RSS_CAN_DENO`, an executable `deno` beside `rssc`, then `deno` on PATH.
An explicit override that does not work is an error; it is never silently ignored.
The startup error explains how to supply the dependency.

v0.4.0 和 v0.5.0 四平台归档内置与 `rssc` 匹配的 Deno 2.9.8。完整解压并保留同目录 `deno` 的可执行权限，不能只复制 Go 二进制；启动时无需联网下载。Linux 归档需要 glibc，musl/Alpine 主机请使用 Debian 容器。源码单独构建仍需安装 Deno。查找顺序为 `RSS_CAN_DENO`、`rssc` 同目录
下可执行的 `deno`、PATH。显式指定的路径无效会报错，不会偷偷改用另一个运行时。

```sh
/path/to/deno --version
RSS_CAN_DENO=/path/to/deno RSS_REDIS=false ./rssc
```

`RSS_REDIS=false` selects memory-only operation. Deno availability is checked
before opening the HTTP listener; Redis and Chrome are separate dependencies.
Chrome is needed for CSR/mixed rules. Configure its endpoint with
`RSS_HEADLESS_SERVER` or the rule's `Headless` option.

## Docker source build / 源码容器

```sh
docker build -f docker/Dockerfile -t rss-can:local .
docker run --rm -p 127.0.0.1:8080:8080 -e RSS_REDIS=false rss-can:local
```

The image includes pinned Deno and example rules. Mount custom rules at `/rules`.
v0.4.0 and v0.5.0 standalone archives also include Deno, rules, notices and this guide.
Release CI validates the four upstream runtime hashes and starts native archives with an empty PATH.

源码镜像内置 Deno 和示例规则，自定义规则挂载到 `/rules`。v0.4.0 和 v0.5.0 独立归档也附带 Deno、规则和升级说明，发布前校验四平台运行时摘要，并以空 PATH 启动本机归档。

## Rule behavior / 规则行为

SSR now uses `Cookies` and `Proxy` for listing, pagination and detail requests.
An explicit rule proxy takes precedence over `RSS_PROXY`. HTTP(S) URLs and
legacy `host:port` proxy values are supported. Remote browser contexts now also
apply their proxy, rather than silently ignoring it. Browser proxy authentication
is subject to Chrome's support; prefer a proxy without credentials for rules
shared between SSR and browser modes.

Configured cookies are host-only and rooted at `/` on the original rule URL.
They are not forwarded to another hostname by redirects, pagination or detail
hooks. Like browser cookies, cookies are not isolated by TCP port. Configure rules
only from trusted sources. HTTP environment proxies retain Go's default behavior
when no explicit proxy is configured.

A configured `readlink` hook must resolve a link, fetch successfully and match
its selector. HTTP failures, missing links/selectors and empty HTTP responses
fail the entire feed (HTTP 502); the overall deadline remains HTTP 504. A matched
empty element is valid; empty content can still use the configured inline
`Content` fallback. Explicit inline DateTime/Category/Description fields override
and skip the corresponding detail hook. There is no partial-feed success mode.

SSR 的首页、分页和详情请求均应用 Cookie 与代理；规则代理优先于全局代理。
Cookie 仅用于规则原始主机，不会随跳转或详情链接发往其他主机。详情 hook 的链接缺失、
上游失败或选择器无匹配会使整次订阅失败，避免成功响应里悄悄缺少正文。
已匹配但内容为空的元素仍合法；日期、分类、摘要的显式内联字段优先于对应 hook。

## Concurrency and memory / 并发与内存

`RSS_JS_WORKERS` is read once per process: default **1**, valid range **1–16**.
Restart after changing it. Startup validates every worker. Each execution gets a
fresh context; a timeout kills only that worker. Queue waiting counts toward the
feed deadline. Shutdown drains active calls and closes every child process.

```sh
RSS_JS_WORKERS=2 RSS_CAN_DENO=/path/to/deno RSS_REDIS=false ./rssc
```

SSR/mixed detail extraction uses up to **4** entry workers per page and **16**
active detail fetches across the process, preserving item order. Pagination stays
sequential; the page budget, de-duplication and shared deadline are unchanged.
Browser work remains capped at **4** tasks. These are concurrency bounds, not
production capacity promises. More JS workers consume more memory; evaluate
`rss_can_js_queue_seconds`, `rss_can_js_active`, feed p95 and process-tree RSS
with real rules before increasing the pool.

JS worker 数默认仍为 1，可通过 `RSS_JS_WORKERS=2` 等配置增加，最大 16，修改后重启。
详情抓取每页最多并行 4 项，全进程最多 16 项，保持订阅顺序。分页仍按顺序处理。
增加 worker 会增加内存占用，应结合队列时间、订阅 p95 与进程树内存观察效果。

## Upgrade checklist / 升级验证

Back up rules and deployment configuration. Check `./rssc -h`, confirm Deno
startup, then check `/_/ready/` and one SSR, CSR and mixed feed that you actually
use. Test authenticated upstreams and detail hooks explicitly. If a formerly
successful feed returns 502, inspect the server log for the failed hook and fix
its link/selector or upstream access instead of accepting truncated content.

先备份规则和部署配置，检查运行时启动、就绪接口及实际使用的订阅。对需要登录、代理、
详情提取的规则单独验证。升级后出现 502 时查看日志定位详情 hook，修正规则或上游访问。

## Historical v0.4.0 to v0.3.12 rollback / 历史版本回退

Keep the old v0.3.12 archive or fixed image tag and a backup of rules/configuration. Stop v0.4.0 before restoring the old executable/image and deployment settings; keep its original embedded runtime rather than mixing files between archives. No persistent configuration format migration is required by this release. Newly introduced settings/features do not work in v0.3.12; restore rules verified with that version. Clear disposable feed/document caches if responses differ after rollback; do not flush a Redis instance shared with other applications.

保留 v0.3.12 原归档/固定镜像与规则、配置备份。先停 v0.4.0，再恢复旧程序、部署设置及旧版验证过的规则，不要混用归档文件。本次无需迁移持久化配置格式，但新配置和新功能在旧版不可用。必要时清理本应用可丢弃的缓存，不可清空共享 Redis。

## Architecture hardening in v0.4.0

Rule evaluation, CSR injection and browser cache identity now share an immutable
copy of the loaded rule. Editing a rule file does not change an in-flight
request or the currently loaded rule; reload rules through the management
endpoint to publish the new source. Direct Go parser callers supplying only a
file path capture its contents once when entering the browser parser.

Upstream HTTP fetching now admits at most 16 active requests and 64 waiting
requests per process. Waiting respects the feed deadline. Response bodies retain
their slot until consumed or closed. Cached documents bypass admission, shared
fetches consume one slot, and `DisableCache` does not bypass the limit. A full
queue returns HTTP 503 from feed endpoints; upstream failures remain 502 and
expired feed deadlines remain 504. Browser task limits remain separate. These
are fixed process-wide defaults, not a per-origin fairness policy or a total
process-memory limit.

Redis cache operations have a 100 ms budget bounded by the caller's remaining
context. A failed operation opens a one-second circuit; afterward one request
probes Redis while other requests continue using the configured memory
fallback. Caller cancellation does not trip the circuit. The memory backend is
an outage cache, not a normally prewarmed second level: successful primary
writes and deletes invalidate the corresponding local fallback value. Readiness
still probes Redis independently. No normal-period dual-write guarantee is
introduced.

Feed generation is now orchestrated by `internal/feedservice`. Startup creates
and injects the service; HTTP retains format/content-type validation, error
status mapping and request metrics. The service owns a single generation
deadline and accepts rule lookup, configuration, fetching and serialization
functions as dependencies. Legacy module globals remain behind the production
adapters for incremental migration.

## Capacity, transactional reload and owned application lifecycle

Source builds now apply fixed capacity budgets in addition to deadlines:

- Each memory cache instance keeps at most 1024 entries and 64 MiB of key/value
  bytes, with a per-entry limit of 10 MiB + 1 KiB. LRU eviction enforces both
  aggregate limits. This accounts for retained payload bytes, not all Go heap
  overhead or Chrome/Deno process memory. Oversize cache writes are rejected
  without replacing an existing value; feeds can still succeed without caching.
- Each FeedService admits eight active generations and 32 waiting generations.
  The slot covers configuration, fetching, parsing and serialization. A full
  queue returns 503; waiting is included in the existing generation deadline.
  The process-wide HTTP budget of 16 active and 64 waiting fetches still applies.
- A decoded or browser-extracted page and a serialized feed are each limited to
  10 MiB. A feed can retain at most 1000 entries, 20 pages and 10 MiB of entry
  fields including details. These are rejection limits, not silent truncation.
  An omitted/zero PagerLimit still means only the current page. JavaScript
  request/response protocol messages are limited to 16 MiB. Chrome checks the
  extracted string before returning it over CDP; this does not cap the remote
  page's own JavaScript allocations or browser memory.

CSR/MIX requests with caching enabled now share in-flight raw page extraction by
immutable browser cache key and application scope. Each caller keeps its own
cancellation and subsequent parsing/detail work. The last departing caller
cancels shared work; `DisableCache` keeps independent execution. This prevents a
cold-cache burst for one subscription from duplicating its browser extraction.

Rule reload now reads and validates a complete candidate set before atomically
publishing it. Missing/unreadable directories or files, duplicate IDs, invalid JS
and invalid configuration reject the reload and preserve the previous snapshot.
An empty rule set is explicitly rejected to protect against a temporarily empty
mount. The management endpoint returns 500 on failure, and production startup
fails if no valid initial rule set can be loaded. Configuration validation
executes rules but does not freeze their time-dependent output; generation still
evaluates dynamic rules normally. Environment options are captured at startup.

Production startup now constructs an `application.App` owning its rule
repository, bounded cache, network client, generation service and JS pool.
Legacy context-aware parsing helpers bind to those owned resources through
request-scoped adapters; legacy standalone wrappers remain for compatibility.
Separate apps do not share cache contents, configuration caches, JS workers or
in-flight fetches. HTTP/browser/detail process limits and telemetry remain
process-wide safeguards. Browser settings and admin credentials are captured
per app as well.

Shutdown stops admission and cancels active requests instead of relying on a
fixed two-second grace shorter than a feed deadline. The HTTP grace defaults to
the server timeout plus one second. Handler and shared-work cleanup completes
before owned cache/JS resources close. Listener/startup/shutdown errors return to
the owner, allowing cleanup instead of exiting through a lower-layer fatal log.

## List, rule-file and cache-hit boundaries

The production subscription list now reads display metadata captured by the
last successful rule reload. Listing no longer evaluates JavaScript or consumes
JS worker slots. A time-dependent display name changes on the next successful
reload; actual feed configuration remains dynamically evaluated on generation.
Failed reloads preserve both the prior rules and their display metadata.
Each list handler admits two active renders and eight waiting renders. Saturation
returns 503, cancellation/deadline expiry returns 504, and output above 10 MiB
fails instead of returning a successful partial list. Legacy standalone list
helpers retain a cancellation-aware configuration adapter.

Safe rule loading accepts regular files only on supported Linux/macOS targets:

- Up to 256 rule groups/rule files, 1 MiB per rule and 16 MiB total source bytes.
- Validated display names are limited to 4096 UTF-8 bytes.
- A symbolic-link rule root (final path component), group directory or `.js`
  file is rejected. Use real directories/files or directory mounts instead.
- FIFO, device and other non-regular `.js` entries reject the reload immediately.
  Opens are descriptor-anchored, nonblocking and no-follow; the opened file's
  type and size are checked again before a bounded read. Growing files cannot
  bypass the read limit. A failed read retains the last published snapshot.

Direct browser-parser compatibility helpers use the same safe rule reader.
Regular-file reads check cancellation before and after reading; this does not
make an unresponsive network filesystem universally interruptible.

SSR, CSR and MIX cache hits now recheck the 10 MiB page budget. Oversized entries
are invalidated on a best-effort basis and treated as misses, then follow normal
bounded fetching. Cancellation during cache access prevents a new fetch. This
bounds values entering parsers; a cache backend may still allocate its stored
value while reading it, so it is not a hard bound on transient Redis response
memory.

## Date, category and executable-rule contracts

Date conversion now rejects invalid or unsupported text instead of returning a
fabricated timestamp. Valid dates before 1970 and Unix epoch values are
preserved. A month/day value such as `12月12日` uses the current local calendar
year; impossible dates, including February 29 in a non-leap year, are errors.
Dates without an explicit zone retain the runtime's local-time interpretation.

Supported date text includes strict ISO 8601 and RFC 2822, Chinese `YYYY年`,
`YYYY年M月`, `[YYYY年]M月D日` (optionally followed by `H:mm` or `H:mm:ss`), and
`YYYY/M/D` (optionally followed by a time). Relative forms accept nonnegative
integer amounts such as `10天前`, `2个月前`, `5 minutes ago` and `1 week ago`.
Fractional/signed relative amounts, malformed text and quoted source-like text
are rejected. Raw numeric strings are not a new Unix-timestamp input format.
Missing date fields remain optional because generation skips empty fields; a
nonempty invalid extracted date fails the feed with HTTP 500 instead of
publishing incorrect or partial content. The standalone conversion helpers
return an error for empty input as well.

Parsed item categories now reach all three formats: RSS item `category`, Atom
entry `category` and JSON Feed item `tags`. Empty categories remain omitted;
Unicode and XML/JSON escaping are handled by the output library.

Successful rule publication now requires an executable minimum configuration,
not only valid JavaScript/JSON:

- Every parsing mode requires an absolute HTTP(S) URL and a nonempty valid CSS
  `ListContainer`. Trusted feed rules can still use internal HTTP(S) origins;
  inspector public-network restrictions are separate.
- SSR and MIX additionally require a nonempty `Title` expression. Title and
  other extraction fields can be literals where supported, so they are not
  uniformly treated as CSS selectors. Direct parsing of supplied HTML validates
  scope/title without requiring an unused fetch URL.
- CSR keeps compatibility with custom `GetData` implementations and does not
  universally require Title/Link. Rules using the native SDK still need its
  title/link expressions to extract the intended data.
- Omitted Mode defaults to SSR; mode whitespace/case is normalized, while an
  explicitly unknown mode is rejected. Pager selectors and active readlink
  Object selectors are checked; unsupported hook actions are rejected. Empty
  readlink URL/Object retain the existing current-item-anchor/body defaults.

The contract is checked before initial publication, on reload and on dynamic
configuration evaluation. Invalid initial rules fail startup; invalid reloads
preserve previous source/display metadata; invalid dynamic configurations fail
feed generation rather than becoming successful empty subscriptions. A valid
selector matching no entries is still an allowed empty SSR feed; browser modes
need the readiness policy described below to distinguish empty from not-yet-loaded.

## Shared field expressions, CSR enrichment and browser readiness

SSR/MIX and the native CSR SDK now interpret extraction fields consistently:

- `literal:固定 作者` forces a literal value, including whitespace or CSS-like
  punctuation; `css:h2` forces a selector. The prefixes apply to Title, Author,
  Category, DateTime, Description, Link and readlink-hook URL expressions.
- Unprefixed known HTML tag names and CSS-looking strings retain selector
  behavior; other bare text is literal. Bare words are matched against complete
  tag names rather than substrings, so `Yahoo` is now a literal. Whitespace is
  selector-looking for legacy descendant selectors: use `literal:` for phrases.
- ListContainer, ReadySelector, Pager and hook Object remain plain selectors,
  without expression prefixes. Explicit invalid/empty `css:` expressions fail
  configuration validation. Required SSR/MIX title expressions cannot resolve
  to an empty literal. Optional `literal:` fields can intentionally be empty.

Custom SDK scripts keep the selector semantics of `GetText` and `GetLink`;
field-expression handling is used by the native extraction path. A native CSR
parser with matched entries but missing/empty Title or Link expressions now
reports an error instead of returning an empty success. A custom `GetData`
override may still define its own fields and output.

CSR output now passes through common Go-side enrichment. Relative links resolve
against the configured page URL. IdByRegexp uses the original item link and
fills only an absent ID. Native readlink hooks capture their per-item links in
the browser, then use the same ordered/cancelable detail workers, scoped
Cookie/proxy options and content budgets as SSR. Failures do not return partial
feeds. Each caller enriches its own decoded copy after shared raw extraction.

Custom output keeps its explicit ID and nonempty fields. For a custom item with
no captured DOM context, an empty hook URL can use the item link and a literal
hook URL can name a static target. A selector hook without captured DOM context
fails explicitly rather than being silently ignored. The browser cache namespace
is now v5 so older payloads without native enrichment metadata are not reused.

`ReadySelector` is a browser-only CSR/MIX setting. When omitted, it continues to
wait for ListContainer, preserving existing dynamic-page behavior. When a page
has a separate trustworthy loaded/empty marker, set ReadySelector to that CSS
selector (for example `#loaded` or `body` when document readiness is sufficient).
After that marker is ready, zero matching entries are a successful empty feed.
Do not use `body` for pages whose data is still loading after document readiness:
choose a marker that also represents their final empty state. Missing markers,
timeouts and cancellation remain failures, never automatic empty success. SSR
rejects ReadySelector because it does not execute the browser readiness phase.

## Main-document status, final URLs and Inspector response budgets

Browser feeds now apply the same HTTP success policy as SSR: the final top-level
main document must return 200. A rendered 404/503 page is rejected before raw
extraction enters the cache, including when ReadySelector would otherwise allow
an empty feed. HTTP redirects and client navigation while waiting for readiness
are correlated to the current main frame/loader; a failed script, image or
iframe alone does not turn a successful main document into an upstream failure.
A navigation that changes the document during extraction is rejected rather
than combining content and metadata from different documents.

Fetched documents retain final URL and status across cache hits. Relative item,
pagination and detail links use the actual document base, including the first
`base` element with an href. An invalid/non-HTTP(S) base falls back to the final
HTTP(S) URL; an invalid first base does not activate later base elements.
The configured original URL remains the Cookie authorization origin. Redirects
or document bases do not grant its credentials to a different host.

Document cache keys use v4 in v0.5.0 and browser extraction keys use v6. Their
versioned records include provenance; obsolete, malformed, unsuccessful or
oversized records are treated as misses and refetched. Decoded HTML/CSR data is
still capped at 10 MiB. Encoded records allow bounded JSON escaping/metadata
headroom (61 MiB, with URL metadata capped at 64 KiB); this does not increase the
decoded page budget. A memory backend may decline a large encoded record under
its separate per-entry budget while the fresh feed still succeeds.

Inspector intercepted responses now have a 10 MiB per-resource limit on both
wire and decoded data, a shared 32 MiB decoded response budget per page, and at
most eight active plus 32 queued interception tasks. Declared oversize responses
are rejected early; streaming/gzip responses are also bounded while reading.
Response bodies close on success, rejection and cancellation. Requests beyond
admission/size budgets fail explicitly and make the Inspector page operation
fail rather than returning a successful partial capture. Deliberately blocked
images/media/fonts do not count as budget errors for native feed extraction.
These limits cover Go-side interception buffers; they are not an operating-system
memory limit on the remote Chrome process.

## Detail content URLs and browser admission

Readlink detail extraction now resolves relative HTML `href` and `src` values
before Markdown conversion. The base comes from the detail response's final URL
and its first effective HTML `base[href]`; template contents do not supply a
base. The same behavior applies to fresh and cached source documents, and the
original Cookie authorization origin remains unchanged. Explicit URI schemes
retain their existing conversion semantics.

Browser execution now admits at most four active operations and 16 waiting
operations across the process. Inspector and CSR/MIX share this budget so
management requests cannot increase total Chrome concurrency. A full admission
queue fails immediately: Inspector and feed endpoints return HTTP 503. Waiting
requests remain cancelable and release admission on timeout; successful raw
browser cache hits do not need a Chrome slot. This is a shared capacity limit,
not a reserved management quota or a strict FIFO scheduling guarantee.

Chrome `/json/version` discovery responses have a separate 64 KiB decoded-body
limit, checked against actual bytes rather than relying on Content-Length.
Discovery requests use identity encoding and reject compressed responses.
Configured and advertised browser endpoint URLs are limited to 8 KiB, including
after replacing the advertised authority with the configured remote authority.
Oversized or malformed discovery responses fail before opening a websocket.
