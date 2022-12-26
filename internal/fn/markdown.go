package fn

import (
	markdown "github.com/JohannesKaufmann/html-to-markdown"
)

func Html2Md(html string) string {
	converter := markdown.NewConverter("", true, nil)
	markdown, _ := converter.ConvertString(html)
	return markdown
}
