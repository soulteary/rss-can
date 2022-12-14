package ssr

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/network"
	"github.com/soulteary/RSS-Can/internal/parser"
)

func GetWebsiteDataWithConfig(config define.JavaScriptConfig) (result define.BodyParsed) {
	doc := network.GetRemoteDocument(config.URL, "utf-8")
	if doc.Body == "" {
		return result
	}

	return parser.ParsePageByGoQuery(doc, func(document *goquery.Document) []define.InfoItem {
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
