package limits

import (
	"github.com/soulteary/RSS-Can/internal/define"
	"strings"
	"testing"
)

func TestItemsBudgetIncludesAllRetainedFields(t *testing.T) {
	if !ItemsWithinBudget([]define.InfoItem{{ID: "id", Title: "title", Link: "link", Date: "date", Author: "author", Category: "category", Description: "description", Content: "content"}}) {
		t.Fatal("small feed rejected")
	}
	if ItemsWithinBudget(make([]define.InfoItem, MaxItems+1)) {
		t.Fatal("too many items accepted")
	}
	if ItemsWithinBudget([]define.InfoItem{{Content: strings.Repeat("x", MaxFeedBytes), Title: "overflow"}}) {
		t.Fatal("aggregate field bytes ignored")
	}
	if !ItemsWithinBudget([]define.InfoItem{{Content: strings.Repeat("x", MaxFeedBytes)}}) {
		t.Fatal("exact budget rejected")
	}
}
