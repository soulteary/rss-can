package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/feeds"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/javascript"
	"github.com/soulteary/RSS-Can/internal/network"
	"github.com/soulteary/RSS-Can/internal/parser"
)

func getWebsiteDataWithConfig(config define.JavaScriptConfig) (result define.BodyParsed) {
	doc := network.GetRemoteDocument("https://36kr.com/", "utf-8")
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

func generateFeeds(data define.BodyParsed) {
	now := time.Now()

	rssFeed := &feeds.Feed{
		Title:   "36Kr",
		Link:    &feeds.Link{Href: "https://36kr.com/"},
		Created: now,
	}

	for _, data := range data.Body {
		feedItem := feeds.Item{
			Title:       data.Title,
			Author:      &feeds.Author{Name: data.Author},
			Description: data.Description,
			Link:        &feeds.Link{Href: data.Link},
			// 时间处理这块比较麻烦，后续文章再展开
			Created: now,
		}
		rssFeed.Items = append(rssFeed.Items, &feedItem)
	}

	atom, err := rssFeed.ToAtom()
	if err != nil {
		log.Fatal(err)
	}

	rss, err := rssFeed.ToRss()
	if err != nil {
		log.Fatal(err)
	}

	json, err := rssFeed.ToJSON()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(atom, "\n", rss, "\n", json)
}

func main() {
	jsApp, _ := os.ReadFile("./config/config.js")
	inject := string(jsApp)

	jsConfig, err := javascript.RunCode(inject, "JSON.stringify(getConfig());")
	if err != nil {
		fmt.Println(err)
		return
	}

	config, err := parser.ParseConfigFromJSON(jsConfig)
	if err != nil {
		fmt.Println(err)
		return
	}
	data := getWebsiteDataWithConfig(config)
	generateFeeds(data)
}
