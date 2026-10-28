# Deno execution backend

The default build uses a single persistent Deno worker behind
`jssdk.RunCode(base, source, originFile) (string, error)`. No build tag is needed.
Install Deno 2.9.8 and ensure it is on PATH, then build and test:

```sh
CGO_ENABLED=0 go build -o rssc .
CGO_ENABLED=0 go test ./...
# The race detector itself requires CGO.
go test -race ./internal/jssdk ./internal/rule
```

Docker images include Deno 2.9.8. Standalone archives require a separately
installed Deno (or RSS_CAN_DENO pointing to its executable). The service checks
runtime startup/execution before serving traffic and fails immediately if unavailable.

Deno is the sole execution backend. The legacy v8go backend, its low-level
context helpers, and its Go module dependency have been removed.

Deno is resolved from PATH, or from `RSS_CAN_DENO`, when the worker starts.
There is no automatic fallback when Deno is missing or fails.

## Lifecycle and protocol

The process starts lazily on the first request and stays alive. Go sends
newline-delimited JSON requests on stdin, each with a string ID. Deno sends an
initial ready message, then a matching JSON result/error per request on stdout.
Messages support embedded newlines and Unicode; protocol writes handle partial
pipe writes. Rule console output stays inside the VM.

Calls serialize behind a mutex: one execution and one Deno process at a time.
Waiting callers block rather than starting more processes. Ordering among
concurrent callers follows mutex scheduling; there is no explicit FIFO or
bounded queue, and the existing RunCode API has no cancellation parameter.

Every request gets a fresh V8 Context. The SDK and rule execute as separate
classic scripts in that context, preserving shared declarations and script
completion values. Results use JavaScript String conversion. Requests never
reuse an existing Context or its modified global/prototype objects.

Startup has a separate 10-second deadline. After acquiring the execution slot
and receiving ready, `JS_EXECUTE_TIMEOUT` covers protocol write/read, both
scripts, and result conversion. Queue waiting and startup do not consume this
execution timeout.

A timeout kills and reaps the process, closes its pipes, and joins its pending
exchange before releasing the slot. The next request (including one already
waiting) creates a replacement. There is no automatic replay of failed scripts.
Normal JS exceptions return an error while keeping the process alive. An idle
process exit is reaped in the background; communication errors or mismatched
IDs discard the process, and the next request starts a fresh one.

`jssdk.CloseRuntime()` drains the active execution and releases the worker; the
main program calls it on normal shutdown.
Drain application requests first: a later RunCode can start the worker again.
If the Go process exits without normal cleanup, closing stdin lets the idle
runner exit; normal shutdown explicitly kills and joins the child.

## Limits and verification

Deno runs without granted I/O permissions; prompts, project config and lockfile
loading are disabled. node:vm separates contexts but is not a security boundary.
There is no hard memory limit or periodic worker recycling. Hostile rule
execution still needs OS/container resource and permission isolation.

Tests cover 16 concurrent queued requests, response matching, fresh global and
prototype state, startup failure/recovery, JS exceptions, timeout/replacement,
unexpected process exit, shutdown/restart, large Unicode protocol messages,
base/export timeouts, and existing date helpers. All seven repository rules
must match the committed v8go canonical-config hashes:

```sh
CGO_ENABLED=0 go test -count=1 -v ./internal/jssdk
```

Reproducible latency and RSS benchmarks live in `scripts/bench-jssdk/` and
`docs/benchmarks/`. The original per-call baseline is retained in the benchmark
report and raw data; the production backend now uses the persistent worker.

See [single-worker measurements](benchmarks/deno-worker.md) for latency and
3-minute sustained RSS comparisons with v8go and the original prototype.
