package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/chromedp/cdproto/cdp"
	cdpnetwork "github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/soulteary/RSS-Can/internal/limits"
)

// browserDocument keeps navigation provenance with raw extraction across cache
// hits. Body is JSON items for CSR and HTML for MIX; neither loses its URL base.
type browserDocument struct {
	Version    int    `json:"version"`
	Body       string `json:"body"`
	FinalURL   string `json:"final_url"`
	BaseURL    string `json:"base_url"`
	HTTPStatus int    `json:"http_status"`
}

func validateBrowserURL(raw string) bool {
	if len(raw) > limits.MaxDocumentURLBytes {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && u.Hostname() != "" && (u.Scheme == "http" || u.Scheme == "https")
}
func (d browserDocument) validate() error {
	if d.Version != 1 {
		return fmt.Errorf("invalid browser document version")
	}
	if d.HTTPStatus != http.StatusOK {
		return fmt.Errorf("browser document returned HTTP %d", d.HTTPStatus)
	}
	if !validateBrowserURL(d.FinalURL) || !validateBrowserURL(d.BaseURL) {
		return fmt.Errorf("invalid browser document URL metadata")
	}
	if len(d.Body) > limits.MaxPageBytes {
		return fmt.Errorf("browser result exceeds size limit")
	}
	return nil
}
func encodeBrowserDocument(document browserDocument) (string, error) {
	if err := document.validate(); err != nil {
		return "", err
	}
	var encoded strings.Builder
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(document); err != nil {
		return "", err
	}
	if encoded.Len() > limits.MaxDocumentCacheBytes {
		return "", fmt.Errorf("browser document record exceeds size limit")
	}
	return encoded.String(), nil
}
func decodeBrowserDocument(encoded string) (browserDocument, error) {
	var document browserDocument
	if len(encoded) > limits.MaxDocumentCacheBytes {
		return document, fmt.Errorf("browser document record exceeds size limit")
	}
	if err := json.Unmarshal([]byte(encoded), &document); err != nil {
		return document, err
	}
	return document, document.validate()
}
func validateBrowserDocument(encoded string) error {
	_, err := decodeBrowserDocument(encoded)
	return err
}

// Only responses for the opened main frame are retained, keyed by loader.
// Checking the currently committed loader also handles script navigation while
// a rule waits for its readiness marker, rather than trusting an earlier page.
type mainDocumentResponses struct {
	mu       sync.Mutex
	frame    cdp.FrameID
	statuses map[cdp.LoaderID]int
	order    []cdp.LoaderID
	changed  chan struct{}
	err      error
}

func (r *mainDocumentResponses) record(event *cdpnetwork.EventResponseReceived) {
	if event.FrameID != r.frame || event.Type != cdpnetwork.ResourceTypeDocument || event.Response == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.statuses[event.LoaderID]; !ok {
		r.order = append(r.order, event.LoaderID)
	}
	r.statuses[event.LoaderID] = int(event.Response.Status)
	if len(r.order) > 16 {
		delete(r.statuses, r.order[0])
		r.order = r.order[1:]
	}
	select {
	case r.changed <- struct{}{}:
	default:
	}
}
func startMainDocumentResponses(ctx context.Context) (*mainDocumentResponses, context.CancelFunc, error) {
	executor := chromedp.FromContext(ctx).Target
	tree, err := cdp.Call(ctx, executor, page.GetFrameTree, cdp.Empty{})
	if err != nil {
		return nil, nil, err
	}
	r := &mainDocumentResponses{frame: tree.FrameTree.Frame.ID, statuses: make(map[cdp.LoaderID]int), changed: make(chan struct{}, 1)}
	eventsCtx, cancel := context.WithCancel(ctx)
	events := cdp.Events(eventsCtx, executor, cdpnetwork.ResponseReceived)
	go func() {
		for event, err := range events {
			if err != nil {
				r.mu.Lock()
				r.err = err
				r.mu.Unlock()
				select {
				case r.changed <- struct{}{}:
				default:
				}
				return
			}
			r.record(&event)
		}
	}()
	if _, err := cdp.Call(ctx, executor, cdpnetwork.Enable, cdpnetwork.EnableParams{}); err != nil {
		cancel()
		return nil, nil, err
	}
	return r, cancel, nil
}

type browserNavigation struct {
	Loader cdp.LoaderID
	Status int
}

func (r *mainDocumentResponses) currentDocument(ctx context.Context) (browserNavigation, error) {
	tree, err := cdp.Call(ctx, chromedp.FromContext(ctx).Target, page.GetFrameTree, cdp.Empty{})
	if err != nil {
		return browserNavigation{}, err
	}
	frame := tree.FrameTree.Frame
	if frame.ID != r.frame {
		return browserNavigation{}, fmt.Errorf("main document frame changed")
	}
	for {
		r.mu.Lock()
		status, ok := r.statuses[frame.LoaderID]
		eventErr := r.err
		r.mu.Unlock()
		if ok {
			return browserNavigation{Loader: frame.LoaderID, Status: status}, nil
		}
		if eventErr != nil {
			return browserNavigation{}, eventErr
		}
		select {
		case <-ctx.Done():
			return browserNavigation{}, ctx.Err()
		case <-r.changed:
		}
	}
}

func browserDocumentExpression(expression string) string {
	return fmt.Sprintf(`(()=>{const body=(%s);const finalURL=location.href;let baseURL=document.baseURI;try{const parsed=new URL(baseURL);if(parsed.protocol!=="http:"&&parsed.protocol!=="https:")baseURL=finalURL;}catch{baseURL=finalURL;}if(finalURL.length>%d||baseURL.length>%d||new TextEncoder().encode(finalURL).length>%d||new TextEncoder().encode(baseURL).length>%d)throw new Error("browser URL exceeds size limit");return {version:1,body,final_url:finalURL,base_url:baseURL,http_status:200};})()`, boundedBrowserExpression(expression), limits.MaxDocumentURLBytes, limits.MaxDocumentURLBytes, limits.MaxDocumentURLBytes, limits.MaxDocumentURLBytes)
}
