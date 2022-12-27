package fn

import (
	"net"
	"regexp"
	"strings"
)

// ref: https://developer.mozilla.org/en-US/docs/Web/HTML/Element
var DomList = func() []string {
	return []string{"html", "base", "head", "link", "meta", "style", "title", "body", "address", "article", "aside", "footer", "header", "h1", "h2", "h3", "h4", "h5", "h6", "main", "nav", "section", "blockquote", "dd", "div", "dl", "dt", "figcaption", "figure", "hr", "li", "menu", "ol", "p", "pre", "ul", "a", "abbr", "b", "bdi", "bdo", "br", "cite", "code", "data", "dfn", "em", "i", "kbd", "mark", "q", "rp", "rt", "ruby", "s", "samp", "small", "span", "strong", "sub", "sup", "time", "u", "var", "wbr", "area", "audio", "img", "map", "track", "video", "embed", "iframe", "object", "picture", "portal", "source", "svg", "math", "canvas", "noscript", "script", "del", "ins", "caption", "col", "colgroup", "table", "tbody", "td", "tfoot", "th", "thead", "tr", "button", "datalist", "fieldset", "form", "input", "label", "legend", "meter", "optgroup", "option", "output", "progress", "select", "textarea", "details", "dialog", "summary", "slot", "template", "acronym", "applet", "bgsound", "big", "blink", "center", "content", "dir", "font", "frame", "frameset", "image", "keygen", "marquee", "menuitem", "nobr", "noembed", "noframes", "param", "plaintext", "rb", "rtc", "shadow", "spacer", "strike", "tt", "xmp"}
}()

func IsDomTagName(field string) bool {
	for _, dom := range DomList {
		if strings.Contains(field, dom) {
			return true
		}
	}
	return false
}

func IsCssSelector(field string) bool {
	return strings.Contains(field, ".") || strings.Contains(field, "#") || strings.Contains(field, " ") || strings.Contains(field, ">")
}

func IsStrInArray(arr []string, s string) bool {
	for _, a := range arr {
		if a == s {
			return true
		}
	}
	return false
}

func IsVaildPortRange(port int) bool {
	return port > 0 && port < 65535
}

func IsNotEmptyAndNotDefaultString(value string, defaults string) bool {
	return value != "" && value != defaults
}

func IsVaildLogLevel(level string) bool {
	s := strings.ToLower(level)
	return s == "info" || s == "error" || s == "warn" || s == "debug"
}

func IsBoolString(input string) bool {
	s := strings.ToLower(input)
	if s == "true" || s == "1" || s == "on" {
		return true
	}
	return false
}

func IsVaildAddr(addr string) bool {
	host := strings.TrimSpace(addr)
	port := ""

	if strings.Contains(addr, ":") {
		arr := strings.Split(addr, ":")
		host = strings.TrimSpace(arr[0])
		port = strings.TrimSpace(arr[1])

		if port != "" {
			p := StringToPositiveInteger(port)
			if p < 0 {
				return false
			}
			if !IsVaildPortRange(p) {
				return false
			}
		} else {
			// empty port
			return false
		}
	}

	var ipRegexp = regexp.MustCompile(`^(\d+\.){3}\d+$`)
	if ipRegexp.MatchString(host) {
		return net.ParseIP(host) != nil
	}

	var domainRegexp = regexp.MustCompile(`^([\w\d\-\_\.]+)?[\w\d\-\_]$`)
	if domainRegexp.MatchString(host) {
		return !regexp.MustCompile(`^([\d\.]+)?[\d]$`).MatchString(host)
	}
	return false
}
