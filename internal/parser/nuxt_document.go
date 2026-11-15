package parser

import (
	"html"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// nuxtStoriesAsArticleHTML reads article cards embedded in a Nuxt payload.
// Sites such as BAAI server-render an empty list and put the stories in
// window.__NUXT__, so CSS selectors on the initial HTML match nothing.
func nuxtStoriesAsArticleHTML(body string) (string, bool) {
	const marker = "window.__NUXT__=(function("
	start := strings.Index(body, marker)
	if start < 0 || !strings.Contains(body, "story_info") {
		return "", false
	}
	params, source, args, ok := parseNuxtCall(body[start+len("window.__NUXT__="):])
	if !ok || len(args) < len(params) {
		return "", false
	}
	env := make(map[string]string, len(params))
	for i, name := range params {
		env[name] = args[i]
	}
	var page strings.Builder
	page.WriteString("<!DOCTYPE html><html><body>")
	eachStory(source, func(fields map[string]string) {
		title := resolveNuxt(fields["title"], env)
		id := resolveNuxt(fields["story_id"], env)
		if id == "" {
			id = resolveNuxt(fields["id"], env)
		}
		if title == "" || !numericID(id) {
			return
		}
		author := resolveNuxt(fields["story_show_user_name"], env)
		summary := resolveNuxt(fields["summary"], env)
		when := nuxtStoryTime(resolveNuxt(fields["created_at"], env))
		page.WriteString(`<main><div class="story-list"><div class="story-list-container"><div class="story-item"><a href="`)
		page.WriteString(html.EscapeString("https://hub.baai.ac.cn/view/" + id))
		page.WriteString(`"><h6 class="story-item-title">`)
		page.WriteString(html.EscapeString(title))
		page.WriteString(`</h6>`)
		if author != "" {
			page.WriteString(`<span class="story-item-author-name">`)
			page.WriteString(html.EscapeString(author))
			page.WriteString(`</span>`)
		}
		if when != "" {
			page.WriteString(`<span class="story-item-time">`)
			page.WriteString(html.EscapeString(when))
			page.WriteString(`</span>`)
		}
		if summary != "" {
			page.WriteString(`<p class="story-item-summary">`)
			page.WriteString(html.EscapeString(summary))
			page.WriteString(`</p>`)
		}
		page.WriteString("</a></div></div></div></main>")
	})
	page.WriteString("</body></html>")
	if !strings.Contains(page.String(), "story-item") {
		return "", false
	}
	return page.String(), true
}

func nuxtStoryTime(raw string) string {
	raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), "分享"))
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04"} {
		parsed, err := time.ParseInLocation(layout, raw, time.FixedZone("CST", 8*3600))
		if err == nil {
			return parsed.Format(time.RFC3339)
		}
	}
	return ""
}

func numericID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func resolveNuxt(token string, env map[string]string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if strings.HasPrefix(token, `"`) || strings.HasPrefix(token, `'`) {
		text, ok := unescapeJSString(token)
		if !ok {
			return ""
		}
		return text
	}
	if value, ok := env[token]; ok {
		return resolveNuxt(value, nil)
	}
	return token
}

func parseNuxtCall(src string) (params []string, body string, args []string, ok bool) {
	const prefix = "(function("
	if !strings.HasPrefix(src, prefix) {
		return nil, "", nil, false
	}
	i := len(prefix)
	for {
		i = skipSpace(src, i)
		if i < len(src) && src[i] == ')' {
			i++
			break
		}
		name, next, valid := readIdent(src, i)
		if !valid {
			return nil, "", nil, false
		}
		params = append(params, name)
		i = skipSpace(src, next)
		if i < len(src) && src[i] == ',' {
			i++
			continue
		}
		if i < len(src) && src[i] == ')' {
			i++
			break
		}
		return nil, "", nil, false
	}
	i = skipSpace(src, i)
	if i >= len(src) || src[i] != '{' {
		return nil, "", nil, false
	}
	end, valid := matchDelim(src, i, '{', '}')
	if !valid {
		return nil, "", nil, false
	}
	body = src[i : end+1]
	i = skipSpace(src, end+1)
	if i >= len(src) || src[i] != '(' {
		return nil, "", nil, false
	}
	argEnd, valid := matchDelim(src, i, '(', ')')
	if !valid {
		return nil, "", nil, false
	}
	args, valid = splitJSList(src[i+1 : argEnd])
	if !valid {
		return nil, "", nil, false
	}
	return params, body, args, true
}

