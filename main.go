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

func getFeeds(config define.JavaScriptConfig) {
	doc := network.GetRemoteDocument("https://36kr.com/", "utf-8")
	if doc.Body == "" {
		return
	}

	items := parser.ParsePageByGoQuery(doc, func(document *goquery.Document) []define.InfoItem {
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
	fmt.Println(items)
}

func generateFeedsTest() {
	now := time.Now()
	feed := &feeds.Feed{
		Title:       "苏洋博客",
		Link:        &feeds.Link{Href: "https://soulteary.com/"},
		Description: "醉里不知天在水，满船清梦压星河。",
		Author:      &feeds.Author{Name: "soulteary", Email: "soulteary@gmail.com"},
		Created:     now,
	}

	feed.Items = []*feeds.Item{
		{
			Title:       "RSS Can：借助 V8 让 Golang 应用具备动态化能力（二）",
			Link:        &feeds.Link{Href: "https://soulteary.com/2022/12/13/rsscan-make-golang-applications-with-v8-part-2.html"},
			Description: "继续聊聊之前做过的一个小东西的踩坑历程，如果你也想高效获取信息，或许这个系列的内容会对你有用。",
			Author:      &feeds.Author{Name: "soulteary", Email: "soulteary@qq.com"},
			Created:     now,
		},
		{
			Title:       "RSS Can：使用 Golang 实现更好的 RSS Hub 服务（一）",
			Link:        &feeds.Link{Href: "https://soulteary.com/2022/12/12/rsscan-better-rsshub-service-build-with-golang-part-1.html"},
			Description: "聊聊之前做过的一个小东西的踩坑历程，如果你也想高效获取信息，或许这个系列的内容会对你有用。这个事情涉及的东西比较多，所以我考虑拆成一个系列来聊，每篇的内容不要太长，整理负担和阅读负担都轻一些。本篇是系列第一篇内容。",
			Author:      &feeds.Author{Name: "soulteary", Email: "soulteary@gmail.com"},
			Created:     now,
		},
		{
			Title:       "在搭载 M1 及 M2 芯片 MacBook设备上玩 Stable Diffusion 模型",
			Link:        &feeds.Link{Href: "https://soulteary.com/2022/12/10/play-the-stable-diffusion-model-on-macbook-devices-with-m1-and-m2-chips.html"},
			Description: "本篇文章，我们聊了如何使用搭载了 Apple Silicon 芯片（M1 和 M2 CPU）的 MacBook 设备上运行 Stable Diffusion 模型。",
			Created:     now,
		},
		{
			Title:       "使用 Docker 来快速上手中文 Stable Diffusion 模型：太乙",
			Link:        &feeds.Link{Href: "https://soulteary.com/2022/12/09/use-docker-to-quickly-get-started-with-the-chinese-stable-diffusion-model-taiyi.html"},
			Description: "本篇文章，我们聊聊如何使用 Docker 快速运行中文 Stable Diffusion 模型：太乙。 ",
			Created:     now,
		},
	}

	atom, err := feed.ToAtom()
	if err != nil {
		log.Fatal(err)
	}

	rss, err := feed.ToRss()
	if err != nil {
		log.Fatal(err)
	}

	json, err := feed.ToJSON()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(atom, "\n", rss, "\n", json)
}

func main() {
	jsApp, _ := os.ReadFile("./config/config.js")
	inject := string(jsApp)

	result, err := javascript.RunCode(inject, "JSON.stringify(getConfig());")
	if err != nil {
		fmt.Println(err)
		return
	}

	config, err := parser.ParseConfigFromJSON(result)
	if err != nil {
		fmt.Println(err)
		return
	}
	getFeeds(config)
	generateFeedsTest()
}
