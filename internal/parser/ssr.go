package parser

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/network"
	"github.com/soulteary/goquery"
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

func ParsePagerByGoQuery(data define.RemoteBodySanitized, callback func(document *goquery.Document) []string) (result []string) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(data.Body))
	if err != nil {
		return result
	}
	result = callback(document)
	return result
}

func jsBridge(field string, method string, s *goquery.Selection) string {
	if field == "" {
		value, exists := s.Attr(strings.TrimSpace(strings.ToLower(method)))
		if exists {
			return strings.TrimSpace(value)
		}
	}

	if fn.IsCssSelector(field) || fn.IsDomTagName(field) {
		action := strings.TrimSpace(strings.ToLower(method))
		switch action {
		case "text":
			return strings.TrimSpace(s.Find(field).Text())
		case "html":
			html, err := s.Find(field).Html()
			if err != nil {
				return ""
			}
			return html
		case "href":
			prop, exists := s.Find(field).Attr(action)
			if !exists {
				return ""
			}
			return strings.TrimSpace(prop)
		}
	}

	// if not a selector, fallback the original content
	return field
}

func GetDataAndConfigBySSR(config define.JavaScriptConfig) (result define.BodyParsed) {
	return GetDataAndConfigBySSRContext(context.Background(), config)
}

func GetDataAndConfigBySSRContext(ctx context.Context, config define.JavaScriptConfig) (result define.BodyParsed) {
	doc := network.GetRemoteDocumentContext(ctx, config.URL, config.Charset, config.Expire, config.DisableCache)
	if doc.Code != define.ERROR_CODE_NULL {
		return define.MixupBodyParsed(doc.Code, doc.Status, doc.Date, nil)
	}
	if doc.Body == "" {
		return define.MixupBodyParsed(define.ERROR_CODE_PARSE_CONTENT_FAILED, "empty upstream response", doc.Date, nil)
	}
	return ParseDataAndConfigBySSRContext(ctx, config, doc, "")
}

func GetPager(config define.JavaScriptConfig, document *goquery.Document) (links []string) {
	if config.Pager != "" {
		document.Find(config.Pager).Each(func(i int, s *goquery.Selection) {
			rawLink, exist := s.Attr("href")
			if exist {
				link, err := fn.LinkResolveRelative(rawLink, config.URL)
				if err == nil {
					links = append(links, link)
				}
			}
		})
		return links
	}
	return links
}

func ParseDataAndConfigBySSR(config define.JavaScriptConfig, userDoc define.RemoteBodySanitized, userHtml string) (result define.BodyParsed) {
	return ParseDataAndConfigBySSRContext(context.Background(), config, userDoc, userHtml)
}

