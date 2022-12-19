package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/soulteary/RSS-Can/internal/generator"
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

	const hello = `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta http-equiv="X-UA-Compatible" content="IE=edge">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>RSS Feed Discovery.</title>
	<link rel="alternate" type="application/rss+xml" title="RSS 2.0 Feed" href="http://localhost:8080/36kr/rss">
	<link rel="alternate" type="application/atom+xml" title="RSS Atom Feed" href="http://localhost:8080/36kr/atom">
	<link rel="alternate" type="application/rss+json" title="RSS JSON Feed" href="http://localhost:8080/36kr/json">
</head>
<body>
	RSS Feed Discovery.
</body>
</html>`

	route.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html", []byte(hello))
	})

	route.Run(":8080")
}
