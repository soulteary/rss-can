package parser

import (
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
)

func TestRSSFeedParsesAsArticleList(t *testing.T) {
	raw := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>36氪</title>
<item>
<title>标题</title>
<link><![CDATA[https://36kr.com/p/4070571639067782?f=rss]]></link>
<pubDate>2026-11-15 13:19:51  +0800</pubDate>
<description><![CDATA[<p>摘要 <b>重点</b></p>]]></description>
<source url="http://36kr.com">36氪</source>
</item>
</channel></rss>`
	got := ParseDataAndConfigBySSR(define.JavaScriptConfig{
		URL:           "https://www.36kr.com/feed",
		ListContainer: "article",
		Title:         "h2",
		Link:          "a",
		Author:        ".rss-author",
		DateTime:      "time",
		Description:   ".rss-description",
		IdByRegexp:    `/p/(\d+)`,
	}, define.RemoteBodySanitized{Body: raw}, "")
	if got.Code != define.ERROR_CODE_NULL || len(got.Body) != 1 {
		t.Fatalf("rss items: %#v", got)
	}
	item := got.Body[0]
	if item.Title != "标题" || item.Link != "https://36kr.com/p/4070571639067782?f=rss" || item.ID != "4070571639067782" {
		t.Fatalf("identity: %#v", item)
	}
	if item.Author != "36氪" || item.Date != "2026-11-15T13:19:51+08:00" || !strings.Contains(item.Description, "摘要") || !strings.Contains(item.Description, "重点") {
		t.Fatalf("fields: %#v", item)
	}
}

func TestHTMLPageIsNotRewrittenAsRSS(t *testing.T) {
	got := ParseDataAndConfigBySSR(define.JavaScriptConfig{URL: "https://example.org", ListContainer: "article", Title: "h2", Link: "a"}, define.RemoteBodySanitized{}, `<article><h2>keep</h2><a href="/post">read</a></article>`)
	if got.Code != define.ERROR_CODE_NULL || len(got.Body) != 1 || got.Body[0].Title != "keep" || got.Body[0].Link != "https://example.org/post" {
		t.Fatalf("html page: %#v", got)
	}
}
