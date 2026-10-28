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
