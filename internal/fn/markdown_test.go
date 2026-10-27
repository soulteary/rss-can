package fn_test

import (
	"strings"
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

func TestMarkdownUpgradePreservesFeedContent(t *testing.T) {
	for _, tc := range []struct {
		name, html string
		fragments  []string
	}{
		{"headings and Unicode", `<h2>中文标题</h2><p>正文 <strong>重点</strong> <em>强调</em></p>`, []string{"## 中文标题", "正文", "**重点**", "*强调*"}},
		{"links and images", `<p><a href="https://example.org/post">Read</a><img src="https://example.org/image.png" alt="Photo"></p>`, []string{"[Read](https://example.org/post)", "![Photo](https://example.org/image.png)"}},
		{"ordered list start", `<ol start="3"><li>Third</li><li>Fourth</li></ol>`, []string{"3. Third", "4. Fourth"}},
		{"code and blockquote", `<blockquote><p>Quoted</p></blockquote><pre><code>if (x &lt; 3) { return x; }</code></pre>`, []string{"> Quoted", "if (x < 3) { return x; }"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := fn.Html2Md(tc.html)
			for _, fragment := range tc.fragments {
				if !strings.Contains(got, fragment) {
					t.Errorf("missing %q in %q", fragment, got)
				}
			}
		})
	}
}
