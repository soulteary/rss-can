# Runtime installation and upgrade guide / 运行时与升级指南

## Which distribution am I running? / 先确认版本

Published **v0.3.12** archives and images use the historical embedded runtime.
They do not require Deno. Current source and future releases built from it use
Deno **2.9.8**, with CGO disabled in Go release binaries. Changes under
`CHANGELOG.md` → `Unreleased` do not change an existing published tag.
Never replace a published release or its images to apply these changes.

已发布的 **v0.3.12** 仍使用旧的内嵌运行时，无需 Deno。当前源码以及由它构建的后续版本
使用 Deno **2.9.8**。`Unreleased` 中的改动不会自动进入已发布版本；升级时使用新版本
或明确的源码构建镜像，不覆盖历史发布物。

## Standalone / 独立二进制

Install Deno 2.9.8 for the same OS/architecture as `rssc`. The executable lookup
order is `RSS_CAN_DENO`, an executable `deno` beside `rssc`, then `deno` on PATH.
An explicit override that does not work is an error; it is never silently ignored.
The startup error explains how to supply the dependency.

安装与 `rssc` 平台、架构相符的 Deno 2.9.8。查找顺序为 `RSS_CAN_DENO`、`rssc` 同目录
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
docker run --rm -p 8080:8080 -e RSS_REDIS=false rss-can:local
```

The image includes pinned Deno and example rules. Mount custom rules at `/rules`.
Do not assume a standalone archive includes Deno: its Go binary, rules, notices
and this guide are packaged together; the runtime remains a separate install.

源码镜像内置 Deno 和示例规则，自定义规则挂载到 `/rules`。独立归档附带升级说明，
但不附带 Deno，需要单独安装或放在 `rssc` 同目录。

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

## Architecture hardening in source builds

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
