//go:build !v8go

package jssdk

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func workerValue(t *testing.T, w *denoWorker, source string) string {
	t.Helper()
	value, err := w.run("", source, "test.js", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestDenoWorkerReusesProcessAndRecoversFromScriptError(t *testing.T) {
	var w denoWorker
	defer w.close()
	workerValue(t, &w, "42")
	pid := w.process.cmd.Process.Pid
	if _, err := w.run("", "throw new Error('expected')", "error.js", time.Second); err == nil || (!strings.Contains(err.Error(), "expected") || !strings.Contains(err.Error(), "error.js")) {
		t.Fatalf("script error: %v", err)
	}
	if value := workerValue(t, &w, "42"); value != "42" {
		t.Fatal(value)
	}
	if w.process.cmd.Process.Pid != pid {
		t.Fatal("normal calls or script errors restarted worker")
	}
	// Errors retain filename/stack context and do not contaminate the protocol.
	value, err := w.run("", `console.log("not protocol"); "中文\n\"value\""`, "unicode.js", time.Second)
	if err != nil || value != "中文\n\"value\"" {
		t.Fatalf("protocol: %q %v", value, err)
	}
}

func TestDenoWorkerConcurrentQueueAndIsolation(t *testing.T) {
	var w denoWorker
	defer w.close()
	workerValue(t, &w, "42")
	pid := w.process.cmd.Process.Pid
	const requests = 16
	start := make(chan struct{})
	errors := make(chan error, requests)
	var wg sync.WaitGroup
	began := time.Now()
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			base := fmt.Sprintf(`if (typeof marker !== 'undefined' || typeof ({}).marker !== 'undefined') throw new Error('leaked state'); var marker = %d; Object.prototype.marker = marker; var until = Date.now()+20; while(Date.now()<until) {}`, i)
			value, err := w.run(base, "marker", "concurrent.js", 150*time.Millisecond)
			if err != nil {
				errors <- err
				return
			}
			if value != fmt.Sprint(i) {
				errors <- fmt.Errorf("response crossed requests: %q, want %d", value, i)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if time.Since(began) < 300*time.Millisecond {
		t.Fatal("expected serialized execution")
	}
	if w.process == nil || w.process.cmd.Process.Pid != pid {
		t.Fatal("queue unexpectedly restarted worker")
	}
	// Total queue time exceeds the per-execution deadline, yet every call succeeds.
}

func TestDenoWorkerTimeoutRebuildAndQueuedRecovery(t *testing.T) {
	var w denoWorker
	defer w.close()
	workerValue(t, &w, "42")
	pid := w.process.cmd.Process.Pid
	timeoutResult := make(chan error, 1)
	go func() {
		_, err := w.run("globalThis.leaked=true;", "while(true) {}", "timeout.js", 50*time.Millisecond)
		timeoutResult <- err
	}()
	// Wait until the first caller owns the execution slot before queuing the
	// second caller, rather than assuming a goroutine scheduling delay.
	deadline := time.Now().Add(time.Second)
	for w.mu.TryLock() {
		w.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("timeout request never acquired execution slot")
		}
		time.Sleep(time.Millisecond)
	}
	value, err := w.run("", "typeof leaked", "queued.js", time.Second)
	if err != nil || value != "undefined" {
		t.Fatalf("queued recovery: %q %v", value, err)
	}
	if err := <-timeoutResult; err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("timeout: %v", err)
	}
	if w.process != nil && w.process.cmd.Process.Pid == pid {
		t.Fatal("timeout reused old process")
	}
	workerValue(t, &w, "42")
	if w.process.cmd.Process.Pid == pid {
		t.Fatal("failed to replace timed-out worker")
	}
}

func TestDenoWorkerUnexpectedExitAndClose(t *testing.T) {
	var w denoWorker
	defer w.close()
	workerValue(t, &w, "42")
	old := w.process
	if err := old.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-old.done:
	case <-time.After(time.Second):
		t.Fatal("worker exit was not reaped")
	}
	workerValue(t, &w, "42")
	if w.process.cmd.Process.Pid == old.cmd.Process.Pid {
		t.Fatal("unexpected exit did not restart")
	}
	p := w.process
	w.close()
	select {
	case <-p.done:
	default:
		t.Fatal("close did not join worker")
	}
	if w.process != nil {
		t.Fatal("close retained process")
	}
	workerValue(t, &w, "42")
}

func TestDenoWorkerLargeMessages(t *testing.T) {
	var w denoWorker
	defer w.close()
	// Exercise framing across pipe buffers in both directions, including UTF-8.
	value, err := w.run("var text = '中文'.repeat(50000);", "text", "large.js", time.Second)
	if err != nil || value != strings.Repeat("中文", 50000) {
		t.Fatalf("large response: bytes=%d, error=%v", len(value), err)
	}
	value, err = w.run("/*"+strings.Repeat("中文\n", 50000)+"*/", "42", "large-input.js", time.Second)
	if err != nil || value != "42" {
		t.Fatalf("large input: %q, %v", value, err)
	}
}

func TestDenoWorkerStartupFailureAndRecovery(t *testing.T) {
	binary := "deno"
	if configured := os.Getenv("RSS_CAN_DENO"); configured != "" {
		binary = configured
	}
	var w denoWorker
	defer w.close()
	t.Setenv("RSS_CAN_DENO", filepath.Join(t.TempDir(), "missing-deno"))
	if _, err := w.run("", "42", "missing.js", time.Second); err == nil {
		t.Fatal("expected startup failure")
	}
	if w.process != nil {
		t.Fatal("startup failure retained process")
	}
	t.Setenv("RSS_CAN_DENO", binary)
	if value := workerValue(t, &w, "42"); value != "42" {
		t.Fatal(value)
	}
}
