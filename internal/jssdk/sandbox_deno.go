//go:build !v8go

package jssdk

import (
	"bufio"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
)

//go:embed deno_runner.js
var denoRunner string

var defaultDenoWorker denoWorker

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
	mu      sync.Mutex // One in-flight request; callers wait here without consuming execution time.
	process *denoProcess
	nextID  uint64
}

type denoResponse struct {
	ID     string `json:"id"`
	Result string `json:"result"`
	Error  string `json:"error"`
}

func startDenoProcess() (*denoProcess, error) {
	binary := os.Getenv("RSS_CAN_DENO")
	if binary == "" {
		binary = "deno"
	}
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

func (w *denoWorker) close() { w.mu.Lock(); defer w.mu.Unlock(); w.stopLocked() }

func (w *denoWorker) run(base, source, filename string, timeout time.Duration) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.process != nil {
		select {
		case <-w.process.done:
			w.stopLocked()
		default:
		}
	}
	if w.process == nil {
		p, err := startDenoProcess()
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

// RunCode serializes calls through one lazy, persistent Deno process. Each
// request uses a fresh JS context. A failed process is replaced on the next call.
func RunCode(base, source, originFile string) (string, error) {
	return defaultDenoWorker.run(base, source, originFile, time.Duration(define.JS_EXECUTE_TIMEOUT)*time.Millisecond)
}

// CloseRuntime releases the worker after active execution finishes. A later
// RunCode call may start it again; callers should drain requests before shutdown.
func CloseRuntime() { defaultDenoWorker.close() }

// InitializeRuntime starts the worker and checks execution before serving traffic.
func InitializeRuntime() error {
	_, err := RunCode("", "0", "runtime-init.js")
	return err
}
