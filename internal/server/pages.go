package server

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
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
