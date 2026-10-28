package generator

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"github.com/soulteary/RSS-Can/internal/limits"
	"github.com/soulteary/feeds"
)

func GenerateFeedsByType(config define.JavaScriptConfig, data define.BodyParsed, rssType string) (string, error) {
	return GenerateFeedsByTypeContext(context.Background(), config, data, rssType)
}

func GenerateFeedsByTypeContext(ctx context.Context, config define.JavaScriptConfig, data define.BodyParsed, rssType string) (string, error) {
	if rssType != define.FEED_TYPE_RSS && rssType != define.FEED_TYPE_ATOM && rssType != define.FEED_TYPE_JSON {
		return "", fmt.Errorf("unsupported feed type: %s", rssType)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !limits.ItemsWithinBudget(data.Body) {
		return "", fmt.Errorf("feed exceeds item or content size limit")
	}
	now := time.Now()

	rssFeed := &feeds.Feed{
		Title:   config.File,
		Created: now,
	}

	if config.URL != "" {
		rssFeed.Link = &feeds.Link{Href: config.URL}
	}

	if config.Name != "" {
		rssFeed.Title = config.Name
	}

	// Relative dates are evaluated once per feed, never cached across requests.
	dates := make(map[string]time.Time)
	for _, data := range data.Body {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		feedItem := feeds.Item{
			Title:       data.Title,
			Author:      &feeds.Author{Name: data.Author},
			Description: data.Description,
			Link:        &feeds.Link{Href: data.Link},
		}

		if data.Date != "" {
			timeUnix, ok := dates[data.Date]
			if !ok {
				var err error
				timeUnix, err = jssdk.ConvertStrToUnixContext(ctx, data.Date)
				if err != nil {
					return "", fmt.Errorf("convert entry date: %w", err)
				}
				dates[data.Date] = timeUnix
			}
			feedItem.Created = timeUnix
		}

		if data.ID != "" {
			feedItem.Id = data.ID
		}

		if data.Content != "" {
			feedItem.Content = data.Content
		}

		rssFeed.Items = append(rssFeed.Items, &feedItem)
	}

	var buffer bytes.Buffer
	writer := contextWriter{ctx: ctx, buffer: &buffer}
	var err error

	switch rssType {
	case define.FEED_TYPE_RSS:
		err = rssFeed.WriteRss(writer)
	case define.FEED_TYPE_ATOM:
		err = rssFeed.WriteAtom(writer)
	case define.FEED_TYPE_JSON:
		err = rssFeed.WriteJSON(writer)
	}

	if err != nil {
		return "", fmt.Errorf("generate feed: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

type contextWriter struct {
	ctx    context.Context
	buffer *bytes.Buffer
}

func (w contextWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if len(p) > limits.MaxFeedBytes-w.buffer.Len() {
		return 0, fmt.Errorf("feed exceeds %d bytes", limits.MaxFeedBytes)
	}
	return w.buffer.Write(p)
}
