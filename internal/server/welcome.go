package server

import (
	"embed"
	"io/fs"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
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

func welcomePage() gin.HandlerFunc {
	var homepage = GetPageByName("home.html")

	return func(c *gin.Context) {

		if define.DEBUG_MODE {
			file, err := os.ReadFile("internal/server/templates/home.html")
			if err == nil {
				c.Data(http.StatusOK, "text/html", file)
			} else {
				c.Data(http.StatusOK, "text/html", homepage)
				logger.Instance.Warnf("rendering template home failed: %v", err)
			}
		} else {
			c.Data(http.StatusOK, "text/html", homepage)
		}
	}
}
