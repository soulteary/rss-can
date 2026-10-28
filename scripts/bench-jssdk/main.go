// bench-jssdk measures the production RunCode boundary, including IPC.
// For a persistent worker, warmup excludes one-time startup.
// Default builds use Deno; -tags v8go selects the legacy backend.
package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/soulteary/RSS-Can/internal/jssdk"
)

type task struct{ base, source, filename string }

func main() {
	workload := flag.String("workload", "rules", "minimal, rules, date, mixed or deno-startup")
	count := flag.Int("n", 100, "measured requests")
	warmup := flag.Int("warmup", 5, "unmeasured requests")
	duration := flag.Duration("duration", 0, "sustained run duration (no per-call latency storage)")
	interval := flag.Duration("interval", 0, "minimum interval between sustained calls")
	idle := flag.Duration("idle", 0, "idle observation period after a sustained run")
	flag.Parse()
	defer jssdk.CloseRuntime()
	if *count < 1 || *warmup < 0 || *duration < 0 || *interval < 0 || *idle < 0 {
		panic("invalid iteration count")
	}
	var tasks []task
	switch *workload {
	case "minimal":
		tasks = []task{{"var value = 42;", "value", "minimal.js"}}
	case "date":
		tasks = []task{{jssdk.TPL_DATE_JS, `ConvertStrToUnix("2026-10-28 12:00:00")`, "date.js"}}
	case "rules", "mixed":
		files, err := filepath.Glob("rules/*/*.js")
		must(err)
		for _, file := range files {
			body, err := os.ReadFile(file)
			must(err)
			tasks = append(tasks, task{jssdk.TPL_SSR_JS, jssdk.GenerateGetConfigWithRule(body), file})
		}
		if len(tasks) == 0 {
			panic("run from repository root: no rules found")
		}
	case "deno-startup":
	default:
		panic("unknown workload")
	}
	if *workload == "mixed" {
		tasks = append(tasks, task{jssdk.TPL_DATE_JS, `ConvertStrToUnix("2026-10-28 12:00:00")`, "date.js"})
	}
	var startupCode string
	if *workload == "deno-startup" {
		body, err := os.ReadFile("internal/jssdk/deno_runner.js")
		must(err)
		startupCode = "data:application/javascript;base64," + base64.StdEncoding.EncodeToString(body)
	}
	run := func(i int) {
		if *workload == "deno-startup" {
			// Match production flags and runner; measure ready handshake plus teardown,
			// without sending any SDK or rule. stdin stays open until cancellation.
			binary := os.Getenv("RSS_CAN_DENO")
			if binary == "" {
				binary = "deno"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			cmd := exec.CommandContext(ctx, binary, "run", "--no-prompt", "--no-config", "--no-lock", startupCode)
			stdin, err := cmd.StdinPipe()
			must(err)
			stdout, err := cmd.StdoutPipe()
			must(err)
			must(cmd.Start())
			line, err := bufio.NewReader(stdout).ReadString('\n')
			cancel()
			_ = stdin.Close()
			_ = cmd.Wait()
			must(err)
			if !strings.Contains(line, `"ready":true`) {
				panic(line)
			}
			return
		}
		t := tasks[i%len(tasks)]
		value, err := jssdk.RunCode(t.base, t.source, t.filename)
		must(err)
		if *workload == "minimal" && value != "42" {
			panic(value)
		}
		if *workload == "rules" && !json.Valid([]byte(value)) {
			panic("invalid config JSON")
		}
	}
	for i := 0; i < *warmup; i++ {
		run(i)
	}
	if *duration > 0 {
		start := time.Now()
		completed := 0
		var elapsed time.Duration
		for time.Since(start) < *duration {
			began := time.Now()
			run(completed)
			elapsed += time.Since(began)
			completed++
			if wait := *interval - time.Since(began); wait > 0 {
				time.Sleep(wait)
			}
		}
		activeWall := time.Since(start).Seconds()
		time.Sleep(*idle)
		must(json.NewEncoder(os.Stdout).Encode(map[string]any{"workload": *workload, "n": completed, "wall_s": activeWall, "idle_s": idle.Seconds(), "warmup": *warmup, "interval_ms": float64(*interval) / float64(time.Millisecond), "mean_execution_ms": float64(elapsed) / float64(time.Millisecond) / float64(completed)}))
		return
	}
	samples := make([]float64, *count)
	start := time.Now()
	for i := range samples {
		t := time.Now()
		run(i)
		samples[i] = float64(time.Since(t).Nanoseconds()) / 1e6
	}
	wall := time.Since(start).Seconds()
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	quantile := func(q float64) float64 { return sorted[int(math.Ceil(q*float64(len(sorted))))-1] }
	var total float64
	for _, v := range samples {
		total += v
	}
	jssdk.CloseRuntime() // Reap persistent child before collecting child rusage.
	var self, children syscall.Rusage
	must(syscall.Getrusage(syscall.RUSAGE_SELF, &self))
	must(syscall.Getrusage(syscall.RUSAGE_CHILDREN, &children))
	scale := 1024.0
	if runtime.GOOS == "darwin" {
		scale = 1024 * 1024
	}
	must(json.NewEncoder(os.Stdout).Encode(map[string]any{
		"workload": *workload, "n": *count, "warmup": *warmup, "wall_s": wall, "mean_ms": total / float64(*count), "p50_ms": quantile(.5), "p95_ms": quantile(.95), "requests_per_s": float64(*count) / wall,
		"self_peak_rss_mib": float64(self.Maxrss) / scale, "child_peak_rss_mib": float64(children.Maxrss) / scale, "latencies_ms": samples,
	}))
}
func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
