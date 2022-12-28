package server

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gin-gonic/gin"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/parser"
	"github.com/soulteary/RSS-Can/internal/rule"
	"github.com/soulteary/RSS-Can/internal/version"
)

//go:embed assets
var embedAssets embed.FS

//go:embed templates
var pageTemplates embed.FS

func ServerAssets() http.FileSystem {
	assets, _ := fs.Sub(embedAssets, "assets")
	return http.FS(assets)
}

func GetPageByName(pageName string) (file []byte) {
	var err error
	file, err = pageTemplates.ReadFile(`templates/` + pageName)
	if err != nil {
		return file
	}
	return file
}

func UpdateHomePage(content []byte) []byte {
	body := bytes.ReplaceAll(content, []byte(`{%PROJECT_NAME%}`), []byte(`RSS Can / RSS 罐头`))
	body = bytes.ReplaceAll(body, []byte(`{%PROJECT_VERSION%}`), []byte(version.Version))
	body = bytes.ReplaceAll(body, []byte(`{%PROJECT_LIST_PAGE%}`), []byte(GetFeedPath()))
	return body
}

func welcomePage() gin.HandlerFunc {
	var homepage = UpdateHomePage(GetPageByName("home.html"))
	return func(c *gin.Context) {
		if define.DEBUG_MODE {
			file, err := os.ReadFile("internal/server/templates/home.html")
			if err == nil {
				c.Data(http.StatusOK, "text/html", UpdateHomePage(file))
			} else {
				c.Data(http.StatusOK, "text/html", homepage)
				logger.Instance.Warnf("rendering template home failed: %v", err)
			}
		} else {
			c.Data(http.StatusOK, "text/html", homepage)
		}
	}
}

func UpdateListPage(content []byte) []byte {
	body := bytes.ReplaceAll(content, []byte(`{%PROJECT_NAME%}`), []byte(`RSS Can / RSS 罐头`))
	baseLink := GetFeedPath()

	// TODO Pre-execute configuration for caching
	// TODO fixed sort

	id := 1
	tpl := ""
	for dirName, RuleFile := range rule.RulesCache {
		rssLink := fmt.Sprintf(`<a target="_blank" href="%s"><span class="badge badge-sm badge-outline badge-warning">RSS</span></a>`, baseLink+`/`+dirName+`/rss`)
		atomLink := fmt.Sprintf(`<a target="_blank" href="%s"><span class="badge badge-sm badge-outline badge-warning">ATOM</span></a>`, baseLink+`/`+dirName+`/atom`)
		jsonLink := fmt.Sprintf(`<a target="_blank" href="%s"><span class="badge badge-sm badge-outline badge-warning">JSON</span></a>`, baseLink+`/`+dirName+`/json`)
		tpl = tpl + fmt.Sprintf(`
<th>%d</th>
	<td>%s</td>
	<td>%s</td>
	<td>%s</td>
	<td>%s</td>
	<td>%s</td>
</tr>`, id, dirName, RuleFile, rssLink, atomLink, jsonLink)
		id++
	}
	body = bytes.ReplaceAll(body, []byte(`{%PROJECT_FEED_LIST%}`), []byte(tpl))

	return body
}

func listPage() gin.HandlerFunc {
	var homepage = UpdateListPage(GetPageByName("list.html"))
	return func(c *gin.Context) {
		if define.DEBUG_MODE {
			file, err := os.ReadFile("internal/server/templates/list.html")
			if err == nil {
				c.Data(http.StatusOK, "text/html", UpdateListPage(file))
			} else {
				c.Data(http.StatusOK, "text/html", homepage)
				logger.Instance.Warnf("rendering template home failed: %v", err)
			}
		} else {
			c.Data(http.StatusOK, "text/html", homepage)
		}
	}
}

func inspectorHome() gin.HandlerFunc {
	return func(c *gin.Context) {
		file, _ := os.ReadFile("internal/server/templates/inspector.html")
		c.Data(http.StatusOK, "text/html", file)
	}
}

func inspectorPrepare() gin.HandlerFunc {
	return func(c *gin.Context) {
		url := c.PostForm("url")
		// todo check url vaild
		if url == "" {
			c.Redirect(http.StatusFound, "./inspector")
			c.Abort()
			return
		}

		engine := c.DefaultPostForm("engine", define.DEFAULT_PARSE_MODE)
		if engine != define.PARSE_MODE_SSR && engine != define.PARSE_MODE_CSR && engine != define.PARSE_MODE_MIX {
			c.Redirect(http.StatusFound, "./inspector")
			c.Abort()
			return
		}

		target := fmt.Sprintf("./inspector/%s/", fn.Base64Encode(url))
		c.Redirect(http.StatusFound, target)
	}
}

func inspectorProxy() gin.HandlerFunc {
	type Params struct {
		URL string `uri:"url" binding:"required"`
	}

	return func(c *gin.Context) {
		var params Params
		if err := c.ShouldBindUri(&params); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"msg": err})
			return
		}

		url := fn.Base64Decode(params.URL)
		// TODO set with config or parameters
		rawHtml := parser.ProxyPageByGoRod(url, define.HEADLESS_SERVER, define.PROXY_SERVER)
		html := parser.ParseFullPageByGoQuery(rawHtml, func(document *goquery.Document) string {
			document.Find("script").Remove()
			html, _ := document.Html()

			hep, _ := os.ReadFile("internal/server/assets/js/html-element-picker/hep.js")

			script := fmt.Sprintf(`
<script>
%s
hep = new ElementPicker();

document.addEventListener('click', function(e){
	console.log(hep.selectors)
	e.preventDefault();
	e.stopPropagation();
});
</script>
`, hep)
			html = strings.Replace(html, "</html>", script+"</html>", -1)
			return html
		})
		c.Data(http.StatusOK, "text/html", []byte(html))
	}
}
