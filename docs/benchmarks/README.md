# JavaScript 执行后端性能报告

测试日期：2026-10-28。环境：Apple M5、32 GiB、macOS 26.5.1、Go 1.27.3。
比较 v8go v0.9.0、Deno 2.9.8 每次启动版本及单个常驻 worker。

- [每次启动 Deno 与 v8go 的对比](deno-v8go.md)：初始耗时、内存与启动开销。
- [常驻 worker 实现与复测](deno-worker.md)：并发隔离验证、三后端复测、持续内存采样与重跑方法。
- [初始逐次测量](2026-10-28-deno-v8go.json)。
- [三后端复测原始数据](2026-10-28-deno-worker.json)。
- [Deno 持续运行采样](2026-10-28-deno-worker-soak.json)。
- [v8go 持续运行采样](2026-10-28-v8go-soak.json)。

基准工具位于 `scripts/bench-jssdk/`。耗时数据不包含网络抓取；持续运行窗口为
3 分钟加 30 秒空闲，不代表数天运行的泄漏保证。RSS 包含进程树，共享页可能
重复计算。当前默认执行方案与部署要求见 [运行时说明](../deno-prototype.md)。
