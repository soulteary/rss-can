package fn

import "strings"

var DomList = func() []string {
	return []string{"p", "span", "em", "strong", "a", "ul", "li", "ol", "dl", "h1", "h2", "h3", "h4", "h5", "h6"}
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
