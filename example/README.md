# RSS-Can deployment examples / 部署示例

All three Compose examples pin the published `soulteary/rss-can:v0.4.0` image. It includes Deno 2.9.8; no separate Deno installation, source build or runtime download is needed. These examples are deployment consumers, not instructions to rebuild or overwrite v0.3.12.

三个示例均固定正式 `v0.4.0` 镜像，内置 Deno 2.9.8，无需另装 Deno。升级前阅读[运行时、规则兼容和回退指南](../docs/upgrading.md)与 [v0.4.0 发布说明](../docs/releases/v0.4.0.md)。这次是运行时次版本升级，不是仅修改标签的补丁更新。

## Start / 启动

```sh
# Run from this checkout; choose one profile, not all three on port 8080.
docker compose -f example/1.standalone/docker-compose.yml up -d
# Or Redis-backed cache:
docker compose -f example/2.with-redis-cache/docker-compose.yml up -d
# Or Redis plus remote Chrome for CSR/MIX rules:
docker compose -f example/3.enable-csr-render/docker-compose.yml up -d
```

Compose resolves the read-only `../../rules:/rules:ro` bind relative to the selected Compose file. It uses this checkout's actual sample rules instead of silently creating an empty per-example `rules` directory. To use custom trusted rules, replace the bind source with their real directory; preserve regular files rather than symlinks. Ensure every configured rule passes the new validation before replacing a deployment.

挂载路径按各 Compose 文件所在目录解析，实际读取仓库 `rules`，不再自动创建空规则目录。自定义规则请改为真实目录，并保留只读挂载。Deno 会启动独立进程，部署环境需允许执行进程；CSR/MIX 仍需 Chrome，Deno 并不替代浏览器。

## Upgrade and verify / 升级验收

Record the old image digest, Compose environment and rule directory; back up custom rules and any persistent Redis configuration/data. Run only one example per host port or change its mapping. Pull and recreate the selected profile, then verify `http://127.0.0.1:8080/_/health/` and `/_/ready/`, the subscription list and representative RSS/Atom/JSON feeds. Verify both a first request and a cached request with Redis; CSR/MIX deployments must exercise a real browser-rendered rule. These minimal examples publish port 8080 on all host interfaces: restrict network exposure or use a reverse proxy before deploying outside a trusted environment.

v0.3.12 uses the old embedded runtime, while v0.4.0 uses Deno and stricter rule/resource boundaries. Invalid rules or hooks now fail explicitly; old cache entries may be refetched. Rehearse with copies before routing traffic. On rollback, stop the new service, restore the prior image and matching rule/configuration backup, and treat cache contents as disposable rather than assuming cross-runtime cache compatibility. Keep Redis and browser services scoped to the Compose network; do not publish their ports just to run these examples.

升级前记录旧镜像摘要、环境和规则目录并备份；先用隔离副本演练规则与真实订阅输出。验收健康/就绪、列表、RSS/Atom/JSON、Redis 缓存和浏览器规则，再切换流量。回退时停新版，恢复旧镜像及对应规则/配置备份；缓存可重建，不保证跨运行时复用。

```sh
docker compose -f example/1.standalone/docker-compose.yml down
```

Use the same selected file for cleanup. Keep custom data and backups; `down -v` is not a routine upgrade step. The minimal Redis example has no persistent volume, so recreating its Redis container discards cached values.
