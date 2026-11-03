package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/source"
)

func TestSourceWizardBrowserCreatesCredentialWithoutExecutingContent(t *testing.T) {
	endpoint := os.Getenv("RSS_TEST_BROWSER_URL")
	if endpoint == "" {
		if os.Getenv("RSS_TEST_BROWSER_REQUIRED") == "1" {
			t.Fatal("browser endpoint required")
		}
		t.Skip("browser endpoint not configured")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(endpoint, "/")+"/json/version", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var discovery struct {
		URL string `json:"webSocketDebuggerUrl"`
	}
	err = json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&discovery)
	res.Body.Close()
	if err != nil || discovery.URL == "" {
		t.Fatal("browser discovery failed", err)
	}
	app, a, _ := accessApp(t)
	a.Preview = func(context.Context, source.Spec) ([]define.InfoItem, error) {
		return []define.InfoItem{{Title: `<img src=x onerror=window.injected=1>`, Link: "https://example.org/post"}}, nil
	}
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-Auth-User", "alice")
		response, err := app.Test(r, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		if err != nil {
			http.Error(w, "test gateway failed", 500)
			return
		}
		defer response.Body.Close()
		for k, values := range response.Header {
			for _, v := range values {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, response.Body)
	}))
	defer proxy.Close()
	a.Config.PublicOrigin = proxy.URL
	alloc, stop := remote.NewAllocator(ctx, discovery.URL, remote.NoModifyURL)
	defer stop()
	root, stopRoot := chromedp.NewContext(alloc)
	defer stopRoot()
	if err := chromedp.Do(root); err != nil {
		t.Fatal(err)
	}
	page, stopPage := chromedp.NewContext(root, chromedp.WithNewBrowserContext())
	defer stopPage()
	if err := chromedp.Do(page, chromedp.Navigate(proxy.URL+"/sources"), chromedp.WaitVisible(chromedp.CSS("#source-form"))); err != nil {
		t.Fatal(err)
	}
	if _, err = chromedp.Run(page, chromedp.Poll[bool](`!document.getElementById("preview").disabled`)); err != nil {
		t.Fatal(err)
	}
	_, err = chromedp.Run(page, chromedp.Evaluate[bool](`(() => {window.prompt=()=>"Browser test";document.getElementById("url").value="https://example.org/news";document.getElementById("name").value="<img src=x onerror=window.injected=1>";document.getElementById("preview").click();return true})()`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = chromedp.Run(page, chromedp.Poll[bool](`document.querySelector("#preview-result li") !== null`)); err != nil {
		t.Fatal(err)
	}
	if _, err = chromedp.Run(page, chromedp.Poll[bool](`!document.getElementById("save").disabled`)); err != nil {
		t.Fatal(err)
	}
	if _, err = chromedp.Run(page, chromedp.Evaluate[bool](`(() => {document.getElementById("save").click();return true})()`)); err != nil {
		t.Fatal(err)
	}
	if _, err = chromedp.Run(page, chromedp.Poll[bool](`document.getElementById("reader-url").value.includes("/s/")`)); err != nil {
		t.Fatal(err)
	}
	safe, err := chromedp.Run(page, chromedp.Evaluate[bool](`!window.injected && document.querySelector("#sources img") === null && document.querySelector("#preview-result img") === null`))
	if err != nil || !safe {
		t.Fatal("untrusted content executed", err)
	}
	managed := a.Store.ManagedSources()
	if len(managed) != 1 || managed[0].Public || managed[0].Owner != "alice" {
		t.Fatal("browser did not create private source")
	}
	grants := a.Store.Grants("alice")
	if len(grants) != 1 || grants[0].Label != "Browser test" {
		t.Fatal("browser did not issue named reader credential")
	}
}
