package fn

import (
	markdown "github.com/JohannesKaufmann/html-to-markdown"
)

func Html2Md(html string) (string, error) {
	converter := markdown.NewConverter("", true, nil)

	markdown, err := converter.ConvertString(html)
	if err != nil {
		return "", err
	}

	return markdown, err
}
