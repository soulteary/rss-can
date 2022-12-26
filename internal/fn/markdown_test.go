package fn_test

import (
	"testing"

	"github.com/soulteary/RSS-Can/internal/fn"
)

func TestHtml2Md(t *testing.T) {
	title := fn.Html2Md("<h1>Title</h1>")
	if title != "# Title" {
		t.Fatal("markdown convert test failed")
	}
}
