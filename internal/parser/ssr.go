package parser

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/limits"
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
	expression := define.ParseFieldExpression(field)
	action := strings.TrimSpace(strings.ToLower(method))
	if !expression.Explicit && expression.Value == "" {
		value, _ := s.Attr(action)
		return strings.TrimSpace(value)
	}
	if !expression.Selector {
		return expression.Value
	}
	if expression.Value == "" {
		return ""
	}
	switch action {
	case "text":
		return strings.TrimSpace(s.Find(expression.Value).Text())
	case "html":
		value, _ := s.Find(expression.Value).Html()
		return value
	case "href":
		value, _ := s.Find(expression.Value).First().Attr(action)
		return strings.TrimSpace(value)
	}
	return ""
}

func GetDataAndConfigBySSR(config define.JavaScriptConfig) (result define.BodyParsed) {
	return GetDataAndConfigBySSRContext(context.Background(), config)
}

func GetDataAndConfigBySSRContext(ctx context.Context, config define.JavaScriptConfig, options ...network.RequestOptions) (result define.BodyParsed) {
	if err := ctx.Err(); err != nil {
		return define.MixupBodyParsed(define.ERROR_CODE_NETWORK, err.Error(), time.Now(), nil)
	}
	if err := ValidateSSRConfig(config); err != nil {
		return define.MixupBodyParsed(define.ERROR_CODE_PARSE_CONTENT_FAILED, err.Error(), time.Now(), nil)
	}
	if len(options) == 0 {
		options = []network.RequestOptions{network.OptionsForConfigContext(ctx, config)}
	}
	doc := network.GetRemoteDocumentContext(ctx, config.URL, config.Charset, config.Expire, config.DisableCache, options...)
	if doc.Code != define.ERROR_CODE_NULL {
		return define.MixupBodyParsed(doc.Code, doc.Status, doc.Date, nil)
	}
	if doc.Body == "" {
		return define.MixupBodyParsed(define.ERROR_CODE_PARSE_CONTENT_FAILED, "empty upstream response", doc.Date, nil)
	}
	return ParseDataAndConfigBySSRContext(ctx, config, doc, "", options...)
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

func ParseDataAndConfigBySSRContext(ctx context.Context, config define.JavaScriptConfig, userDoc define.RemoteBodySanitized, userHtml string, options ...network.RequestOptions) (result define.BodyParsed) {
	if len(options) == 0 {
		options = []network.RequestOptions{network.OptionsForConfigContext(ctx, config)}
	}
	failure := func(err error) define.BodyParsed {
		return define.MixupBodyParsed(define.ERROR_CODE_PARSE_CONTENT_FAILED, err.Error(), time.Now(), nil)
	}
	if err := ctx.Err(); err != nil {
		return failure(err)
	}
	if err := validateSSRFields(config); err != nil {
		return failure(err)
	}
	idRegexp, err := config.CompileIDRegexp()
	if err != nil {
		return failure(err)
	}
	if config.PagerLimit > limits.MaxPages {
		return failure(fmt.Errorf("PagerLimit exceeds %d pages", limits.MaxPages))
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

	if len(doc.Body) > limits.MaxPageBytes {
		return failure(fmt.Errorf("page exceeds %d bytes", limits.MaxPageBytes))
	}
	var items []define.InfoItem
	retained := 0

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
		first := ParseDataAndConfigBySSRContext(ctx, firstConfig, doc, "", options...)
		if first.Code != define.ERROR_CODE_NULL {
			return first
		}
		for _, item := range first.Body {
			retained += itemBytes(item)
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
			ret := GetDataAndConfigBySSRContext(ctx, newConfig, options...)
			if ret.Code != define.ERROR_CODE_NULL {
				return ret
			}
			if len(ret.Body) > limits.MaxItems-len(items) {
				return failure(fmt.Errorf("feed exceeds %d items", limits.MaxItems))
			}
			for _, item := range ret.Body {
				retained += itemBytes(item)
			}
			if retained > limits.MaxFeedBytes {
				return failure(fmt.Errorf("feed content exceeds size limit"))
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

	var selections []*goquery.Selection
	itemLimitExceeded := false
	contentLimitExceeded := false
	result = ParsePageByGoQuery(doc, func(document *goquery.Document) (items []define.InfoItem) {
		document.Find(config.ListContainer).EachWithBreak(func(i int, s *goquery.Selection) bool {
			if ctx.Err() != nil {
				return false
			}
			if len(items) >= limits.MaxItems {
				itemLimitExceeded = true
				return false
			}
			var item define.InfoItem
			// TODO bind all hook action
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

			if item.Content == "" {
				if config.Content != "" {
					content := jsBridge(config.Content, "text", s)
					item.Content = fn.Html2Md(content)
				}
			}

			retained += itemBytes(item)
			if retained > limits.MaxFeedBytes {
				contentLimitExceeded = true
				return false
			}
			selections = append(selections, s)
			items = append(items, item)
			return ctx.Err() == nil
		})
		return items
	})
	if err := ctx.Err(); err != nil {
		return failure(err)
	}
	if contentLimitExceeded {
		return failure(fmt.Errorf("feed content exceeds size limit"))
	}
	if itemLimitExceeded {
		return failure(fmt.Errorf("feed exceeds %d items", limits.MaxItems))
	}
	if result.Code == define.ERROR_CODE_NULL {
		if detailFailure := fillDetails(ctx, config, result.Body, selections, options); detailFailure != nil {
			return *detailFailure
		}
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

func itemBytes(item define.InfoItem) int {
	return len(item.ID) + len(item.Title) + len(item.Link) + len(item.Date) + len(item.Author) + len(item.Category) + len(item.Description) + len(item.Content)
}
