package fn

import (
	markdown "github.com/soulteary/html-to-markdown/v2"
)

func Html2Md(html string) string {
	markdown, _ := markdown.ConvertString(html)
	return markdown
}
