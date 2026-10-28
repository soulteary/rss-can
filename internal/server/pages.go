package server

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"slices"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/jssdk"
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

func (v pageView) updateListPage(ctx context.Context, content []byte) []byte {
	body := bytes.ReplaceAll(content, []byte(`{%PROJECT_NAME%}`), []byte(`RSS Can / RSS 罐头`))
	body = bytes.ReplaceAll(body, []byte(`{%PROJECT_VERSION%}`), []byte(version.Version))

	baseLink := v.options.FeedPath

	// TODO Pre-execute configuration for caching

	id := 1
	tpl := ""

	filePathFix := strings.Replace(v.options.RuleDirectory, "./", "", -1) + "/"

	rules := v.snapshot()
	rulesOrdered := []string{}
	for dirName := range rules {
		rulesOrdered = append(rulesOrdered, dirName)
	}
	slices.Sort(rulesOrdered)

	for _, dirName := range rulesOrdered {
		RuleFile, exist := rules[dirName]
		if !exist {
			continue
		}

		config, err := v.configure(ctx, RuleFile)
		if err != nil {
			logger.Instance.Errorf("Parsing rule file failed: %v", err)
			continue
		}

		rssLink := fmt.Sprintf(`<a target="_blank" href="%s"><span class="badge badge-sm badge-outline badge-warning">RSS</span></a>`, baseLink+`/`+dirName+`/rss`)
		atomLink := fmt.Sprintf(`<a target="_blank" href="%s"><span class="badge badge-sm badge-outline badge-warning">ATOM</span></a>`, baseLink+`/`+dirName+`/atom`)
		jsonLink := fmt.Sprintf(`<a target="_blank" href="%s"><span class="badge badge-sm badge-outline badge-warning">JSON</span></a>`, baseLink+`/`+dirName+`/json`)
		tpl = tpl + fmt.Sprintf(`
<th>%d</th>
<td>%s</td>
<td>%s</td>
<td><span class="badge badge-sm badge-outline badge-success">%s</span></td>
<td>%s</td>
<td>%s</td>
<td>%s</td>
</tr>`, id, config.Name, strings.Replace(RuleFile.File, filePathFix, "", -1), RuleFile.Sign[0:6], rssLink, atomLink, jsonLink)
		id++
	}
	body = bytes.ReplaceAll(body, []byte(`{%PROJECT_FEED_LIST%}`), []byte(tpl))

	return body
}

func (v pageView) listPage() fiber.Handler {

	return func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		if v.options.Debug {
			file := fn.GetFileContent("internal/server/templates/list.html")
			if file != nil {
				return c.Status(http.StatusOK).Send(v.updateListPage(c.Context(), file))
			} else {
				logger.Instance.Warnf("rendering template list failed")
				return c.Status(http.StatusOK).Send(v.updateListPage(c.Context(), GetPageByName("list.html")))
			}
		}
		return c.Status(http.StatusOK).Send(v.updateListPage(c.Context(), GetPageByName("list.html")))
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
		rawHtml, err := parser.ProxyPageByBrowserWithTimeout(c.Context(), url, options.Headless, options.Proxy, "", options.BrowserTimeout)
		if err != nil {
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
