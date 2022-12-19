package fn

import (
	"log"

	markdown "github.com/JohannesKaufmann/html-to-markdown"
)

func Html2Md(html string) string {
	converter := markdown.NewConverter("", true, nil)
	markdown, err := converter.ConvertString(html)
	if err != nil {
		log.Fatal(err)
	}
	return markdown
}
