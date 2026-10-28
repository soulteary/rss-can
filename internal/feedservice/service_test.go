package feedservice

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
)

func fixture(calls *[]string) Dependencies {
	return Dependencies{
		Lookup: func(id string) (define.RuleCache, bool) {
			*calls = append(*calls, "lookup:"+id)
			return define.RuleCache{File: "snapshot.js"}, true
		},
		Configure: func(ctx context.Context, r define.RuleCache) (define.JavaScriptConfig, error) {
			*calls = append(*calls, "configure:"+r.File)
			return define.JavaScriptConfig{Name: "snapshot"}, nil
		},
		Fetch: func(ctx context.Context, c define.JavaScriptConfig) define.BodyParsed {
			*calls = append(*calls, "fetch:"+c.Name)
			return define.BodyParsed{}
		},
		Serialize: func(ctx context.Context, c define.JavaScriptConfig, d define.BodyParsed, f string) (string, error) {
			*calls = append(*calls, "serialize:"+f)
			return "feed", nil
		},
	}
}
func TestGenerateOrderAndFormat(t *testing.T) {
	var calls []string
	body, err := New(time.Second, fixture(&calls)).Generate(context.Background(), "site", "RSS")
	if err != nil || body != "feed" || !reflect.DeepEqual(calls, []string{"lookup:site", "configure:snapshot.js", "fetch:snapshot", "serialize:rss"}) {
		t.Fatalf("%q %v %v", body, err, calls)
	}
}
func TestGenerateStopsAtFailedBoundary(t *testing.T) {
	cause := errors.New("dependency failed")
	for _, stage := range []Stage{InvalidFormat, MissingRule, Configuration, Fetch, Serialization} {
		t.Run(string(stage), func(t *testing.T) {
			var calls []string
			d := fixture(&calls)
			format := "rss"
			wantCalls := 0
			switch stage {
			case InvalidFormat:
				format = "bad"
			case MissingRule:
				d.Lookup = func(string) (define.RuleCache, bool) { return define.RuleCache{}, false }
			case Configuration:
				wantCalls = 1
				d.Configure = func(context.Context, define.RuleCache) (define.JavaScriptConfig, error) {
					return define.JavaScriptConfig{}, cause
				}
			case Fetch:
				wantCalls = 2
				d.Fetch = func(context.Context, define.JavaScriptConfig) define.BodyParsed {
					return define.BodyParsed{Code: define.ERROR_CODE_NETWORK, Status: cause.Error()}
				}
			case Serialization:
				wantCalls = 3
				d.Serialize = func(context.Context, define.JavaScriptConfig, define.BodyParsed, string) (string, error) {
					return "partial", cause
				}
			}
			body, err := New(time.Second, d).Generate(context.Background(), "site", format)
			var failure *Error
			if body != "" || !errors.As(err, &failure) || failure.Stage != stage || len(calls) != wantCalls {
				t.Fatalf("body=%q error=%v calls=%v", body, err, calls)
			}
			if stage == Configuration || stage == Serialization {
				if !errors.Is(err, cause) {
					t.Fatal("lost underlying cause")
				}
			}
		})
	}
}
func TestGenerateBudgetAndCancellation(t *testing.T) {
	var calls []string
	d := fixture(&calls)
	var configDeadline time.Time
	d.Configure = func(ctx context.Context, _ define.RuleCache) (define.JavaScriptConfig, error) {
		configDeadline, _ = ctx.Deadline()
		return define.JavaScriptConfig{}, nil
	}
	d.Fetch = func(ctx context.Context, _ define.JavaScriptConfig) define.BodyParsed {
		deadline, _ := ctx.Deadline()
		if deadline != configDeadline {
			t.Error("budget reset between stages")
		}
		<-ctx.Done()
		return define.BodyParsed{Code: define.ERROR_CODE_NETWORK}
	}
	body, err := New(20*time.Millisecond, d).Generate(context.Background(), "site", "rss")
	var failure *Error
	if body != "" || !errors.As(err, &failure) || failure.Stage != Timeout || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("%q %v", body, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls = nil
	_, err = New(time.Second, fixture(&calls)).Generate(ctx, "site", "rss")
	if !errors.Is(err, context.Canceled) || len(calls) != 0 {
		t.Fatalf("%v %v", err, calls)
	}
}
