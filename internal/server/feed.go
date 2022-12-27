package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/generator"
	"github.com/soulteary/RSS-Can/internal/rule"
)

func generateFeedResponse(config define.JavaScriptConfig, data define.BodyParsed, rssType string) (mimetype string, response string) {
	switch rssType {
	case define.FEED_TYPE_RSS:
		mimetype = define.FEED_MIME_TYPE_RSS
		response = generator.GenerateFeedsByType(config, data, rssType)
	case define.FEED_TYPE_ATOM:
		mimetype = define.FEED_MIME_TYPE_ATOM
		response = generator.GenerateFeedsByType(config, data, rssType)
	case define.FEED_TYPE_JSON:
		mimetype = define.FEED_MIME_TYPE_JSON
		response = generator.GenerateFeedsByType(config, data, rssType)
	default:
		mimetype = define.FEED_MIME_TYPE_JSON
		response = "incorrect type"
	}
	return mimetype, response
}

func apiRSS() gin.HandlerFunc {
	type Params struct {
		Type string `uri:"type" binding:"required"` // RSS Type
		ID   string `uri:"id" binding:"required"`
	}

	return func(c *gin.Context) {
		var rss Params
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
		if data.Code != define.ERROR_CODE_NULL {
			c.JSON(http.StatusNoContent, gin.H{"msg": fmt.Sprintf("get website data failed, code:%v", data.Code)})
			return
		}

		mimetype, response := generateFeedResponse(config, data, strings.ToLower(rss.Type))
		c.Data(http.StatusOK, mimetype, []byte(response))
	}
}
