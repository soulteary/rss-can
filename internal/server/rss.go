package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/soulteary/RSS-Can/internal/generator"
	"github.com/soulteary/RSS-Can/internal/rule"
)

type RSS struct {
	Type string `uri:"type" binding:"required"` // RSS Type
	ID   string `uri:"id" binding:"required"`
}

func apiRSS() gin.HandlerFunc {
	return func(c *gin.Context) {
		var rss RSS
		if err := c.ShouldBindUri(&rss); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"msg": err})
			return
		}

		ruleFile, exist := rule.GetRuleByName(rss.ID)
		if !exist {
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
	}
}
