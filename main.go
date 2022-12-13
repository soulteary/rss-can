package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/javascript"
	"github.com/soulteary/RSS-Can/internal/network"
)

type Config struct {
	ListContainer string `json:"ListContainer"`
	Title         string `json:"Title"`
	Author        string `json:"Author"`
	Category      string `json:"Category"`
	DateTime      string `json:"DateTime"`
	Description   string `json:"Description"`
	Link          string `json:"Link"`
}

func getFeeds(config Config) {
	doc := network.GetRemoteDocument("https://36kr.com/", "utf-8", func(document *goquery.Document) []define.Item {
		var items []define.Item
		document.Find(config.ListContainer).Each(func(i int, s *goquery.Selection) {
			var item define.Item

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
	fmt.Println(doc)
}

func main() {
	jsApp, _ := os.ReadFile("./config/config.js")
	inject := string(jsApp)

	result, err := javascript.RunCode(inject, "JSON.stringify(getConfig());")
	if err != nil {
		fmt.Println(err)
		return
	}

	var config Config
	err = json.Unmarshal([]byte(result), &config)
	if err != nil {
		fmt.Println(err)
		return
	}
	getFeeds(config)
}
