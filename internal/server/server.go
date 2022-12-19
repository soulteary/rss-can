package server

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/rule"
)

func ServAPI() {
	rule.InitRules()

	route := gin.Default()

	route.Use(Logger(logger.Instance), gin.Recovery())
	route.GET("/:id/:type/", apiRSS())
	route.GET("/config/:type/:value/", apiConfig())
	route.GET("/", welcomePage())

	route.Run(fmt.Sprintf(":%d", define.DEFAULT_HTT_PORT))
}
