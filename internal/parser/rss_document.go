package parser

import (
	"encoding/xml"
	"html"
	"strings"
	"time"

	"github.com/soulteary/goquery"
)

// rssAsArticleHTML turns an RSS 2.0 document into an article list the SSR
// selectors can read. HTML parsing drops RSS link text: link is a void element,
// and CDATA becomes a comment, so title and permalink never land on an anchor.
func rssAsArticleHTML(body string) (string, bool) {
	if !looksLikeRSS(body) {
		return "", false
	}
	var doc rssDocument
	if err := xml.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(body), "\uFEFF")), &doc); err != nil || len(doc.Channel.Items) == 0 {
		return "", false
	}
	var page strings.Builder
	page.WriteString("<!DOCTYPE html><html><body>")
	for _, item := range doc.Channel.Items {
		title := strings.TrimSpace(item.Title)
		link := strings.TrimSpace(item.Link)
		if title == "" && link == "" {
			continue
		}
		author := strings.TrimSpace(item.Author)
		if author == "" {
			author = strings.TrimSpace(item.Creator)
		}
		if author == "" {
			author = strings.TrimSpace(item.Source)
		}
		page.WriteString("<article><h2>")
		page.WriteString(html.EscapeString(title))
		page.WriteString("</h2>")
		if link != "" {
			page.WriteString(`<a href="`)
			page.WriteString(html.EscapeString(link))
			page.WriteString(`">`)
			page.WriteString(html.EscapeString(title))
			page.WriteString("</a>")
		}
		if date := rssPubDate(item.PubDate); date != "" {
			page.WriteString("<time>")
			page.WriteString(html.EscapeString(date))
			page.WriteString("</time>")
		}
		if author != "" {
			page.WriteString(`<span class="rss-author">`)
			page.WriteString(html.EscapeString(author))
			page.WriteString("</span>")
		}
		if description := rssDescriptionText(item.Description); description != "" {
			page.WriteString(`<p class="rss-description">`)
			page.WriteString(html.EscapeString(description))
			page.WriteString("</p>")
		}
		page.WriteString("</article>")
	}
	page.WriteString("</body></html>")
	if !strings.Contains(page.String(), "<article>") {
		return "", false
	}
	return page.String(), true
}

func looksLikeRSS(body string) bool {
	head := strings.TrimPrefix(strings.TrimSpace(body), "\uFEFF")
	if len(head) > 512 {
		head = head[:512]
	}
	head = strings.ToLower(head)
	return strings.HasPrefix(head, "<rss") || (strings.HasPrefix(head, "<?xml") && strings.Contains(head, "<rss"))
}

func rssDescriptionText(fragment string) string {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return ""
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader("<div>" + fragment + "</div>"))
	if err != nil {
		return fragment
	}
	return strings.TrimSpace(document.Find("div").First().Text())
}

func rssPubDate(raw string) string {
	raw = strings.Join(strings.Fields(raw), " ")
	if raw == "" {
		return ""
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 -07:00",
		time.RFC3339,
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		"2006-01-02 15:04:05",
	} {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed.Format(time.RFC3339)
		}
	}
	return ""
}

type rssDocument struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
	Source      string `xml:"source"`
	Author      string `xml:"author"`
	Creator     string `xml:"http://purl.org/dc/elements/1.1/ creator"`
}
