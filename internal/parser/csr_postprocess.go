package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/limits"
	"github.com/soulteary/RSS-Can/internal/network"
)

// csrWireItem keeps browser-only hook extraction out of the public InfoItem
// contract. Custom GetData implementations may continue returning plain items.
type csrWireItem struct {
	define.InfoItem
	Native bool          `json:"__rsscan_native"`
	Hooks  *csrHookLinks `json:"__rsscan_hooks,omitempty"`
}

// Fixed fields avoid allocating a map for arbitrary browser-owned JSON keys.
type csrHookLinks struct {
	DateTime    string
	Category    string
	Description string
	Content     string
}

func (h *csrHookLinks) link(field string) string {
	if h == nil {
		return ""
	}
	switch field {
	case "DateTime":
		return h.DateTime
	case "Category":
		return h.Category
	case "Description":
		return h.Description
	case "Content":
		return h.Content
	}
	return ""
}

func decodeCSRWire(ctx context.Context, data string) ([]csrWireItem, error) {
	if len(data) > limits.MaxPageBytes {
		return nil, fmt.Errorf("browser result exceeds size limit")
	}
	decoder := json.NewDecoder(strings.NewReader(data))
	start, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if start == nil {
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			return nil, fmt.Errorf("invalid trailing browser JSON")
		}
		return nil, nil
	}
	if start != json.Delim('[') {
		return nil, fmt.Errorf("browser result must be an array")
	}
	var items []csrWireItem
	for decoder.More() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(items) >= limits.MaxItems {
			return nil, fmt.Errorf("browser result exceeds item limit")
		}
		var item csrWireItem
		if err := decoder.Decode(&item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, fmt.Errorf("invalid trailing browser JSON")
	}
	return items, nil
}

// processCSRItemsContext applies transport-independent item processing after
// shared raw extraction. Every waiter owns its own decoded items and detail
// work, so a canceled caller cannot cancel another caller's enrichment.
func processCSRItemsContext(ctx context.Context, config define.JavaScriptConfig, data string, documentBase ...string) define.BodyParsed {
	fail := func(err error) define.BodyParsed {
		return define.MixupBodyParsed(define.ERROR_CODE_PARSE_CONTENT_FAILED, err.Error(), time.Now(), nil)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	// Credentials remain anchored to the originally configured source, even if
	// redirects or a document base move extraction to another origin.
	options := []network.RequestOptions{network.OptionsForConfigContext(ctx, config)}
	if len(documentBase) > 0 && documentBase[0] != "" {
		config.URL = documentBase[0]
	}
	wire, err := decodeCSRWire(ctx, data)
	if err != nil {
		return fail(err)
	}
	idRegexp, err := config.CompileIDRegexp()
	if err != nil {
		return fail(err)
	}
	items := make([]define.InfoItem, len(wire))
	rawLinks := make([]string, len(wire))
	for i, item := range wire {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		items[i] = item.InfoItem
		rawLinks[i] = item.Link
		if item.Link != "" {
			absolute, err := fn.LinkResolveRelative(item.Link, config.URL)
			if err != nil {
				return fail(fmt.Errorf("entry %d link is invalid: %w", i+1, err))
			}
			items[i].Link = absolute
		}
		if items[i].ID == "" && idRegexp != nil {
			match := idRegexp.FindStringSubmatch(rawLinks[i])
			if len(match) > 1 {
				items[i].ID = match[1]
			}
		}
	}
	if !limits.ItemsWithinBudget(items) {
		return fail(fmt.Errorf("feed exceeds item or content size limit"))
	}
	result := fillDetailsWithLinks(ctx, config, items, options, func(i int, field string, hook define.ConfigHook) string {
		item := wire[i]
		if item.Native || item.Hooks != nil {
			return item.Hooks.link(field)
		}
		// Plain custom output has no DOM selector context. An empty hook URL uses
		// its declared item link; nonempty selectors need explicit captured metadata.
		if hook.URL == "" {
			return rawLinks[i]
		}
		expression := define.ParseFieldExpression(hook.URL)
		if !expression.Selector {
			return expression.Value
		}
		return ""
	}, true)
	if result != nil {
		return *result
	}
	if !limits.ItemsWithinBudget(items) {
		return fail(fmt.Errorf("feed exceeds item or content size limit"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return define.MixupBodyParsed(define.ERROR_CODE_NULL, define.ERROR_STATUS_NULL, time.Now(), items)
}
