# RSS Can / RSS 罐头

[![CodeQL](https://github.com/soulteary/RSS-Can/actions/workflows/codeql.yml/badge.svg)](https://github.com/soulteary/RSS-Can/actions/workflows/codeql.yml) ![Go Report Card](https://goreportcard.com/badge/github.com/soulteary/RSS-Can)

<p style="text-align: center;">
  <a href="README.md" target="_blank">ENGLISH</a> | <a href="README_CN.md">中文文档</a>
</p>

📰 🥫 **做更好的 RSS 聚合方案**

<p style="text-align: center;">
  <img src="./assets/images/project.jpg" width="300">
</p>

*图片由 stable diffusion 生成*

## 支持软硬件环境

- Linux: AMD64(x86_64)
- macOS: AMD64(x86_64) /  ARMv64

## 使用方法

从 GitHub 的软件发布页面，[下载软件](https://github.com/soulteary/RSS-Can/releases)之后，直接运行就可以啦：

```bash
./rssc
```

### Docker 容器方式运行

使用下面的命令，下载最新版本的软件之后，使用 `docker run` 运行即可：

```
docker pull soulteary/rss-can:0.2.0
docker run --rm -it -p 8080:8080 soulteary/rss-can:0.2.0
```

### 支持的命令行参数及环境变量

**所有的参数都是可选使用，根据自己的实际需要来即可。**

想要获取程序支持的参数，可以通过在执行程序后添加参数 `-h` 或者 `--help`：
 
```bash
Usage of RSS-Can:
  -debug RSS_DEBUG
    	whether to output debugging logging, env: RSS_DEBUG
  -debug-level RSS_DEBUG_LEVEL
    	set debug log printing level, env: RSS_DEBUG_LEVEL (default "info")
  -headless-addr RSS_HEADLESS_SERVER
    	set Headless server address, env: RSS_HEADLESS_SERVER (default "127.0.0.1:9222")
  -headless-slow-motion RSS_HEADLESS_SLOW_MOTION
    	set Headless slow motion, env: RSS_HEADLESS_SLOW_MOTION (default 2)
  -memory RSS_MEMORY
    	using Memory(build-in) as a cache service, env: RSS_MEMORY (default true)
  -memory-expiration RSS_MEMORY_EXPIRATION
    	set Memory cache expiration, env: RSS_MEMORY_EXPIRATION (default 600)
  -port RSS_PORT
    	web service listening port, env: RSS_PORT (default 8080)
  -proxy RSS_PROXY
    	Proxy, env: RSS_PROXY
  -redis RSS_REDIS
    	using Redis as a cache service, env: RSS_REDIS (default true)
  -redis-addr RSS_SERVER
    	set Redis server address, env: RSS_SERVER (default "127.0.0.1:6379")
  -redis-db RSS_REDIS_DB
    	set Redis db, env: RSS_REDIS_DB
  -redis-pass RSS_REDIS_PASSWD
    	set Redis password, env: RSS_REDIS_PASSWD
  -rod string
    	Set the default value of options used by rod.
  -rule RSS_RULE
    	set Rule directory, env: RSS_RULE (default "./rules")
  -timeout-headless RSS_HEADLESS_EXEC_TIMEOUT
    	set headless execution timeout, env: RSS_HEADLESS_EXEC_TIMEOUT (default 5)
  -timeout-js RSS_JS_EXEC_TIMEOUT
    	set js sandbox code execution timeout, env: RSS_JS_EXEC_TIMEOUT (default 200)
  -timeout-request RSS_REQUEST_TIMEOUT
    	set request timeout, env: RSS_REQUEST_TIMEOUT (default 5)
  -timeout-server RSS_SERVER_TIMEOUT
    	set web server response timeout, env: RSS_SERVER_TIMEOUT (default 8)
```

## 项目计划

- [x] 2022.12.22 程序支持参数化调用，发布版本 v0.2.0。
- [x] 2022.12.21 支持跨多个页面聚合信息为 RSS 订阅源，完成第一个版本的 JS SDK，发布 v0.1.0 版本程序和应用 Docker 镜像。
- [x] 2022.12.20 支持使用 Redis 和 应用内存 作为数据缓存，避免大量不必要的网络请求造成的麻烦，支持动态加载 RSS 规则文件。
- [x] 2022.12.19 支持自动解析目标网站的网页编码格式，支持混合解析模式，提供比 CSR 解析模式更快的处理速度，支持从其他页面抽取数据装填 RSS 列表页面数据。
- [x] 2022.12.15 支持使用 CSR 解析模式处理数据，[博客](https://soulteary.io/2022/12/15/rsscan-use-golang-rod-to-parse-the-content-dynamically-rendered-in-the-browser-part-4.html)
- [x] 2022.12.14 支持将网站数据转换为可订阅的 RSS 订阅源, [博客](https://soulteary.com/2022/12/14/rsscan-convert-website-information-stream-to-rss-feed-part-3.html)
- [x] 2022.12.13 支持“动态化”能力，[博客](https://soulteary.com/2022/12/13/rsscan-make-golang-applications-with-v8-part-2.html)
- [x] 2022.12.12 支持使用 SSR 解析模式处理数据，[博客](https://soulteary.com/2022/12/12/rsscan-better-rsshub-service-build-with-golang-part-1.html)

- [ ] 文档: 提供简单的教程和文档，阐述如何使用常见技术栈来玩转 RSS Can。
- [ ] Golang: 为 Golang 1.19 进一步优化代码。
- [ ] Pipeline: 支持 RSS 信息流水线，能够定制信息处理任务，以及提供集成到各种开源软件的能力。
- [ ] AI: NLP 任务的集成和使用。
- [ ] 规则: 能够将社区两款软件的规则导入程序： [rss-bridge](https://github.com/RSS-Bridge/rss-bridge/tree/master/bridges) / [RSSHub](https://github.com/DIYgod/RSSHub/tree/master/lib)
- [ ] 工具: 支持通过界面工具快速生成规则，或参考: [damoeb/rss-proxy](https://github.com/damoeb/rss-proxy)


## License & Credits

This project is licensed under the [MIT License](https://github.com/soulteary/RSS-Can/blob/main/LICENSE)

- [@PuerkitoBio](https://github.com/PuerkitoBio), He implements a good DOM parsing tool library [goquery](https://github.com/PuerkitoBio/goquery) for Go under the [BSD-3-Clause license](https://github.com/PuerkitoBio/goquery/blob/master/LICENSE). In the project, it is used as a SSR method to parse remote document data. Because there is no Release for the new version, the code base used by the project is [[#3b7929a](https://github.com/PuerkitoBio/goquery/commit/3b7929a0d759a20968ba605c56bc3027c30d3527)].
- [@andybalholm](https://github.com/andybalholm), He implements a Go implementation of a CSS selector library [cascadia](https://github.com/andybalholm/cascadia), which is the core dependency of goquery under the [BSD-2-Clause license](https://github.com/andybalholm/cascadia/blob/master/LICENSE). Because there is no Release for the new version, the code base used by the project is [[#c6065e4](https://github.com/andybalholm/cascadia/commit/c6065e4618b7f538edf5ca0d6b5b2fd0fe129fdd)]
- [@rogchap](https://github.com/rogchap), He implements a good JavaScript runtime library [https://github.com/rogchap/v8go](https://github.com/rogchap/v8go) under the [BSD-3-Clause license](https://github.com/rogchap/v8go/blob/master/LICENSE). In the project, it used as a dynamic configuration execution sandbox environment with version [[v0.7.0](https://github.com/rogchap/v8go/releases/tag/v0.7.0)].
- [@gorilla](https://github.com/gorilla), Gorilla Web Toolkit Dev Team, they offer an amazing library of great tools, eg. [gorilla/feeds](https://github.com/gorilla/feeds) an tiny RSS generator library under the [BSD-2-Clause license](https://github.com/gorilla/feeds/blob/master/LICENSE). In the project, it is used as RSS generator. Sadly, the team decided to archive all projects on December 9th, 2022, the code base used by the project is [#b60f215](https://github.com/gorilla/feeds/commit/b60f215f72c708b0800622c804167bea85539ea5).
- [@gin-gonic](https://github.com/gin-gonic), Gin-Gonic Dev Team, they offer an great HTTP web framework [gin](https://github.com/gin-gonic/gin) under the [MIT license](https://github.com/gin-gonic/gin/blob/master/LICENSE). In the project, it used as Web Server to provides RSS API. The code base is [v1.8.1](https://github.com/gin-gonic/gin/releases/tag/v1.8.1).
- [@go-rod](https://github.com/go-rod/rod), Go-Rod Dev Team, they offer an tiny and high-performance CDP driver [go-rod](https://github.com/go-rod/rod) under the [MIT license](https://github.com/go-rod/rod/blob/master/LICENSE). In the project, it is used as CSR parser processing the content dynamically rendered in the browser. The code base is [v0.112.2](https://github.com/go-rod/rod/releases/tag/v0.112.2).
- [@jquery](https://github.com/jquery/jquery), the them offer an great JavaScript library [jquery](https://github.com/jquery/jquery) under the [MIT license](https://github.com/jquery/jquery/blob/main/LICENSE.txt). In the project, it used as CSR in-browser helper, to helper user simply complete the element positioning and information processing in the page. The code is [v1.12.4](https://github.com/jquery/jquery/releases/tag/1.12.4), avoid affecting the execution of the original program of the page after injecting the page, if the page also relies on the same program.
- [@JohannesKaufmann](https://github.com/JohannesKaufmann), He implements a good HTML to Markdown converter [html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown) under the [MIT license](https://github.com/JohannesKaufmann/html-to-markdown/blob/master/LICENSE). In the project, it used for enhanced content processing in the SSR phase to generate content in a clean Markdown format. The code base is [v1.3.6](https://github.com/JohannesKaufmann/html-to-markdown/releases/tag/v1.3.6).
- [@muesli](https://github.com/muesli), He implements a good concurrency-safe Go caching library [cache2go](https://github.com/muesli/cache2go) under the [BSD-3-Clause license](https://github.com/muesli/cache2go/blob/master/LICENSE.txt). In the project, it used for in-memory cache. The code base is [#518229c](https://github.com/muesli/cache2go/commit/518229cd8021d8568e4c6c13743bb050dc1f3a05).
- [@go-redis](https://github.com/go-redis/redis), The Go-Redis Dev Team, they offer a type-safe Redis client Go library [redis](https://github.com/go-redis/redis) under the [BSD-2-Clause license](https://github.com/go-redis/redis/blob/master/LICENSE). In the project, it used for redis cache. The code base is [v8.11.5](https://github.com/go-redis/redis/releases/tag/v8.11.5).
