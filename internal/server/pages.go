package server

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"github.com/soulteary/RSS-Can/internal/limits"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/network"
	"github.com/soulteary/RSS-Can/internal/parser"
	"github.com/soulteary/RSS-Can/internal/rule"
	"github.com/soulteary/RSS-Can/internal/version"
	"github.com/soulteary/goquery"
)

//go:embed assets
var embedAssets embed.FS

//go:embed templates
var pageTemplates embed.FS

func ServerAssets() fs.FS {
	assets, _ := fs.Sub(embedAssets, "assets")
	return assets
}

func GetPageByName(pageName string) (file []byte) {
	var err error
	file, err = pageTemplates.ReadFile(`templates/` + pageName)
	if err != nil {
		return file
	}
	return file
}

type pageView struct {
	options   Options
	snapshot  func() map[string]define.RuleCache
	configure func(context.Context, define.RuleCache) (define.JavaScriptConfig, error)
	metadata  func() map[string]rule.Metadata
	admission *listAdmission
}

func defaultPageView() pageView {
	return pageView{options: DefaultOptions(), snapshot: rule.Snapshot, configure: rule.GenerateConfigByRuleContext}
}
func UpdateHomePage(content []byte) []byte { return defaultPageView().updateHomePage(content) }
func welcomePage() fiber.Handler           { return defaultPageView().welcomePage() }
func UpdateListPage(content []byte) []byte {
	return defaultPageView().updateListPage(context.Background(), content)
}
func listPage() fiber.Handler { return defaultPageView().listPage() }

func (v pageView) updateHomePage(content []byte) []byte {
	body := bytes.ReplaceAll(content, []byte(`{%PROJECT_NAME%}`), []byte(`RSS Can / RSS 罐头`))
	body = bytes.ReplaceAll(body, []byte(`{%PROJECT_VERSION%}`), []byte(version.Version))
	body = bytes.ReplaceAll(body, []byte(`{%PROJECT_LIST_PAGE%}`), []byte(v.options.FeedPath))
	return body
}

func (v pageView) welcomePage() fiber.Handler {
	var homepage = v.updateHomePage(GetPageByName("home.html"))
	return func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		if v.options.Debug {
			file := fn.GetFileContent("internal/server/templates/home.html")
			if file != nil {
				return c.Status(http.StatusOK).Send(v.updateHomePage(file))
			} else {
				logger.Instance.Warnf("rendering template home failed")
				return c.Status(http.StatusOK).Send(homepage)
			}
		}
		return c.Status(http.StatusOK).Send(homepage)
	}
}

// Legacy callers retain their byte-only helper; production handlers use the
// error-returning renderer and validated metadata instead of executing rules.
func (v pageView) updateListPage(ctx context.Context, content []byte) []byte {
	body, _ := v.renderListPage(ctx, content)
	return body
}

func (v pageView) renderListPage(ctx context.Context, content []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries := map[string]rule.Metadata{}
	if v.metadata != nil {
		entries = v.metadata()
	} else {
		// Standalone compatibility path. Production always injects metadata from
		// the repository's last successfully validated transaction.
		for id, snapshot := range v.snapshot() {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			config, err := v.configure(ctx, snapshot)
			if err != nil {
				if canceled := ctx.Err(); canceled != nil {
					return nil, canceled
				}
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return nil, err
				}
				logger.Instance.Errorf("Parsing rule file failed: %v", err)
				continue
			}
			entries[id] = rule.Metadata{Name: config.Name, File: snapshot.File, Sign: snapshot.Sign}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ordered := make([]string, 0, len(entries))
	for id := range entries {
		ordered = append(ordered, id)
	}
	slices.Sort(ordered)
	var rows strings.Builder
	for i, id := range ordered {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entry := entries[id]
		filePrefix := strings.TrimPrefix(v.options.RuleDirectory, "./") + "/"
		sign := entry.Sign[:min(6, len(entry.Sign))]
		base := strings.TrimRight(v.options.FeedPath, "/") + "/" + url.PathEscape(id)
		links := make([]string, 0, 3)
		for _, format := range []string{"rss", "atom", "json"} {
			links = append(links, fmt.Sprintf(`<a target="_blank" href="%s"><span class="badge badge-sm badge-outline badge-warning">%s</span></a>`, html.EscapeString(base+"/"+format), strings.ToUpper(format)))
		}
		row := fmt.Sprintf("<tr><th>%d</th><td>%s</td><td>%s</td><td><span class=\"badge badge-sm badge-outline badge-success\">%s</span></td><td>%s</td><td>%s</td><td>%s</td></tr>", i+1, html.EscapeString(entry.Name), html.EscapeString(strings.TrimPrefix(entry.File, filePrefix)), html.EscapeString(sign), links[0], links[1], links[2])
		if len(row) > limits.MaxFeedBytes-rows.Len() {
			return nil, errors.New("rule list exceeds output size limit")
		}
		rows.WriteString(row)
	}
	body := bytes.ReplaceAll(content, []byte(`{%PROJECT_NAME%}`), []byte(`RSS Can / RSS 罐头`))
	body = bytes.ReplaceAll(body, []byte(`{%PROJECT_VERSION%}`), []byte(version.Version))
	body = bytes.ReplaceAll(body, []byte(`{%PROJECT_FEED_LIST%}`), []byte(rows.String()))
	if len(body) > limits.MaxFeedBytes {
		return nil, errors.New("rule list exceeds output size limit")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return body, nil
}

const ListConcurrency = 2
const ListQueueLimit = 8

var errListOverloaded = errors.New("rule list queue is full")

type listAdmission struct{ admitted, active chan struct{} }

func newListAdmission() *listAdmission {
	return &listAdmission{make(chan struct{}, ListConcurrency+ListQueueLimit), make(chan struct{}, ListConcurrency)}
}
func (a *listAdmission) acquire(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case a.admitted <- struct{}{}:
	default:
		return nil, errListOverloaded
	}
	select {
	case a.active <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-a.active
			<-a.admitted
			return nil, err
		}
		var once sync.Once
		return func() { once.Do(func() { <-a.active; <-a.admitted }) }, nil
	case <-ctx.Done():
		<-a.admitted
		return nil, ctx.Err()
	}
}

