# Credits

RSS Can uses the following open source projects. Go modules are resolved from
upstream releases through `go.mod` and verified with `go.sum`; the historical
source copies under `pkg/` are no longer used or maintained here.

- [goquery](https://github.com/PuerkitoBio/goquery), by PuerkitoBio: HTML traversal for SSR parsing, v1.13.0, BSD-3-Clause.
- [cascadia](https://github.com/andybalholm/cascadia), by Andy Balholm: CSS selectors, v1.3.5, BSD-2-Clause.
- [v8go](https://github.com/rogchap/v8go), by rogchap and contributors: JavaScript execution, v0.9.0, BSD-3-Clause.
- [gorilla/feeds](https://github.com/gorilla/feeds), by the Gorilla Web Toolkit team: RSS, Atom and JSON Feed generation, v1.2.0, BSD-2-Clause.
- [Gin](https://github.com/gin-gonic/gin), by the Gin team: HTTP service, v1.12.0, MIT.
- [gin-contrib/gzip](https://github.com/gin-contrib/gzip): response compression, v1.2.8, MIT.
- [Rod](https://github.com/go-rod/rod), by the go-rod team: Chrome DevTools Protocol automation, v0.116.2, MIT.
- [html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown), by Johannes Kaufmann and contributors: Markdown conversion, v2.5.2, MIT.
- [cache2go](https://github.com/muesli/cache2go), by muesli and contributors: in-memory caching, commit `518229cd8021`, BSD-3-Clause.
- [go-redis](https://github.com/redis/go-redis), by the Redis Go team: Redis caching, v9.23.0, BSD-2-Clause.
- [Zap](https://github.com/uber-go/zap), by Uber and contributors: structured logging, v1.28.0, MIT.
- [jQuery](https://jquery.com/), by the jQuery team: browser-side DOM traversal for rules, v4.0.0, MIT. The SDK restores page-owned `$` and `jQuery` globals after injection.
- [Moment](https://momentjs.com/), by the Moment team: date parsing, v2.31.0, MIT.
- [Tailwind CSS](https://github.com/tailwindlabs/tailwindcss), by Tailwind Labs: existing Web UI stylesheet, v2.2.0, MIT.
- [daisyUI](https://github.com/saadeghi/daisyui), by saadeghi and contributors: existing Web UI components, v2.46.0, MIT.
- [html-element-picker](https://github.com/AlienKevin/html-element-picker), by AlienKevin: inspector element selection, commit `f4555ae1a7ab3887ff2648f4fd7a4167bb36b883`, MIT.

Embedded jQuery and Moment assets retain their upstream license text alongside
the scripts in `internal/jssdk/js/`. Their source packages, versions, SHA-512
package integrity values and SHA-256 script checksums are recorded in
[`dependencies.json`](internal/jssdk/js/dependencies.json).

The license notices formerly kept with the local Go source copies are preserved
in [`third_party/licenses/`](third_party/licenses/), using the versions now selected
in `go.mod`.

See [dependency maintenance](DEPENDENCIES.md) for upgrade constraints and checks.

Project images were generated with Stable Diffusion. Thanks to all contributors.
