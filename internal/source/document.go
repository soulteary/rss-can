package source

import (
	"errors"
	"io"
	"strings"

	"golang.org/x/net/html"
)

const MaxDocumentBytes = 1 << 20
const MaxDocumentNodes = 10000
const MaxDocumentDepth = 48
const MaxItems = 100

var ErrDocumentBudget = errors.New("page exceeds declarative source complexity budget")

// ValidateDocument bounds work before any CSS matching. The lexical pass also
// rejects pathological nesting before building a DOM; the second pass checks
// the actual repaired HTML tree. It is intentionally conservative.
func ValidateDocument(body string) error {
	if len(body) > MaxDocumentBytes {
		return ErrDocumentBudget
	}
	z := html.NewTokenizer(strings.NewReader(body))
	tokens, depth := 0, 0
	void := map[string]bool{"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true, "img": true, "input": true, "link": true, "meta": true, "param": true, "source": true, "track": true, "wbr": true}
	for {
		kind := z.Next()
		if kind == html.ErrorToken {
			if z.Err() != io.EOF {
				return ErrDocumentBudget
			}
			break
		}
		tokens++
		if tokens > MaxDocumentNodes {
			return ErrDocumentBudget
		}
		switch kind {
		case html.StartTagToken:
			name, _ := z.TagName()
			if !void[string(name)] {
				depth++
				if depth > MaxDocumentDepth {
					return ErrDocumentBudget
				}
			}
		case html.EndTagToken:
			if depth > 0 {
				depth--
			}
		}
	}
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return ErrDocumentBudget
	}
	type entry struct {
		node  *html.Node
		depth int
	}
	stack := []entry{{doc, 0}}
	nodes := 0
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		nodes++
		if nodes > MaxDocumentNodes || current.depth > MaxDocumentDepth {
			return ErrDocumentBudget
		}
		for child := current.node.FirstChild; child != nil; child = child.NextSibling {
			stack = append(stack, entry{child, current.depth + 1})
		}
	}
	return nil
}
