package telemetry

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConcurrentMetricsAndHistogram(t *testing.T) {
	r := New()
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			for range 100 {
				r.Add("feed_requests_total", 1)
				r.Observe("feed_duration_seconds", 10*time.Millisecond)
				r.Add("js_active", 1)
				r.Add("js_active", -1)
			}
		})
	}
	wg.Wait()
	if r.Value("feed_requests_total") != 2000 || r.Value("js_active") != 0 || r.Value("unknown") != 0 {
		t.Fatal("lost counter or gauge update")
	}
	out := r.Prometheus()
	for _, want := range []string{"# TYPE rss_can_feed_requests_total counter", "rss_can_feed_requests_total 2000", `rss_can_feed_duration_seconds_bucket{le="0.005"} 0`, `rss_can_feed_duration_seconds_bucket{le="0.01"} 2000`, `rss_can_feed_duration_seconds_bucket{le="+Inf"} 2000`, "rss_can_feed_duration_seconds_count 2000", "rss_can_js_queued 0"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if sum := r.Value("feed_duration_seconds"); sum < 19.99 || sum > 20.01 {
		t.Fatal(sum)
	}
}

func TestMetricsRejectUnregisteredFamilies(t *testing.T) {
	for _, run := range []func(*Registry){func(r *Registry) { r.Add("unknown", 1) }, func(r *Registry) { r.Add("feed_duration_seconds", 1) }, func(r *Registry) { r.Observe("unknown", time.Second) }, func(r *Registry) { r.Observe("feed_requests_total", time.Second) }} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid metric family accepted")
				}
			}()
			run(New())
		}()
	}
}