func eachStory(body string, visit func(map[string]string)) {
	for i := 0; i < len(body); {
		if body[i] == '"' || body[i] == '\'' {
			next, ok := skipJSString(body, i)
			if !ok {
				return
			}
			i = next
			continue
		}
		const key = "story_info:"
		if strings.HasPrefix(body[i:], key) {
			j := skipSpace(body, i+len(key))
			if j < len(body) && body[j] == '{' {
				end, ok := matchDelim(body, j, '{', '}')
				if !ok {
					return
				}
				fields, ok := parseJSObject(body[j : end+1])
				if ok {
					restEnd := len(body)
					if next := strings.Index(body[end+1:], key); next >= 0 {
						restEnd = end + 1 + next
					}
					if id, valid := readField(body[end+1:restEnd], "story_id:"); valid {
						fields["story_id"] = id
					}
					visit(fields)
				}
				i = end + 1
				continue
			}
		}
		i++
	}
}

func parseJSObject(src string) (map[string]string, bool) {
	if len(src) < 2 || src[0] != '{' || src[len(src)-1] != '}' {
		return nil, false
	}
	fields := map[string]string{}
	parts, ok := splitJSList(src[1 : len(src)-1])
	if !ok {
		return nil, false
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		name, rest, valid := readIdent(part, 0)
		if !valid {
			continue
		}
		rest = skipSpace(part, rest)
		if rest >= len(part) || part[rest] != ':' {
			continue
		}
		fields[name] = strings.TrimSpace(part[rest+1:])
	}
	return fields, true
}

func readField(src, key string) (string, bool) {
	at := strings.Index(src, key)
	if at < 0 {
		return "", false
	}
	i := skipSpace(src, at+len(key))
	if i >= len(src) {
		return "", false
	}
	if src[i] == '"' || src[i] == '\'' {
		end, ok := skipJSString(src, i)
		if !ok {
			return "", false
		}
		return src[i:end], true
	}
	if name, _, ok := readIdent(src, i); ok {
		return name, true
	}
	end := i
	for end < len(src) && (src[end] == '-' || unicode.IsDigit(rune(src[end]))) {
		end++
	}
	if end == i {
		return "", false
	}
	return src[i:end], true
}

func splitJSList(src string) ([]string, bool) {
	var parts []string
	start := 0
	for i := 0; i < len(src); {
		switch src[i] {
		case '"', '\'':
			next, ok := skipJSString(src, i)
			if !ok {
				return nil, false
			}
			i = next
		case '{', '[', '(':
			close := map[byte]byte{'{': '}', '[': ']', '(': ')'}[src[i]]
			end, ok := matchDelim(src, i, src[i], close)
			if !ok {
				return nil, false
			}
			i = end + 1
		case ',':
			parts = append(parts, src[start:i])
			start = i + 1
			i++
		default:
			i++
		}
	}
	if tail := strings.TrimSpace(src[start:]); tail != "" || len(parts) > 0 {
		parts = append(parts, src[start:])
	}
	return parts, true
}

func matchDelim(src string, i int, open, close byte) (int, bool) {
	depth := 0
	for i < len(src) {
		switch src[i] {
		case '"', '\'':
			next, ok := skipJSString(src, i)
			if !ok {
				return 0, false
			}
			i = next
			continue
		}
		if src[i] == open {
			depth++
		} else if src[i] == close {
			depth--
			if depth == 0 {
				return i, true
			}
		}
		i++
	}
	return 0, false
}

func skipJSString(src string, i int) (int, bool) {
	quote := src[i]
	i++
	for i < len(src) {
		if src[i] == '\\' {
			i += 2
			continue
		}
		if src[i] == quote {
			return i + 1, true
		}
		i++
	}
	return 0, false
}

func unescapeJSString(token string) (string, bool) {
	if len(token) < 2 {
		return "", false
	}
	quote := token[0]
	if token[len(token)-1] != quote {
		return "", false
	}
	raw := token[1 : len(token)-1]
	var out strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			out.WriteByte(raw[i])
			continue
		}
		i++
		if i >= len(raw) {
			return "", false
		}
		switch raw[i] {
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 't':
			out.WriteByte('\t')
		case 'u':
			if i+4 >= len(raw) {
				return "", false
			}
			value, err := strconv.ParseUint(raw[i+1:i+5], 16, 32)
			if err != nil {
				return "", false
			}
			out.WriteRune(rune(value))
			i += 4
		default:
			out.WriteByte(raw[i])
		}
	}
	return out.String(), true
}

func readIdent(src string, i int) (string, int, bool) {
	if i >= len(src) {
		return "", i, false
	}
	r := rune(src[i])
	if r != '_' && r != '$' && !unicode.IsLetter(r) {
		return "", i, false
	}
	j := i + 1
	for j < len(src) {
		r = rune(src[j])
		if r != '_' && r != '$' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			break
		}
		j++
	}
	return src[i:j], j, true
}

func skipSpace(src string, i int) int {
	for i < len(src) {
		switch src[i] {
		case ' ', '\n', '\r', '\t':
			i++
		default:
			return i
		}
	}
	return i
}
