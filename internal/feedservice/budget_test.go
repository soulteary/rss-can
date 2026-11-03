package feedservice

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/network"
)

func TestGenerationSharesBudgetAcrossConfigurationAndFetch(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); _, _ = io.WriteString(w, "ok") }))
	defer upstream.Close()
	fetch := func(ctx context.Context) (define.ErrorCode, string) {
		code, status, response := network.HttpGetContext(ctx, upstream.URL, "")
		if response != nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
		}
		return code, status
	}
	deps := Dependencies{
		Lookup: func(string) (define.RuleCache, bool) { return define.RuleCache{}, true },
		Configure: func(ctx context.Context, _ define.RuleCache) (define.JavaScriptConfig, error) {
			for i := 0; i < 32; i++ {
				if code, status := fetch(ctx); code != define.ERROR_CODE_NULL {
					t.Fatalf("early configuration budget: %s", status)
				}
			}
			return define.JavaScriptConfig{}, nil
		},
		Fetch: func(ctx context.Context, _ define.JavaScriptConfig) define.BodyParsed {
			for i := 0; i < 33; i++ {
				if code, status := fetch(ctx); code != define.ERROR_CODE_NULL {
					return define.BodyParsed{Code: code, Status: status}
				}
			}
			return define.BodyParsed{}
		},
		Serialize: func(context.Context, define.JavaScriptConfig, define.BodyParsed, string) (string, error) {
			t.Fatal("over-budget work serialized")
			return "", nil
		},
	}
	_, err := New(5*time.Second, deps).Generate(t.Context(), "source", "rss")
	if err == nil || !strings.Contains(err.Error(), "fetch task budget exceeded") {
		t.Fatalf("budget failure lost: %v", err)
	}
	if calls.Load() != network.MaxTaskRequests {
		t.Fatalf("performed %d upstream requests", calls.Load())
	}
}
