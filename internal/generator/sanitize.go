package generator

import (
	"context"
	"errors"
	"html"
	"io"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/soulteary/RSS-Can/internal/limits"
	xhtml "golang.org/x/net/html"
)

// Feed fragments are untrusted even when their containing XML/JSON is escaped.
// This deliberately small allowlist retains article formatting, not page UI.
var feedTags = map[string]bool{
	"a": true, "abbr": true, "b": true, "blockquote": true, "br": true, "cite": true,
	"code": true, "dd": true, "del": true, "div": true, "dl": true, "dt": true, "em": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "hr": true,
	"i": true, "kbd": true, "li": true, "ol": true, "p": true, "pre": true, "q": true,
	"s": true, "samp": true, "small": true, "span": true, "strong": true, "sub": true,
	"sup": true, "table": true, "tbody": true, "td": true, "tfoot": true, "th": true,
	"thead": true, "time": true, "tr": true, "ul": true, "var": true,
}

var dropFeedSubtrees = map[string]bool{
	"applet": true, "audio": true, "canvas": true, "embed": true, "frameset": true,
	"iframe": true, "math": true, "noembed": true, "noframes": true, "noscript": true,
	"object": true, "plaintext": true, "script": true, "style": true, "svg": true,
	"template": true, "textarea": true, "video": true, "xmp": true,
}

// sanitizeFragment emits only fixed tag/attribute names and escaped text. It
// never returns raw tokenizer bytes, so malformed markup cannot escape the
// allowlist. No resource-loading element or CSS is retained; images become alt
// text to avoid making subscription reads into third-party tracking requests.
func sanitizeFragment(ctx context.Context, input, base string, maxBytes int) (string, error) {
	if len(input) > limits.MaxFeedBytes || maxBytes < 0 {
		return "", errors.New("feed HTML exceeds content budget")
	}
	z := xhtml.NewTokenizer(strings.NewReader(input))
	z.SetMaxBuf(limits.MaxFeedBytes)
	var out strings.Builder
	write := func(value string) error {
		if len(value) > maxBytes-out.Len() {
			return errors.New("sanitized feed HTML exceeds content budget")
		}
		out.WriteString(value)
		return nil
	}
	writeEscaped := func(prefix, value, suffix string) error {
		needed := len(prefix) + len(value) + len(suffix)
		for i := 0; i < len(value); i++ {
			switch value[i] {
			case '&', '\'', '"':
				needed += 4
			case '<', '>':
				needed += 3
			}
			if needed > maxBytes-out.Len() {
				return errors.New("sanitized feed HTML exceeds content budget")
			}
		}
		if needed > maxBytes-out.Len() {
			return errors.New("sanitized feed HTML exceeds content budget")
		}
		return write(prefix + html.EscapeString(value) + suffix)
	}
	stack := make([]string, 0, 16)
	skip := ""
	skipDepth := 0
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		kind := z.Next()
		if kind == xhtml.ErrorToken {
			if err := z.Err(); err != io.EOF {
				return "", errors.New("invalid or oversized feed HTML")
			}
			break
		}
		// Bound attribute materialization separately from the whole fragment:
		// one giant start tag must not allocate an enormous attribute slice.
		if (kind == xhtml.StartTagToken || kind == xhtml.SelfClosingTagToken) && len(z.Raw()) > 64<<10 {
			return "", errors.New("feed HTML tag exceeds size limit")
		}
		token := z.Token()
		if len(token.Attr) > 128 {
			return "", errors.New("feed HTML attribute count exceeds limit")
		}
		name := strings.ToLower(token.Data)
		if skip != "" {
			if kind == xhtml.StartTagToken && name == skip {
				skipDepth++
			}
			if kind == xhtml.EndTagToken && name == skip {
				skipDepth--
				if skipDepth == 0 {
					skip = ""
				}
			}
			continue
		}
		switch kind {
		case xhtml.TextToken:
			if err := writeEscaped("", token.Data, ""); err != nil {
				return "", err
			}
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			if dropFeedSubtrees[name] {
				// embed is a void element; consuming its following article would
				// be unnecessary. Every other dropped subtree is inertly omitted.
				if name != "embed" && kind == xhtml.StartTagToken {
					skip = name
					skipDepth = 1
				}
				continue
			}
			if name == "img" {
				for _, attr := range token.Attr {
					if attr.Namespace == "" && attr.Key == "alt" {
						if err := writeEscaped("", attr.Val, ""); err != nil {
							return "", err
						}
						break
					}
				}
				continue
			}
			if !feedTags[name] {
				continue
			}
			if len(stack) >= 128 {
				return "", errors.New("feed HTML nesting exceeds limit")
			}
			if err := write("<" + name); err != nil {
				return "", err
			}
			seen := map[string]bool{}
			for _, attr := range token.Attr {
				if attr.Namespace != "" || seen[attr.Key] {
					continue
				}
				seen[attr.Key] = true
				key, value := attr.Key, attr.Val
				allowed := false
				switch {
				case key == "title" && len(value) <= 1024:
					allowed = true
				case key == "href" && name == "a":
					value, allowed = safeFeedLink(value, base, true)
				case (key == "colspan" || key == "rowspan") && (name == "td" || name == "th"):
					n, err := strconv.Atoi(value)
					if err == nil && n >= 1 && n <= 100 {
						value = strconv.Itoa(n)
						allowed = true
					}
				case key == "dir" && (value == "ltr" || value == "rtl" || value == "auto"):
					allowed = true
				}
				if allowed {
					if err := writeEscaped(" "+key+"=\"", value, "\""); err != nil {
						return "", err
					}
				}
			}
			if name == "a" {
				if err := write(` rel="noopener noreferrer"`); err != nil {
					return "", err
				}
			}
			if name == "br" || name == "hr" {
				if err := write("/>"); err != nil {
					return "", err
				}
				continue
			}
			if err := write(">"); err != nil {
				return "", err
			}
			if kind == xhtml.SelfClosingTagToken {
				if err := write("</" + name + ">"); err != nil {
					return "", err
				}
			} else {
				stack = append(stack, name)
			}
		case xhtml.EndTagToken:
			match := -1
			for i := len(stack) - 1; i >= 0; i-- {
				if stack[i] == name {
					match = i
					break
				}
			}
			if match >= 0 {
				for i := len(stack) - 1; i >= match; i-- {
					if err := write("</" + stack[i] + ">"); err != nil {
						return "", err
					}
				}
				stack = stack[:match]
			}
		}
	}
	for i := len(stack) - 1; i >= 0; i-- {
		if err := write("</" + stack[i] + ">"); err != nil {
			return "", err
		}
	}
	return out.String(), nil
}

