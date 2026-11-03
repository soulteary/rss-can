package generator

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/limits"
	xhtml "golang.org/x/net/html"
)

func sanitizeTest(t testing.TB, raw string) string {
	t.Helper()
	clean, err := sanitizeFragment(context.Background(), raw, "https://example.org/articles/", limits.MaxFeedBytes)
	if err != nil {
		t.Fatal(err)
	}
	return clean
}

func TestFeedHTMLAllowlistRetainsArticleFormatting(t *testing.T) {
	raw := `<h2 id="title">Title</h2><p>Hello <strong>reader</strong> &amp; friend.<br><a href="../post?a=1&amp;b=2" title="A &quot;link&quot;" target="_blank">Read</a></p><table><tr><td colspan="2">Cell</td></tr></table>`
	clean := sanitizeTest(t, raw)
	for _, want := range []string{`<h2>Title</h2>`, `<strong>reader</strong>`, `&amp; friend`, `<br/>`, `href="https://example.org/post?a=1&amp;b=2"`, `title="A &#34;link&#34;"`, `rel="noopener noreferrer"`, `colspan="2"`} {
		if !strings.Contains(clean, want) {
			t.Fatalf("missing %q from %s", want, clean)
		}
	}
	if strings.Contains(clean, "id=") || strings.Contains(clean, "target=") {
		t.Fatalf("unapproved attribute survived: %s", clean)
	}
	if again := sanitizeTest(t, clean); again != clean {
		t.Fatalf("not idempotent:\n%s\n%s", clean, again)
	}
}

func TestFeedHTMLDropsActiveMarkupAndTrackingResources(t *testing.T) {
	raw := `<p class="x" style="background:url(https://tracker/x)" onclick="secret()">Keep</p><script>script-secret</script><style>style-secret</style><svg><foreignObject><p>svg-secret</p></foreignObject></svg><math><mi>math-secret</mi></math><iframe srcdoc="<script>x</script>">iframe-secret</iframe><object data="https://tracker">object-secret</object><template>template-secret</template><video src="https://tracker">video-secret</video><form action="https://tracker"><input type="image" src="https://tracker"><button>Text</button></form><meta http-equiv="refresh" content="0;url=https://tracker"><link rel="stylesheet" href="https://tracker"><img src="https://tracker/pixel" srcset="https://tracker/x 2x" onerror="x" alt="Image &lt;text&gt;">`
	clean := sanitizeTest(t, raw)
	if clean != `<p>Keep</p>TextImage &lt;text&gt;` {
		t.Fatalf("unsafe or missing content: %s", clean)
	}
}

func TestFeedHTMLRejectsDangerousURLSchemes(t *testing.T) {
	for _, target := range []string{"javascript:alert(1)", "JaVaScRiPt:alert(1)", "java&#x73;cript:alert(1)", "java&#10;script:alert(1)", "data:text/html,x", "vbscript:x", "file:///etc/passwd", "blob:https://example.org/x", "https://user:secret@example.org/", "http:opaque"} {
		t.Run(target, func(t *testing.T) {
			clean := sanitizeTest(t, `<a href="`+target+`">Read</a>`)
			if strings.Contains(clean, "href=") {
				t.Fatalf("unsafe href: %s", clean)
			}
		})
	}
	for _, target := range []string{"https://example.org/post", "http://example.org/post", "//example.net/post", "/post", "#part", "mailto:reader@example.org"} {
		clean := sanitizeTest(t, `<a href="`+target+`">Read</a>`)
		if !strings.Contains(clean, "href=") {
			t.Fatalf("safe link removed: %s", target)
		}
	}
}

func TestFeedLinkValidationHandlesMissingBasesAndMail(t *testing.T) {
	for _, tc := range []struct {
		raw, base     string
		mail, allowed bool
	}{
		{"", "https://example.org", false, false},
		{"/relative", "", false, false},
		{"/relative", "javascript:x", false, false},
		{"https://user:password@example.org", "", false, false},
		{"mailto:reader@example.org", "", false, false},
		{"mailto:reader@example.org", "", true, true},
		{"mailto://example.org", "", true, false},
		{"https://example.org/path", "", false, true},
		{"http:opaque", "", false, false},
		{"https://example.org/a\npath", "", false, false},
	} {
		if _, ok := safeFeedLink(tc.raw, tc.base, tc.mail); ok != tc.allowed {
			t.Fatalf("unexpected decision for %+v", tc)
		}
	}
}

func TestFeedHTMLHandlesMalformedAndNestedInputs(t *testing.T) {
	for _, raw := range []string{
		`<svg><p><style><img src=x onerror=alert(1)></style></p></svg><p>safe</p>`,
		`<math><mtext><table><mglyph><style><!--</style><img title="--><img src=x onerror=alert(1)>">`,
		`<p title='a" onmouseover="alert(1)'>text<b>bold</p>tail</b>`,
		`<!--<script>alert(1)</script>--><p><a href="javascript:alert(1)"><i>text`,
		`<script/>alert(1)</script><p>text</p>`,
		`<textarea><img src=x onerror=x></textarea>`,
	} {
		clean := sanitizeTest(t, raw)
		assertSafeFragment(t, clean)
	}
}

