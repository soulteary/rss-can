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

	empty := fn.Html2Md("")
	if empty != "" {
		t.Fatal("markdown convert test failed")
	}

	unstructured := fn.Html2Md("<htm1l><body><aef<eqf>>>qq></body></ht>")
	if unstructured == "" {
		t.Fatal("markdown convert test failed")
	}
}
