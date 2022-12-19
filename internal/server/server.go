package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/soulteary/RSS-Can/internal/generator"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/rule"
)

func makeMap(list []string) map[string]string {
	result := make(map[string]string)
	for _, s := range list {
		result[strings.Split(s, "/")[1]] = s
	}
	return result
}

func ServAPI() {

	// TODO dynamic refresh rules
	rules := rule.LoadRules()
	rulesAlived := makeMap(rules)

	type RSS struct {
		Type string `uri:"type" binding:"required"` // RSS Type
		ID   string `uri:"id" binding:"required"`
	}

	route := gin.Default()
	route.GET("/:id/:type/", func(c *gin.Context) {
		var rss RSS
		if err := c.ShouldBindUri(&rss); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"msg": err})
			return
		}

		ruleFile, ok := rulesAlived[rss.ID]
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"msg": "rule not found"})
			return
		}

		config, err := rule.GenerateConfigByRule(ruleFile)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"msg": "parse config failed"})
			return
		}

		data := rule.GetWebsiteDataWithConfig(config)

		var response string
		var mimetype string
		switch strings.ToUpper(rss.Type) {
		case "RSS":
			mimetype = "application/rss+xml"
			response = generator.GenerateFeedsByType(config, data, "RSS")
		case "ATOM":
			mimetype = "application/atom+xml"
			response = generator.GenerateFeedsByType(config, data, "ATOM")
		case "JSON":
			mimetype = "application/feed+json"
			response = generator.GenerateFeedsByType(config, data, "JSON")
		}
		c.Data(http.StatusOK, mimetype, []byte(response))
	})

	type DynamicConfig struct {
		Type  string `uri:"type" binding:"required"`
		Value string `uri:"value" binding:"required"`
	}

	route.GET("/config/:type/:value/", func(c *gin.Context) {
		var config DynamicConfig
		if err := c.ShouldBindUri(&config); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"msg": err})
			return
		}

		if strings.ToLower(config.Type) == "set-loglevel" {
			logLevel := strings.ToLower(config.Value)
			if (logLevel == "debug") || (logLevel == "info") || (logLevel == "warn") || (logLevel == "error") {
				logger.SetLevel(logLevel)
			}

		}
	})

	route.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html", []byte(welcomePageForTest))
	})

	route.Run(":8080")
}
