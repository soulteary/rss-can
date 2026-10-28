package network

import (
	"context"
	"errors"
	escapedhtml "html"
	"net/url"
	"strings"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/limits"
	"github.com/soulteary/goquery"
	"golang.org/x/net/html"
)

// walkDetailElements visits the active document in tree order. Template
// contents are inert and must not supply a base or acquire enclosing URLs.
func walkDetailElements(root *html.Node, visit func(*html.Node) bool) {
	for node := root; node != nil; {
		inert := node.Type == html.ElementNode && node.Namespace == "" && node.Data == "template"
		if !inert && node.Type == html.ElementNode && !visit(node) {
			return
		}
		if !inert && node.FirstChild != nil {
			node = node.FirstChild
			continue
		}
		for node != root && node.NextSibling == nil {
			node = node.Parent
		}
		if node == root {
			return
		}
		node = node.NextSibling
	}
}

func detailDocumentBase(doc define.RemoteBodySanitized, document *goquery.Document) string {
	if validDocumentURL(doc.BaseURL) {
		return doc.BaseURL
	}
	base := doc.FinalURL
	for _, root := range document.Nodes {
		found := false
		walkDetailElements(root, func(node *html.Node) bool {
			if node.Namespace != "" || node.Data != "base" {
				return true
			}
			for _, attr := range node.Attr {
				if attr.Key != "href" {
					continue
				}
				found = true
				initial, err := url.Parse(doc.FinalURL)
				if err != nil {
					return false
				}
				ref, err := url.Parse(strings.TrimSpace(attr.Val))
				if err == nil {
					resolved := initial.ResolveReference(ref).String()
					if validDocumentURL(resolved) {
						base = resolved
					}
				}
				// The first href-bearing base wins, including an invalid value.
				return false
			}
			return true
		})
		if found {
			break
		}
	}
	return base
}

// Only detached detail HTML is rewritten. Non-web schemes and malformed URLs
// retain their existing conversion behavior; relative web references use the
// final document/base URL. Rewriting never changes request cookie authority.
func normalizeDetailURLs(ctx context.Context, selection *goquery.Selection, base string) error {
	parsedBase, err := url.Parse(base)
	if err != nil {
		return err
	}
	// selection.Html emits the first selection's children, not the selected
	// root or the surrounding upstream document. Count exactly that content
	// without materializing another potentially large HTML string.
	counter := &detailHTMLCounter{ctx: ctx}
	for _, root := range selection.First().Nodes {
		for child := root.FirstChild; child != nil; child = child.NextSibling {
			if err := html.Render(counter, child); err != nil {
				return err
			}
		}
	}
	retained := counter.bytes
	visit := func(node *html.Node) bool {
		if err = ctx.Err(); err != nil {
			return false
		}
		for i, attr := range node.Attr {
			if attr.Key != "href" && attr.Key != "src" {
				continue
			}
			raw := strings.TrimSpace(attr.Val)
			if raw == "" {
				continue
			}
			ref, parseErr := url.Parse(raw)
			if parseErr != nil || ref.Scheme != "" {
				continue
			}
			resolved := parsedBase.ResolveReference(ref).String()
			growth := len(escapedhtml.EscapeString(resolved)) - len(escapedhtml.EscapeString(attr.Val))
			if growth > 0 {
				if growth > limits.MaxFeedBytes-retained {
					err = errors.New("detail URL expansion exceeds content size limit")
					return false
				}
				retained += growth
			}
			node.Attr[i].Val = resolved
		}
		return true
	}
	for _, root := range selection.First().Nodes {
		if root.Type == html.ElementNode && root.Namespace == "" && root.Data == "template" {
			continue
		}
		for child := root.FirstChild; child != nil; child = child.NextSibling {
			walkDetailElements(child, visit)
			if err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}

// detailHTMLCounter bounds serialized inner HTML while retaining no content.
type detailHTMLCounter struct {
	ctx   context.Context
	bytes int
}

func (w *detailHTMLCounter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if len(p) > limits.MaxPageBytes-w.bytes {
		return 0, errors.New("detail HTML exceeds content size limit")
	}
	w.bytes += len(p)
	return len(p), nil
}
