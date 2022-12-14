package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/generator"
)

func ServAPI(data define.BodyParsed) {

	type RSSType struct {
		Type string `uri:"type" binding:"required"`
	}

	route := gin.Default()
	route.GET("/:type/", func(c *gin.Context) {
		var rssType RSSType
		if err := c.ShouldBindUri(&rssType); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"msg": err})
			return
		}

		var response string
		var mimetype string
		switch strings.ToUpper(rssType.Type) {
		case "RSS":
			mimetype = "application/rss+xml"
			response = generator.GenerateFeedsByType(data, "RSS")
		case "ATOM":
			mimetype = "application/atom+xml"
			response = generator.GenerateFeedsByType(data, "ATOM")
		case "JSON":
			mimetype = "application/feed+json"
			response = generator.GenerateFeedsByType(data, "JSON")
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
	<link rel="alternate" type="application/rss+xml" title="RSS 2.0 Feed" href="http://localhost:8080/rss">
	<link rel="alternate" type="application/atom+xml" title="RSS Atom Feed" href="http://localhost:8080/atom">
	<link rel="alternate" type="application/rss+json" title="RSS JSON Feed" href="http://localhost:8080/json">
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
