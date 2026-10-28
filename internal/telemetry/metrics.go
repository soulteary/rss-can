// Package telemetry exposes bounded, process-local metrics without URL or rule labels.
package telemetry

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var bounds = []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

var counters = []string{
	"feed_requests_total", "feed_successes_total", "feed_failures_total", "feed_timeouts_total",
	"fetch_requests_total", "fetch_failures_total", "fetch_shared_total",
	"cache_hits_total", "cache_misses_total", "cache_errors_total", "cache_fallbacks_total",
	"cache_memory_evictions_total", "cache_memory_rejections_total",
	"js_requests_total", "js_failures_total", "js_cancellations_total", "js_starts_total",
	"config_cache_hits_total", "config_cache_misses_total", "config_shared_total",
	"browser_requests_total", "browser_failures_total", "browser_shared_total",
}
var gauges = []string{"js_queued", "js_active", "browser_queued", "browser_active"}
var histograms = []string{"feed_duration_seconds", "fetch_duration_seconds", "js_queue_seconds", "js_execution_seconds", "browser_queue_seconds", "browser_duration_seconds"}

type metric struct {
	kind    string
	value   float64
	count   uint64
	buckets []uint64
}

type Registry struct {
	mu     sync.Mutex
	values map[string]*metric
}

func New() *Registry {
	r := &Registry{values: make(map[string]*metric)}
	for _, name := range counters {
		r.values[name] = &metric{kind: "counter"}
	}
	for _, name := range gauges {
		r.values[name] = &metric{kind: "gauge"}
	}
	for _, name := range histograms {
		r.values[name] = &metric{kind: "histogram", buckets: make([]uint64, len(bounds))}
	}
	return r
}

var Default = New()

func (r *Registry) Add(name string, delta float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.values[name]
	if m == nil || m.kind == "histogram" {
		panic("unknown numeric metric: " + name)
	}
	m.value += delta
}

func (r *Registry) Observe(name string, duration time.Duration) {
	value := duration.Seconds()
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.values[name]
	if m == nil || m.kind != "histogram" {
		panic("unknown histogram: " + name)
	}
	m.value += value
	m.count++
	for i, bound := range bounds {
		if value <= bound {
			m.buckets[i]++
		}
	}
}

// Value is a synchronized snapshot; histogram values return the sum in seconds.
func (r *Registry) Value(name string) float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m := r.values[name]; m != nil {
		return m.value
	}
	return 0
}

func (r *Registry) Prometheus() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.values))
	for name := range r.values {
		names = append(names, name)
	}
	sort.Strings(names)
	var out strings.Builder
	for _, name := range names {
		m := r.values[name]
		full := "rss_can_" + name
		fmt.Fprintf(&out, "# TYPE %s %s\n", full, m.kind)
		if m.kind != "histogram" {
			fmt.Fprintf(&out, "%s %g\n", full, m.value)
			continue
		}
		for i, bound := range bounds {
			fmt.Fprintf(&out, "%s_bucket{le=%q} %d\n", full, fmt.Sprint(bound), m.buckets[i])
		}
		fmt.Fprintf(&out, "%s_bucket{le=\"+Inf\"} %d\n%s_sum %g\n%s_count %d\n", full, m.count, full, m.value, full, m.count)
	}
	return out.String()
}