func assertSafeFragment(t testing.TB, clean string) {
	t.Helper()
	z := xhtml.NewTokenizer(strings.NewReader(clean))
	for {
		kind := z.Next()
		if kind == xhtml.ErrorToken {
			return
		}
		tok := z.Token()
		if kind == xhtml.StartTagToken || kind == xhtml.SelfClosingTagToken {
			if !feedTags[tok.Data] {
				t.Fatalf("unsafe output element %q", tok.Data)
			}
			for _, a := range tok.Attr {
				if strings.HasPrefix(a.Key, "on") || a.Key == "style" || a.Key == "src" {
					t.Fatalf("unsafe output attribute %+v", a)
				}
				if a.Key == "href" {
					if _, ok := safeFeedLink(a.Val, "https://example.org", true); !ok {
						t.Fatalf("unsafe output href %q", a.Val)
					}
				}
			}
		}
	}
}

func TestFeedHTMLBudgetCancellationAndDepth(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := sanitizeFragment(ctx, "text", "", 100); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	var attributes strings.Builder
	attributes.WriteString("<p ")
	for i := range 129 {
		fmt.Fprintf(&attributes, "a%d=\"x\" ", i)
	}
	attributes.WriteString(">text</p>")
	for _, tc := range []struct {
		raw    string
		budget int
	}{{"&&&&", 10}, {`<p title="&&&&">text</p>`, 20}, {"text", -1}, {strings.Repeat("<p>", 129), 10000}, {strings.Repeat("x", limits.MaxFeedBytes+1), limits.MaxFeedBytes}, {`<p title="` + strings.Repeat("x", 64<<10) + `">text</p>`, limits.MaxFeedBytes}, {attributes.String(), limits.MaxFeedBytes}} {
		if _, err := sanitizeFragment(context.Background(), tc.raw, "https://example.org", tc.budget); err == nil {
			t.Fatalf("budget accepted %d bytes", len(tc.raw))
		}
	}
	if clean, err := sanitizeFragment(context.Background(), "&", "", 5); err != nil || clean != "&amp;" {
		t.Fatalf("exact budget: %q %v", clean, err)
	}
}

func TestGeneratorSanitizesEveryOutputFormat(t *testing.T) {
	data := define.BodyParsed{Body: []define.InfoItem{{Title: "Title", Link: "javascript:alert(1)", Description: `<p onclick="x">Summary<script>bad</script></p>`, Content: `<strong>Article</strong><img src="https://tracker" alt="photo"><a href="data:x">link</a>`}}}
	config := define.JavaScriptConfig{Name: "Feed", URL: "https://example.org/"}
	for _, format := range []string{"rss", "atom", "json"} {
		t.Run(format, func(t *testing.T) {
			body, err := GenerateFeedsByTypeContext(t.Context(), config, data, format)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(body, "javascript:") || strings.Contains(body, "https://tracker") || strings.Contains(body, "onclick") || strings.Contains(body, "<script>") {
				t.Fatalf("unsafe feed: %s", body)
			}
			if format == "json" {
				var decoded struct {
					Items []struct {
						Content string `json:"content_html"`
					}
				}
				if err := json.Unmarshal([]byte(body), &decoded); err != nil {
					t.Fatal(err)
				}
				assertSafeFragment(t, decoded.Items[0].Content)
			} else {
				var envelope struct{ XMLName xml.Name }
				if err := xml.Unmarshal([]byte(body), &envelope); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestJSONFeedSummaryRemainsPlainText(t *testing.T) {
	data := define.BodyParsed{Body: []define.InfoItem{{Description: `<p>Tom &amp; Jerry's summary</p>`}}}
	body, err := GenerateFeedsByTypeContext(t.Context(), define.JavaScriptConfig{}, data, "json")
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Items []struct {
			Summary string `json:"summary"`
		}
	}
	if err = json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Items[0].Summary != "Tom & Jerry's summary" {
		t.Fatalf("summary=%q", decoded.Items[0].Summary)
	}
}

func FuzzFeedHTMLSanitizer(f *testing.F) {
	for _, raw := range []string{`<p>Hello</p>`, `<img src=x onerror=x>`, `<a href="javascript:alert(1)">text</a>`, `<svg><style><img src=x></style></svg>`} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 64<<10 {
			return
		}
		clean, err := sanitizeFragment(t.Context(), raw, "https://example.org/", limits.MaxFeedBytes)
		if err != nil {
			return
		}
		assertSafeFragment(t, clean)
		again, err := sanitizeFragment(t.Context(), clean, "https://example.org/", limits.MaxFeedBytes)
		if err != nil || again != clean {
			t.Fatalf("not stable: %q -> %q (%v)", clean, again, err)
		}
	})
}

func BenchmarkFeedHTMLSanitizer(b *testing.B) {
	raw := strings.Repeat(`<p>Article <strong>text</strong> <a href="/post">link</a></p>`, 100)
	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	for range b.N {
		if _, err := sanitizeFragment(b.Context(), raw, "https://example.org/", limits.MaxFeedBytes); err != nil {
			b.Fatal(err)
		}
	}
}
