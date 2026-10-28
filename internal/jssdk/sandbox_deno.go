package jssdk

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/telemetry"
)

//go:embed deno_runner.js
var denoRunner string

var defaultDenoPool denoPool

type denoProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	reader *bufio.Reader
	done   chan struct{}
}

// stop closes all descriptors, kills the process and joins Wait before reuse.
func (p *denoProcess) stop() {
	_ = p.cmd.Process.Kill()
	_ = p.stdin.Close()
	_ = p.stdout.Close()
	<-p.done
}

type denoWorker struct {
	once    sync.Once
	slot    chan struct{} // One in-flight request; queueing respects the caller's context.
	process *denoProcess
	nextID  uint64
}

type denoResponse struct {
	ID     string `json:"id"`
	Result string `json:"result"`
	Error  string `json:"error"`
}

func startDenoProcess(ctx context.Context) (*denoProcess, error) {
	telemetry.Default.Add("js_starts_total", 1)
	binary := denoExecutable()
	cmd := exec.Command(binary, "run", "--no-prompt", "--no-config", "--no-lock", "data:application/javascript;base64,"+base64.StdEncoding.EncodeToString([]byte(denoRunner)))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	// Own the stdout pipe so background Wait cannot close it before a response
	// has been read. Wait reaps unexpected exits even when the worker is idle.
	stdout, writer, err := os.Pipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	cmd.Stdout = writer
	if err = cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = writer.Close()
		return nil, fmt.Errorf("start Deno: %w", err)
	}
	_ = writer.Close()
	p := &denoProcess{cmd: cmd, stdin: stdin, stdout: stdout, reader: bufio.NewReader(stdout), done: make(chan struct{})}
	go func() { _ = cmd.Wait(); close(p.done) }()
	ready := make(chan error, 1)
	go func() {
		var message struct {
			Ready bool `json:"ready"`
		}
		line, err := p.reader.ReadBytes('\n')
		if err == nil {
			err = json.Unmarshal(line, &message)
			if err == nil && !message.Ready {
				err = fmt.Errorf("invalid Deno handshake")
			}
		}
		ready <- err
	}()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		p.stop()
		<-ready
		return nil, ctx.Err()
	case err := <-ready:
		if err != nil {
			p.stop()
			return nil, fmt.Errorf("Deno startup failed: %w", err)
		}
	case <-timer.C:
		p.stop()
		<-ready
		return nil, fmt.Errorf("Deno startup timed out")
	}
	return p, nil
}

func (w *denoWorker) stopLocked() {
	if w.process != nil {
		w.process.stop()
		w.process = nil
	}
}

func (w *denoWorker) acquire(ctx context.Context) error {
	w.once.Do(func() { w.slot = make(chan struct{}, 1) })
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case w.slot <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-w.slot
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *denoWorker) close() {
	_ = w.acquire(context.Background())
	defer func() { <-w.slot }()
	w.stopLocked()
}

func (w *denoWorker) run(base, source, filename string, timeout time.Duration) (string, error) {
	return w.runContext(context.Background(), base, source, filename, timeout)
}

func (w *denoWorker) runContext(ctx context.Context, base, source, filename string, timeout time.Duration) (value string, err error) {
	return runDeno(ctx, base, source, filename, timeout, func(ctx context.Context) (*denoWorker, func(), error) {
		if err := w.acquire(ctx); err != nil {
			return nil, nil, err
		}
		return w, func() { <-w.slot }, nil
	})
}

func runDeno(ctx context.Context, base, source, filename string, timeout time.Duration, acquire func(context.Context) (*denoWorker, func(), error)) (value string, err error) {
	telemetry.Default.Add("js_requests_total", 1)
	defer func() {
		if err != nil {
			telemetry.Default.Add("js_failures_total", 1)
		}
		if ctx.Err() != nil {
			telemetry.Default.Add("js_cancellations_total", 1)
		}
	}()
	began := time.Now()
	telemetry.Default.Add("js_queued", 1)
	w, release, acquireErr := acquire(ctx)
	err = acquireErr
	telemetry.Default.Add("js_queued", -1)
	telemetry.Default.Observe("js_queue_seconds", time.Since(began))
	if err != nil {
		return "", err
	}
	defer release()
	telemetry.Default.Add("js_active", 1)
	began = time.Now()
	defer func() {
		telemetry.Default.Add("js_active", -1)
		telemetry.Default.Observe("js_execution_seconds", time.Since(began))
	}()
	return w.executeContext(ctx, base, source, filename, timeout)
}

func (w *denoWorker) executeContext(ctx context.Context, base, source, filename string, timeout time.Duration) (string, error) {
	if w.process != nil {
		select {
		case <-w.process.done:
			w.stopLocked()
		default:
		}
	}
	if w.process == nil {
		p, err := startDenoProcess(ctx)
		if err != nil {
			return "", err
		}
		w.process = p
	}
	w.nextID++
	id := strconv.FormatUint(w.nextID, 10)
	request, err := json.Marshal(struct {
		ID       string `json:"id"`
		Base     string `json:"base"`
		Source   string `json:"source"`
		Filename string `json:"filename"`
	}{id, base, source, filename})
	if err != nil {
		return "", err
	}
	p := w.process
	type execution struct {
		response denoResponse
		err      error
	}
	done := make(chan execution, 1)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	go func() {
		var result execution
		if _, result.err = p.stdin.Write(append(request, '\n')); result.err == nil {
			var line []byte
			line, result.err = p.reader.ReadBytes('\n')
			if result.err == nil {
				result.err = json.Unmarshal(line, &result.response)
			}
		}
		done <- result
	}()
	select {
	case <-ctx.Done():
		w.stopLocked()
		<-done
		return "", ctx.Err()
	case result := <-done:
		if result.err != nil {
			w.stopLocked()
			return "", fmt.Errorf("Deno communication failed: %w", result.err)
		}
		if result.response.ID != id {
			w.stopLocked()
			return "", fmt.Errorf("Deno response ID mismatch")
		}
		if result.response.Error != "" {
			return "", fmt.Errorf("JavaScript execution failed: %s", result.response.Error)
		}
		return result.response.Result, nil
	case <-timer.C:
		w.stopLocked()
		<-done // Join the old exchange before any replacement process can be used.
		return "", fmt.Errorf("JavaScript execution timed out: %s", filename)
	}
}

// RunCode uses the bounded persistent Deno pool. Each request receives a fresh
// JS context. A failed process is replaced on its next call.
func RunCode(base, source, originFile string) (string, error) {
	return RunCodeContext(context.Background(), base, source, originFile)
}

func RunCodeContext(ctx context.Context, base, source, originFile string) (string, error) {
	return defaultDenoPool.runContext(ctx, base, source, originFile, time.Duration(define.JS_EXECUTE_TIMEOUT)*time.Millisecond)
}

// CloseRuntime releases all workers after active calls finish. A later
// RunCode call may start it again; callers should drain requests before shutdown.
func CloseRuntime() { defaultDenoPool.close() }

// InitializeRuntime validates every worker before serving traffic.
func InitializeRuntime() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return defaultDenoPool.initialize(ctx)
}

// An explicit override wins, followed by a sibling binary for portable archives.
func denoExecutable() string {
	if configured := os.Getenv("RSS_CAN_DENO"); configured != "" {
		return configured
	}
	if executable, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(executable), "deno")
		if info, err := os.Stat(sibling); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return sibling
		}
	}
	return "deno"
}
