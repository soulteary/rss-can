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

func jsBridge(field string, method string, s *goquery.Selection) string {
	if strings.Contains(field, ".") || strings.Contains(field, "#") {
		// extract information by attributes
		find := strings.ToLower(method)
		if find == "text" {
			return strings.TrimSpace(s.Find(field).Text())
		} else if find == "href" || strings.HasPrefix(find, "data-") {
			prop, exists := s.Find(field).Attr(method)
			if !exists {
				return ""
			}
			return strings.TrimSpace(prop)
		}
	}

	// if not a selector, fallback the original content
	return field
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
			// title must exist in the config
			if config.Title != "" {
				title := jsBridge(config.Title, "text", s)
				item.Title = title

				if config.Author != "" {
					author := jsBridge(config.Author, "text", s)
					item.Author = author
				}

				if config.DateTime != "" {
					time := jsBridge(config.DateTime, "text", s)
					item.Date = time
				}

				if config.Category != "" {
					category := jsBridge(config.Category, "text", s)
					item.Category = category
				}

				if config.Description != "" {
					description := jsBridge(config.Description, "text", s)
					item.Description = description
				}

				if config.Link != "" {
					link := jsBridge(config.Link, "href", s)
					item.Link = link
				}

				items = append(items, item)
			}
		})
		return items
	})
}
