package ssr

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/network"
)

func ParsePageByGoQuery(data define.RemoteBodySanitized, callback func(document *goquery.Document) []define.InfoItem) define.BodyParsed {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(data.Body))

	if err != nil {
		code := define.ERROR_CODE_PARSE_CONTENT_FAILED
		status := fmt.Sprintf("%s: %s", define.ERROR_STATUS_PARSE_CONTENT_FAILED, fmt.Errorf("%w", err))
		return define.MixupBodyParsed(code, status, data.Date, nil)
	}

	code := define.ERROR_CODE_NULL
	status := define.ERROR_STATUS_NULL
	items := callback(document)
	return define.MixupBodyParsed(code, status, data.Date, items)
}

func GetWebsiteDataWithConfig(config define.JavaScriptConfig) (result define.BodyParsed) {
	// TODO allows for automatic charset recognition
	// TODO allow set charset by JS Config
	doc := network.GetRemoteDocument(config.URL, "utf-8")
	if doc.Body == "" {
		return result
	}

	return ParsePageByGoQuery(doc, func(document *goquery.Document) []define.InfoItem {
		var items []define.InfoItem
		document.Find(config.ListContainer).Each(func(i int, s *goquery.Selection) {
			var item define.InfoItem

			title := strings.TrimSpace(s.Find(config.Title).Text())
			author := strings.TrimSpace(s.Find(config.Author).Text())
			time := strings.TrimSpace(s.Find(config.DateTime).Text())
			category := strings.TrimSpace(s.Find(config.Category).Text())
			description := strings.TrimSpace(s.Find(config.Description).Text())

			href, _ := s.Find(config.Link).Attr("href")
			link := strings.TrimSpace(href)

			item.Title = title
			item.Author = author
			item.Date = time
			item.Category = category
			item.Description = description
			item.Link = link
			items = append(items, item)
		})
		return items
	})
}
