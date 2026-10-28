package jssdk

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestDenoPoolParallelExecutionCancellationAndRecovery(t *testing.T) {
	t.Setenv("RSS_JS_WORKERS", "2")
	var pool denoPool
	defer pool.close()
	if err := pool.initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Warm both workers before checking whether an occupied process blocks another.
	blocked, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := pool.runContext(blocked, "", "while(true) {}", "busy.js", 5*time.Second)
		done <- err
	}()
	defer cancel()
	until := time.Now().Add(time.Second)
	for len(pool.available) != 1 {
		if time.Now().After(until) {
			t.Fatal("busy worker was not acquired")
		}
		time.Sleep(time.Millisecond)
	}
	ctx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	value, err := pool.runContext(ctx, "", "typeof leaked", "fast.js", time.Second)
	if err != nil || value != "undefined" {
		t.Fatalf("healthy worker blocked: %q %v", value, err)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	// Both slots return, and the canceled worker is replaced lazily.
	for range 2 {
		value, err = pool.runContext(context.Background(), "", "42", "recovered.js", time.Second)
		if err != nil || value != "42" {
			t.Fatalf("recovery: %q %v", value, err)
		}
	}
	if len(pool.available) != 2 {
		t.Fatal("pool slot leaked")
	}
	workers := pool.workers
	pool.close()
	for _, w := range workers {
		if w.process != nil {
			t.Fatal("close retained process")
		}
	}
	if value, err := pool.runContext(context.Background(), "", "7", "restart.js", time.Second); err != nil || value != "7" {
		t.Fatalf("restart: %q %v", value, err)
	}
}

func TestDenoPoolQueueDeadlineAndConfiguration(t *testing.T) {
	t.Setenv("RSS_JS_WORKERS", "2")
	var pool denoPool
	defer pool.close()
	if err := pool.initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	first, second := <-pool.available, <-pool.available
	firstPID, secondPID := first.process.cmd.Process.Pid, second.process.cmd.Process.Pid
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := pool.runContext(ctx, "", "0", "queued.js", time.Second)
	pool.available <- first
	pool.available <- second
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("queue: %v", err)
	}
	if first.process.cmd.Process.Pid != firstPID || second.process.cmd.Process.Pid != secondPID {
		t.Fatal("queued cancellation killed healthy worker")
	}
	t.Setenv("RSS_JS_WORKERS", "16")
	if err := pool.configure(); err != nil || len(pool.workers) != 2 {
		t.Fatal("live configuration changed")
	}
	for _, raw := range []string{"0", "17", "-1", "invalid"} {
		t.Setenv("RSS_JS_WORKERS", raw)
		var invalid denoPool
		if _, err := invalid.runContext(context.Background(), "", "0", "invalid.js", time.Second); err == nil {
			t.Fatalf("accepted %q", raw)
		}
		invalid.close()
	}
}

func TestDenoPoolInitializationFailure(t *testing.T) {
	t.Setenv("RSS_JS_WORKERS", "2")
	t.Setenv("RSS_CAN_DENO", filepath.Join(t.TempDir(), "missing"))
	var pool denoPool
	defer pool.close()
	if err := pool.initialize(context.Background()); err == nil {
		t.Fatal("missing Deno passed startup")
	}
	for _, w := range pool.workers {
		if w.process != nil {
			t.Fatal("startup failure retained process")
		}
	}
}
