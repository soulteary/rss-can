package server

import (
	"fmt"
	"path"
	"strings"
)

// ValidateFeedPath prevents a configured feed route from shadowing a built-in
// namespace. Empty paths are left to the application validation; utility-only
// embedders have historically omitted the feed path.
func ValidateFeedPath(value string, team bool) error {
	if value == "" {
		return nil
	}
	clean := strings.TrimRight(value, "/")
	if clean == "" || !strings.HasPrefix(clean, "/") || path.Clean(clean) != clean || strings.ContainsAny(clean, ":*+?#\\") {
		return fmt.Errorf("RSS_HTTP_FEED_PATH must be a static absolute path outside reserved routes")
	}
	first := strings.ToLower(strings.Split(strings.TrimPrefix(clean, "/"), "/")[0])
	reserved := first == "config" || first == "inspector" || first == "assets" || first == "_"
	if team {
		reserved = reserved || first == "api" || first == "sources" || first == "s"
	}
	if reserved {
		return fmt.Errorf("RSS_HTTP_FEED_PATH conflicts with a reserved route namespace")
	}
	return nil
}
