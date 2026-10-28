package jssdk

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runtimeTestBinary(t *testing.T) string {
	t.Helper()
	binary := os.Getenv("RSS_CAN_DENO")
	if binary == "" {
		binary = denoExecutable()
	}
	return binary
}

func TestOwnedRuntimeRejectsInvalidConfiguration(t *testing.T) {
	for _, config := range []RuntimeConfig{
		{Workers: 0, Timeout: time.Second},
		{Workers: 17, Timeout: time.Second},
		{Workers: 1, Timeout: 0},
		{Workers: 1, Timeout: -time.Second},
		{Binary: filepath.Join(t.TempDir(), "missing-deno"), Workers: 1, Timeout: time.Second},
	} {
		runtime, err := NewRuntime(t.Context(), config)
		if err == nil || runtime != nil {
			if runtime != nil {
				runtime.Close()
			}
			t.Fatalf("invalid config succeeded: %+v, %v", config, err)
		}
	}
	// Finding an executable is insufficient: startup must validate the protocol.
	runtime, err := NewRuntime(t.Context(), RuntimeConfig{Binary: "false", Workers: 1, Timeout: time.Second})
	if err == nil || runtime != nil {
		if runtime != nil {
			runtime.Close()
		}
		t.Fatalf("invalid runtime handshake accepted: %v", err)
	}
}

func TestOwnedRuntimeCapturesBinaryAndWorkerConfiguration(t *testing.T) {
	binary := runtimeTestBinary(t)
	t.Setenv("RSS_CAN_DENO", binary)
	first, err := NewRuntime(t.Context(), RuntimeConfig{Workers: 1, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	// Environment changes must not alter an existing owner, nor the explicit
	// configuration of a second independent runtime created afterward.
	t.Setenv("RSS_CAN_DENO", filepath.Join(t.TempDir(), "missing-deno"))
	t.Setenv("RSS_JS_WORKERS", "invalid")
	second, err := NewRuntime(t.Context(), RuntimeConfig{Binary: binary, Workers: 2, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if len(first.pool.workers) != 1 || len(second.pool.workers) != 2 {
		t.Fatal("runtime worker counts inherited changed environment")
	}
	if first.pool.workers[0] == second.pool.workers[0] || first.pool.workers[0].process.cmd.Process.Pid == second.pool.workers[0].process.cmd.Process.Pid {
		t.Fatal("owners share a worker process")
	}
	// Exercise recovery rather than only an already-started executable. This
	// would consult the changed environment if ownership were lost on restart.
	first.pool.workers[0].close()
	if err := first.Check(context.Background()); err != nil {
		t.Fatalf("worker restart lost captured binary: %v", err)
	}
	if err := second.Check(context.Background()); err != nil {
		t.Fatalf("second runtime unavailable: %v", err)
	}
}

func TestOwnedRuntimeCloseAndDateContextIsolation(t *testing.T) {
	binary := runtimeTestBinary(t)
	first, err := NewRuntime(t.Context(), RuntimeConfig{Binary: binary, Workers: 1, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := NewRuntime(t.Context(), RuntimeConfig{Binary: binary, Workers: 1, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	firstContext := WithRuntime(context.Background(), first)
	secondContext := WithRuntime(context.Background(), second)
	t.Setenv("RSS_CAN_DENO", filepath.Join(t.TempDir(), "missing-deno"))
	// Date conversion travels through the public context-aware API and must use
	// the selected owner rather than the global default JavaScript pool.
	date, err := ConvertStrToUnixContext(secondContext, "2026-10-29T00:00:00Z")
	if err != nil || !date.Equal(time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("scoped date conversion: %v %v", date, err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("repeated close: %v", err)
	}
	if _, err := RunCodeContext(firstContext, "", "42", "closed.js"); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("closed owner restarted: %v", err)
	}
	if _, err := ConvertStrToUnixContext(firstContext, "2026-10-29"); err == nil {
		t.Fatal("date conversion escaped a closed owner")
	}
	if err := first.Check(context.Background()); err == nil {
		t.Fatal("closed readiness restarted runtime")
	}
	value, err := RunCodeContext(secondContext, "", "21*2", "open.js")
	if err != nil || value != "42" {
		t.Fatalf("closing first affected second: %q %v", value, err)
	}
}