func safeFeedLink(raw, base string, mail bool) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	for _, r := range raw {
		if r <= 32 || r == 127 {
			return "", false
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil {
		return "", false
	}
	if u.Scheme == "" {
		b, err := url.Parse(base)
		if err != nil || (b.Scheme != "http" && b.Scheme != "https") || b.Hostname() == "" || b.User != nil {
			return "", false
		}
		u = b.ResolveReference(u)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		if u.Hostname() == "" || u.Opaque != "" || u.User != nil {
			return "", false
		}
	case "mailto":
		if !mail || u.Opaque == "" || u.Host != "" {
			return "", false
		}
	default:
		return "", false
	}
	return u.String(), true
}

// JSON Feed summary is plain text, unlike RSS description or content_html.
// Decode the already-sanitized fragment rather than showing HTML entities and
// tags as text in clients that correctly implement that field's contract.
func plainFeedSummary(ctx context.Context, fragment string) (string, error) {
	z := xhtml.NewTokenizer(strings.NewReader(fragment))
	var out strings.Builder
	boundary := false
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		kind := z.Next()
		if kind == xhtml.ErrorToken {
			if z.Err() != io.EOF {
				return "", errors.New("invalid sanitized summary")
			}
			break
		}
		token := z.Token()
		if kind == xhtml.TextToken {
			if boundary && out.Len() > 0 && token.Data != "" {
				last, _ := utf8.DecodeLastRuneInString(out.String())
				first, _ := utf8.DecodeRuneInString(token.Data)
				if !unicode.IsSpace(last) && !unicode.IsSpace(first) {
					out.WriteByte(' ')
				}
			}
			out.WriteString(token.Data)
			if token.Data != "" {
				boundary = false
			}
		} else if kind == xhtml.StartTagToken || kind == xhtml.SelfClosingTagToken || kind == xhtml.EndTagToken {
			// Inline formatting does not create word boundaries. Block tags and
			// explicit breaks do, including an opening block after inline text.
			switch token.Data {
			case "blockquote", "br", "dd", "div", "dl", "dt", "h1", "h2", "h3", "h4", "h5", "h6", "hr", "li", "ol", "p", "pre", "table", "tbody", "td", "tfoot", "th", "thead", "tr", "ul":
				boundary = true
			}
		}
	}
	return strings.TrimSpace(out.String()), nil
}
