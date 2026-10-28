package parser

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/network"
	"github.com/soulteary/goquery"
)

// Per-feed workers preserve entry order; the process-wide limit also bounds
// upstream pressure when many feeds are generated concurrently.
const detailWorkers = 4

var detailSlots = make(chan struct{}, 16)

func fillDetails(ctx context.Context, config define.JavaScriptConfig, items []define.InfoItem, selections []*goquery.Selection, options []network.RequestOptions) *define.BodyParsed {
	if len(items) == 0 || (config.DateTimeHook.Action != define.ConfigHookReadLink && config.CategoryHook.Action != define.ConfigHookReadLink && config.DescriptionHook.Action != define.ConfigHookReadLink && config.ContentHook.Action != define.ConfigHookReadLink) {
		return nil
	}
	work, cancel := context.WithCancel(ctx)
	defer cancel()
	var first *define.BodyParsed
	var once sync.Once
	fail := func(code define.ErrorCode, status string) {
		once.Do(func() {
			result := define.MixupBodyParsed(code, status, time.Now(), nil)
			first = &result
			cancel()
		})
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range min(detailWorkers, len(items)) {
		wg.Go(func() {
			for i := range jobs {
				if work.Err() != nil {
					return
				}
				item := &items[i]
				hooks := []struct {
					name     string
					hook     define.ConfigHook
					override string
					value    *string
				}{
					{"DateTime", config.DateTimeHook, config.DateTime, &item.Date},
					{"Category", config.CategoryHook, config.Category, &item.Category},
					{"Description", config.DescriptionHook, config.Description, &item.Description},
					{"Content", config.ContentHook, "", &item.Content},
				}
				for _, field := range hooks {
					if field.hook.Action != define.ConfigHookReadLink || field.override != "" {
						continue
					}
					raw := jsBridge(field.hook.URL, "href", selections[i])
					link, err := fn.LinkResolveRelative(raw, config.URL)
					if raw == "" || err != nil {
						fail(define.ERROR_CODE_PARSE_CONTENT_FAILED, fmt.Sprintf("entry %d %s detail link is missing or invalid", i+1, field.name))
						return
					}
					select {
					case detailSlots <- struct{}{}:
					case <-work.Done():
						return
					}
					result := network.GetRemoteMarkdownContext(work, link, field.hook.Object, config.Charset, config.Expire, config.DisableCache, options...)
					<-detailSlots
					if result.Code != define.ERROR_CODE_NULL {
						fail(result.Code, fmt.Sprintf("entry %d %s detail failed: %s", i+1, field.name, result.Status))
						return
					}
					if result.Body != "" || field.name != "Content" {
						*field.value = result.Body
					}
				}
			}
		})
	}
send:
	for i := range items {
		select {
		case jobs <- i:
		case <-work.Done():
			break send
		}
	}
	close(jobs)
	wg.Wait()
	if first != nil {
		return first
	}
	if err := ctx.Err(); err != nil {
		result := define.MixupBodyParsed(define.ERROR_CODE_NETWORK, err.Error(), time.Now(), nil)
		return &result
	}
	return nil
}
