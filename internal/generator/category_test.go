package generator_test

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/generator"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/parser"
)

// Read the actual format-specific category fields, rather than accepting a
// category string appearing accidentally elsewhere in serialized output.
func categoriesFromFeed(t *testing.T, format, body string) [][]string {
	t.Helper()
	var categories [][]string
	switch format {
	case define.FEED_TYPE_RSS:
		var feed struct {
			Channel struct {
				Items []struct {
					Categories []string `xml:"category"`
				} `xml:"item"`
			} `xml:"channel"`
		}
		if err := xml.Unmarshal([]byte(body), &feed); err != nil {
			t.Fatal(err)
		}
		for _, item := range feed.Channel.Items {
			categories = append(categories, item.Categories)
		}
	case define.FEED_TYPE_ATOM:
		var feed struct {
			Entries []struct {
				Categories []struct {
					Term string `xml:"term,attr"`
				} `xml:"category"`
			} `xml:"entry"`
		}
		if err := xml.Unmarshal([]byte(body), &feed); err != nil {
			t.Fatal(err)
		}
		for _, item := range feed.Entries {
			var terms []string
			for _, category := range item.Categories {
				terms = append(terms, category.Term)
			}
			categories = append(categories, terms)
		}
	case define.FEED_TYPE_JSON:
		var feed struct {
			Items []struct {
				Tags []string `json:"tags"`
			} `json:"items"`
		}
		if err := json.Unmarshal([]byte(body), &feed); err != nil {
			t.Fatal(err)
		}
		for _, item := range feed.Items {
			categories = append(categories, item.Tags)
		}
	default:
		t.Fatalf("unexpected format %s", format)
	}
	return categories
}

func assertCategoryFormats(t *testing.T, config define.JavaScriptConfig, data define.BodyParsed, want [][]string) {
	t.Helper()
	for _, format := range []string{define.FEED_TYPE_RSS, define.FEED_TYPE_ATOM, define.FEED_TYPE_JSON} {
		t.Run(format, func(t *testing.T) {
			output, err := generator.GenerateFeedsByType(config, data, format)
			if err != nil {
				t.Fatal(err)
			}
			got := categoriesFromFeed(t, format, output)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("categories=%#v want=%#v\n%s", got, want, output)
			}
		})
	}
}

func TestCategoryMappingPreservesUnicodeEscapingAndPerItemScope(t *testing.T) {
	const category = `技术 & <工具> "精选"`
	config := define.JavaScriptConfig{Name: "分类订阅", URL: "https://example.org"}
	data := define.BodyParsed{Body: []define.InfoItem{
		{ID: "first", Title: "First", Link: "https://example.org/first", Category: category},
		{ID: "second", Title: "Second", Link: "https://example.org/second", Category: category},
		{ID: "empty", Title: "Without category", Link: "https://example.org/empty"},
	}}
	assertCategoryFormats(t, config, data, [][]string{{category}, {category}, nil})
}

func TestSSRCategorySelectorAndDetailHookReachEveryFeedFormat(t *testing.T) {
	logger.Initialize()
	cache := cacher.New(cacher.Config{})
	defer cache.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<article><h2>Entry</h2><span class="category">技术 &amp; 工具</span><a href="/detail">Read</a></article>`)
		case "/detail":
			fmt.Fprint(w, `<body><span class="detail-category">研究 &amp; 开发</span></body>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	for _, tc := range []struct {
		name, category string
		hook           bool
	}{
		{"selector", "技术 & 工具", false},
		{"detail-hook", "研究 & 开发", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := define.JavaScriptConfig{Name: "Categories", URL: upstream.URL, Mode: define.PARSE_MODE_SSR, ListContainer: "article", Title: "h2", Link: "a", DisableCache: true}
			if tc.hook {
				config.CategoryHook = define.ConfigHook{Action: define.ConfigHookReadLink, URL: "a", Object: ".detail-category"}
			} else {
				config.Category = ".category"
			}
			parsed := parser.GetDataAndConfigBySSRContext(cacher.WithCache(t.Context(), cache), config)
			if parsed.Code != define.ERROR_CODE_NULL || len(parsed.Body) != 1 || parsed.Body[0].Category != tc.category {
				t.Fatalf("category extraction: %#v", parsed)
			}
			assertCategoryFormats(t, config, parsed, [][]string{{tc.category}})
		})
	}
}
