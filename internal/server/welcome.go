package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const welcomePageForTest = `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta http-equiv="X-UA-Compatible" content="IE=edge">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>RSS Feed Discovery.</title>
	<link rel="alternate" type="application/rss+xml" title="RSS 2.0 Feed" href="http://localhost:8080/feed/36kr/rss">
	<link rel="alternate" type="application/atom+xml" title="RSS Atom Feed" href="http://localhost:8080/feed/36kr/atom">
	<link rel="alternate" type="application/rss+json" title="RSS JSON Feed" href="http://localhost:8080/feed/36kr/json">
</head>
<body>
	RSS Feed Discovery.
</body>
</html>`

func welcomePage() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html", []byte(welcomePageForTest))
	}
}
