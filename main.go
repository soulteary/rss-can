package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
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

func getFeeds() {
	// Request the HTML page.
	doc, err := getRemoteDocument("https://36kr.com/")
	if err != nil {
		log.Fatal(err)
	}

	// Find the article items
	doc.Find("#app .main-right .kr-home-main .kr-home-flow .kr-home-flow-list .kr-flow-article-item").Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Find(".article-item-title").Text())
		time := strings.TrimSpace(s.Find(".kr-flow-bar-time").Text())
		fmt.Printf("Aritcle %d: %s (%s)\n", i+1, title, time)
	})
}

func main() {
	getFeeds()
}
