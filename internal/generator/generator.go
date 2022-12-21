package generator

import (
	"time"

	"github.com/gorilla/feeds"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func GenerateFeedsByType(config define.JavaScriptConfig, data define.BodyParsed, rssType string) string {
	now := time.Now()

	// TODO update with rules
	rssFeed := &feeds.Feed{
		Title:   config.File,
		Link:    &feeds.Link{Href: config.URL},
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

		if data.ID != "" {
			feedItem.Id = data.ID
		}

		if data.Content != "" {
			feedItem.Content = data.Content
		}

		rssFeed.Items = append(rssFeed.Items, &feedItem)
	}

	var rss string
	var err error

	switch rssType {
	case define.FEED_TYPE_RSS:
		rss, err = rssFeed.ToRss()
	case define.FEED_TYPE_ATOM:
		rss, err = rssFeed.ToAtom()
	case define.FEED_TYPE_JSON:
		rss, err = rssFeed.ToJSON()
	default:
		rss = ""
	}

	if err != nil {
		logger.Instance.Errorf("Generate feed failed: %v", err)
		return ""
	}

	return rss
}
