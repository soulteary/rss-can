package limits

import "github.com/soulteary/RSS-Can/internal/define"

// ItemsWithinBudget bounds retained field bytes before dates or serialization
// allocate additional representations. It is deliberately independent of the
// final encoded output budget, which also accounts for escaping and metadata.
func ItemsWithinBudget(items []define.InfoItem) bool {
	if len(items) > MaxItems {
		return false
	}
	remaining := MaxFeedBytes
	for _, item := range items {
		for _, field := range []string{item.ID, item.Title, item.Link, item.Date, item.Author, item.Category, item.Description, item.Content} {
			if len(field) > remaining {
				return false
			}
			remaining -= len(field)
		}
	}
	return true
}