func (v pageView) listPage() fiber.Handler {
	admission := v.admission
	if admission == nil {
		admission = newListAdmission()
	}
	return func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), v.options.Timeout)
		defer cancel()
		release, err := admission.acquire(ctx)
		if err != nil {
			if errors.Is(err, errListOverloaded) {
				return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"msg": "rule list queue is full"})
			}
			return c.Status(http.StatusGatewayTimeout).JSON(fiber.Map{"msg": "rule list generation timed out"})
		}
		defer release()
		content := GetPageByName("list.html")
		if v.options.Debug {
			if file := fn.GetFileContent("internal/server/templates/list.html"); file != nil {
				content = file
			} else {
				logger.Instance.Warnf("rendering template list failed")
			}
		}
		body, err := v.renderListPage(ctx, content)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return c.Status(http.StatusGatewayTimeout).JSON(fiber.Map{"msg": "rule list generation timed out"})
			}
			logger.Instance.Errorf("Rendering rule list failed: %v", err)
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"msg": "rule list generation failed"})
		}
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Status(http.StatusOK).Send(body)
	}
}

func inspectorHome() fiber.Handler {
	return func(c fiber.Ctx) error {
		file := GetPageByName("inspector.html")
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Status(http.StatusOK).Send(file)
	}
}

func inspectorPrepare() fiber.Handler {
	return func(c fiber.Ctx) error {
		url := c.FormValue("url")
		// Validate before allocating browser resources.
		if network.PublicURL(url) != nil {
			return c.Redirect().Status(http.StatusFound).To("./inspector")
		}

		engine := c.FormValue("engine", define.DEFAULT_PARSE_MODE)
		if engine != define.PARSE_MODE_SSR && engine != define.PARSE_MODE_CSR && engine != define.PARSE_MODE_MIX {
			return c.Redirect().Status(http.StatusFound).To("./inspector")
		}

		target := fmt.Sprintf("./inspector/%s/", fn.Base64Encode(url))
		return c.Redirect().Status(http.StatusFound).To(target)
	}
}

func inspectorProxy() fiber.Handler { return inspectorProxyWithOptions(DefaultOptions()) }

func inspectorProxyWithOptions(options Options) fiber.Handler {
	return inspectorProxyWithFetcher(func(ctx context.Context, url string) (string, error) {
		return parser.ProxyPageByBrowserWithTimeout(ctx, url, options.Headless, options.Proxy, "", options.BrowserTimeout)
	})
}

func inspectorProxyWithFetcher(fetch func(context.Context, string) (string, error)) fiber.Handler {
	return func(c fiber.Ctx) error {
		encodedURL := c.Params("url")
		if encodedURL == "" {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"msg": "missing inspector URL"})
		}

		url := fn.Base64Decode(encodedURL)
		// TODO set with config or parameters
		if err := network.PublicURL(url); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"msg": "invalid inspector URL"})
		}
		rawHtml, err := fetch(c.Context(), url)
		if err != nil {
			if errors.Is(err, parser.ErrBrowserOverloaded) {
				return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"msg": "browser queue is full"})
			}
			return c.Status(http.StatusBadGateway).JSON(fiber.Map{"msg": "inspector fetch failed"})
		}
		html := parser.ParseFullPageByGoQuery(rawHtml, func(document *goquery.Document) string {
			document.Find("script").Remove()
			html, _ := document.Html()

			app := []byte(jssdk.FILE_INSPECTOR_JS)
			script := jssdk.GenerateInspector(app)

			html = strings.Replace(html, "</html>", script+"</html>", -1)
			return html
		})
		// Render fetched markup in an opaque origin: event handlers and frames
		// must not inherit the management service's origin or credentials.
		c.Set("Content-Security-Policy", "sandbox allow-scripts; default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline' http: https:; img-src data: http: https:; connect-src 'none'; frame-src 'none'; form-action 'none'; base-uri 'none'")
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Status(http.StatusOK).SendString(html)
	}
}
