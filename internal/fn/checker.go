package fn

import "strings"

func IsDomTagName(field string) bool {
	domList := []string{"p", "span", "em", "strong", "a", "ul", "li", "ol", "dl", "h1", "h2", "h3", "h4", "h5", "h6"}
	for _, dom := range domList {
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
