package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/rule"
)

type DynamicConfig struct {
	Type  string `uri:"type" binding:"required"`
	Value string `uri:"value" binding:"required"`
}

func apiConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		var config DynamicConfig
		if err := c.ShouldBindUri(&config); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"msg": err})
			return
		}

		switch strings.ToLower(config.Type) {
		case "set-loglevel":
			logger.SetLevel(config.Value)
			c.JSON(http.StatusOK, gin.H{"msg": fmt.Sprintf("Update loglevel with %s", config.Value)})
			return
		case "rules":
			if strings.ToLower(config.Value) == "fresh" {
				rule.InitRules()
				c.JSON(http.StatusOK, gin.H{"msg": "Rules refreshed"})
			}
			return
		}

	}
}