func ParseDataAndConfigBySSRContext(ctx context.Context, config define.JavaScriptConfig, userDoc define.RemoteBodySanitized, userHtml string) (result define.BodyParsed) {
	failure := func(err error) define.BodyParsed {
		return define.MixupBodyParsed(define.ERROR_CODE_PARSE_CONTENT_FAILED, err.Error(), time.Now(), nil)
	}
	if err := ctx.Err(); err != nil {
		return failure(err)
	}
	idRegexp, err := config.CompileIDRegexp()
	if err != nil {
		return failure(err)
	}
	if config.PagerLimit < 0 {
		return failure(fmt.Errorf("PagerLimit must not be negative"))
	}
	var doc define.RemoteBodySanitized
	if userHtml != "" {
		doc.Code = define.ERROR_CODE_NULL
		doc.Status = define.ERROR_STATUS_NULL
		doc.Body = userHtml
		doc.Date = time.Now()
	} else {
		if userDoc.Code == define.ERROR_CODE_NULL {
			doc = userDoc
		}
	}

	var items []define.InfoItem

	if config.Pager != "" {
		pageLinks := ParsePagerByGoQuery(doc, func(document *goquery.Document) []string {
			return GetPager(config, document)
		})

		// PagerLimit is the total page budget, including the current page.
		// An omitted/zero limit parses only the current page.
		limit := config.PagerLimit
		if limit == 0 {
			limit = 1
		}
		firstConfig := config
		firstConfig.Pager = ""
		first := ParseDataAndConfigBySSRContext(ctx, firstConfig, doc, "")
		if first.Code != define.ERROR_CODE_NULL {
			return first
		}
		items = append(items, first.Body...)
		canonical := func(link string) string {
			u, err := url.Parse(link)
			if err != nil {
				return link
			}
			u.Fragment = ""
			if u.Path == "" {
				u.Path = "/"
			}
			return u.String()
		}
		seen := map[string]bool{canonical(config.URL): true}
		pages := 1

		for _, link := range pageLinks {
			if pages >= limit {
				break
			}
			key := canonical(link)
			if seen[key] {
				continue
			}
			seen[key] = true
			pages++
			if err := ctx.Err(); err != nil {
				return failure(err)
			}
			newConfig := config
			newConfig.URL = link
			newConfig.Pager = ""
			ret := GetDataAndConfigBySSRContext(ctx, newConfig)
			if ret.Code != define.ERROR_CODE_NULL {
				return ret
			}
			items = append(items, ret.Body...)
		}
		if err := ctx.Err(); err != nil {
			return failure(err)
		}
		code := define.ERROR_CODE_NULL
		status := define.ERROR_STATUS_NULL
		return define.MixupBodyParsed(code, status, time.Now(), items)
	}

	result = ParsePageByGoQuery(doc, func(document *goquery.Document) (items []define.InfoItem) {
		document.Find(config.ListContainer).EachWithBreak(func(i int, s *goquery.Selection) bool {
			if ctx.Err() != nil {
				return false
			}
			var item define.InfoItem
			// title must exist in the config
			if config.Title == "" {
				return true
			}

			// TODO bind all hook action
			title := jsBridge(config.Title, "text", s)
			item.Title = title

			if config.Author != "" {
				author := jsBridge(config.Author, "text", s)
				item.Author = author
			}

			if config.DateTimeHook.Action == define.ConfigHookReadLink {
				rawLink := jsBridge(config.DateTimeHook.URL, "href", s)
				link, err := fn.LinkResolveRelative(rawLink, config.URL)
				if err == nil {
					time := network.GetRemoteDocumentAsMarkdownContext(ctx, link, config.DateTimeHook.Object, config.Charset, config.Expire, config.DisableCache)
					item.Date = time
				}
			}
			if config.DateTime != "" {
				time := jsBridge(config.DateTime, "text", s)
				item.Date = time
			}

			if config.CategoryHook.Action == define.ConfigHookReadLink {
				rawLink := jsBridge(config.CategoryHook.URL, "href", s)
				link, err := fn.LinkResolveRelative(rawLink, config.URL)
				if err == nil {
					category := network.GetRemoteDocumentAsMarkdownContext(ctx, link, config.CategoryHook.Object, config.Charset, config.Expire, config.DisableCache)
					item.Category = category
				}
			}
			if config.Category != "" {
				category := jsBridge(config.Category, "text", s)
				item.Category = category
			}

			if config.DescriptionHook.Action == define.ConfigHookReadLink {
				rawLink := jsBridge(config.DescriptionHook.URL, "href", s)
				link, err := fn.LinkResolveRelative(rawLink, config.URL)
				if err == nil {
					description := network.GetRemoteDocumentAsMarkdownContext(ctx, link, config.DescriptionHook.Object, config.Charset, config.Expire, config.DisableCache)
					item.Description = description
				}
			}
			if config.Description != "" {
				description := jsBridge(config.Description, "text", s)
				item.Description = description
			}

			if config.Link != "" {
				rawLink := jsBridge(config.Link, "href", s)
				link, err := fn.LinkResolveRelative(rawLink, config.URL)
				if err == nil {
					item.Link = link
				}

				if idRegexp != nil {
					match := idRegexp.FindStringSubmatch(rawLink)
					if len(match) > 1 {
						item.ID = match[1]
					}
				}
			}

			if config.IdByProp.Object != "" || config.IdByProp.Prop != "" {
				prop := jsBridge(config.IdByProp.Object, config.IdByProp.Prop, s)
				item.ID = prop
			}

			if config.ContentHook.Action == define.ConfigHookReadLink {
				rawLink := jsBridge(config.ContentHook.URL, "href", s)
				link, err := fn.LinkResolveRelative(rawLink, config.URL)
				if err == nil {
					content := network.GetRemoteDocumentAsMarkdownContext(ctx, link, config.ContentHook.Object, config.Charset, config.Expire, config.DisableCache)
					item.Content = content
				}
			}

			if item.Content == "" {
				if config.Content != "" {
					content := jsBridge(config.Content, "text", s)
					item.Content = fn.Html2Md(content)
				}
			}

			items = append(items, item)
			return ctx.Err() == nil
		})
		return items
	})
	if err := ctx.Err(); err != nil {
		return failure(err)
	}
	return result
}

func ParseFullPageByGoQuery(html string, callback func(document *goquery.Document) string) (result string) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return result
	}
	result = callback(document)
	return result
}
