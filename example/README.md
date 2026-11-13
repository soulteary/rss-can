# RSS-Can deployment examples / 部署示例

These three examples run the pinned `soulteary/rss-can:v0.5.1` image in **standalone mode**. They include Deno 2.9.8 and do not enable per-user source permissions. For shared Stargate/Warden hosting, use the [team deployment guide](4.with-stargate/README.md).

三个独立部署示例固定 `v0.5.1` 镜像，内置 Deno 2.9.8，无需另装运行时。多人使用时，请选择[Stargate 团队部署](4.with-stargate/README.md)，不要直接公开独立模式服务。升级前阅读[迁移和回退说明](../docs/upgrading.md)、[v0.5.0 发布说明](../docs/releases/v0.5.0.md)与 [v0.5.1 发布说明](../docs/releases/v0.5.1.md)。

## Start / 启动

```sh
# Run from this checkout; choose one profile, not all three on port 8080.
docker compose -f example/1.standalone/docker-compose.yml up -d
# Or Redis-backed cache:
docker compose -f example/2.with-redis-cache/docker-compose.yml up -d
# Or Redis plus remote Chrome for CSR/MIX rules:
docker compose -f example/3.enable-csr-render/docker-compose.yml up -d
```

The image and explicit `user: "65532:65532"` setting run the service as non-root. Rule files must be readable by that numeric user.

Compose resolves the read-only `../../rules:/rules:ro` bind relative to the selected Compose file. It uses this checkout's actual sample rules instead of silently creating an empty per-example `rules` directory. To use custom trusted rules, replace the bind source with their real directory; preserve regular files rather than symlinks. Ensure every configured rule passes the new validation before replacing a deployment.

挂载路径按各 Compose 文件所在目录解析，实际读取仓库 `rules`，不再自动创建空规则目录。自定义规则请改为真实目录，并保留只读挂载。Deno 会启动独立进程，部署环境需允许执行进程；CSR/MIX 仍需 Chrome，Deno 并不替代浏览器。第三个示例等待浏览器的 `/json/version` 健康检查通过后才启动 RSS 服务，避免容器刚启动时首个浏览器请求连接被拒绝。

The CSR profile waits for the browser CDP discovery endpoint to return HTTP 200 before starting RSS-Can; a running container alone does not establish browser readiness.

## Upgrade and verify / 升级验收

Record the old image digest, Compose environment and rule directory; back up custom rules and any persistent Redis configuration/data. Run only one example per host port or change its mapping. Pull and recreate the selected profile, then verify `http://127.0.0.1:8080/_/health/` and `/_/ready/`, the subscription list and representative RSS/Atom/JSON feeds. Verify both a first request and a cached request with Redis; CSR/MIX deployments must exercise a real browser-rendered rule. These examples bind port 8080 to loopback. Keep the backend private and use an authenticated reverse proxy for any remote access; loopback binding does not add per-source authorization.

v0.3.12 uses the old embedded runtime; v0.4.0 introduced Deno, and v0.5.0 adds content sanitization, snapshot caching and optional team permissions. Historical v0.4.0 ignores team settings and must never replace a team deployment behind its existing gateway. Invalid rules or hooks now fail explicitly; old cache entries may be refetched. Rehearse with copies before routing traffic. On rollback, stop the new service, restore the prior image and matching rule/configuration backup, and treat cache contents as disposable rather than assuming cross-runtime cache compatibility. Keep Redis and browser services scoped to the Compose network; do not publish their ports just to run these examples.

升级前记录旧镜像摘要、环境和规则目录并备份；先用隔离副本演练规则与真实订阅输出。验收健康/就绪、列表、RSS/Atom/JSON、Redis 缓存和浏览器规则，再切换流量。回退时停新版，恢复旧镜像及对应规则/配置备份；缓存可重建，不保证跨运行时复用。

```sh
docker compose -f example/1.standalone/docker-compose.yml down
```

Use the same selected file for cleanup. Keep custom data and backups; `down -v` is not a routine upgrade step. The minimal Redis example has no persistent volume, so recreating its Redis container discards cached values.
