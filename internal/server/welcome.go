package server

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
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
