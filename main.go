package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/soulteary/RSS-Can/internal/javascript"
)

const DEFAULT_UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36"

func getRemoteDocument(url string) (*goquery.Document, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", DEFAULT_UA)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
	}

	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

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
	doc, err := getRemoteDocument("https://36kr.com/")
	if err != nil {
		log.Fatal(err)
	}

	doc.Find(config.ListContainer).Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Find(config.Title).Text())
		author := strings.TrimSpace(s.Find(config.Author).Text())
		time := strings.TrimSpace(s.Find(config.DateTime).Text())
		category := strings.TrimSpace(s.Find(config.Category).Text())
		description := strings.TrimSpace(s.Find(config.Description).Text())

		href, _ := s.Find(config.Link).Attr("href")
		link := strings.TrimSpace(href)

		fmt.Printf("Aritcle #%d\n", i+1)
		fmt.Printf("%s (%s)\n", title, time)
		fmt.Printf("[%s] , [%s]\n", author, category)
		fmt.Printf("> %s %s\n", description, link)
		fmt.Println()
	})
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
