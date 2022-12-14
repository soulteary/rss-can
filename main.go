package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/feeds"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/javascript"
	"github.com/soulteary/RSS-Can/internal/parser"
)

func generateFeeds(data define.BodyParsed, rssType string) string {
	now := time.Now()

	rssFeed := &feeds.Feed{
		Title:   "36Kr",
		Link:    &feeds.Link{Href: "https://36kr.com/"},
		Created: now,
	}

	for _, data := range data.Body {
		feedItem := feeds.Item{
			Title:       data.Title,
			Author:      &feeds.Author{Name: data.Author},
			Description: data.Description,
			Link:        &feeds.Link{Href: data.Link},
			// 时间处理这块比较麻烦，后续文章再展开
			Created: now,
		}
		rssFeed.Items = append(rssFeed.Items, &feedItem)
	}

	var rss string
	var err error

	switch rssType {
	case "RSS":
		rss, err = rssFeed.ToRss()
	case "ATOM":
		rss, err = rssFeed.ToAtom()
	case "JSON":
		rss, err = rssFeed.ToJSON()
	default:
		rss = ""
	}

	if err != nil {
		fmt.Println(err)
		return ""
	}

	return rss
}

func main() {
	jsApp, _ := os.ReadFile("./config/config.js")
	inject := string(jsApp)

	jsConfig, err := javascript.RunCode(inject, "JSON.stringify(getConfig());")
	if err != nil {
		fmt.Println(err)
		return
	}

	config, err := parser.ParseConfigFromJSON(jsConfig)
	if err != nil {
		fmt.Println(err)
		return
	}
	data := parser.GetWebsiteDataWithConfig(config)

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
			response = generateFeeds(data, "RSS")
		case "ATOM":
			mimetype = "application/atom+xml"
			response = generateFeeds(data, "ATOM")
		case "JSON":
			mimetype = "application/feed+json"
			response = generateFeeds(data, "JSON")
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
