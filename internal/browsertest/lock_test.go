package browsertest

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestLockSubprocess(t *testing.T) {
	path := os.Getenv("RSS_BROWSER_LOCK_CHILD")
	if path == "" {
		return
	}
	release, err := acquire(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	fmt.Println("LOCKED")
	if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
		t.Fatal(err)
	}
}

func TestLockCoordinatesProcessesAndCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "browser.lock")
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLockSubprocess$")
	cmd.Env = append(os.Environ(), "RSS_BROWSER_LOCK_CHILD="+path)
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = input.Close(); _ = cmd.Process.Kill() })
	scanner := bufio.NewScanner(output)
	if !scanner.Scan() || scanner.Text() != "LOCKED" {
		t.Fatal("child did not acquire lock")
	}
	waiting, stop := context.WithTimeout(ctx, 40*time.Millisecond)
	defer stop()
	// The child signaled ownership through its pipe, so a timed acquisition
	// must expire while that process retains the lock.
	if release, err := acquire(waiting, path); !errors.Is(err, context.DeadlineExceeded) {
		if release != nil {
			release()
		}
		t.Fatalf("child-owned acquisition: %v", err)
	}
	canceled, cancelWait := context.WithCancel(ctx)
	cancelWait()
	if release, err := acquire(canceled, path); !errors.Is(err, context.Canceled) {
		if release != nil {
			release()
		}
		t.Fatalf("canceled acquisition: %v", err)
	}
	if _, err := fmt.Fprintln(input, "release"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	release, err := acquire(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	release()
	release()
}

func TestLockWaitsUntilReleased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "browser.lock")
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	release, err := acquire(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	// Same-process flock descriptors have the same exclusion semantics as
	// separate test binaries. A deadline must interrupt actual contention.
	blocked, stop := context.WithTimeout(ctx, 40*time.Millisecond)
	defer stop()
	if other, err := acquire(blocked, path); !errors.Is(err, context.DeadlineExceeded) {
		if other != nil {
			other()
		}
		t.Fatalf("contended acquisition: %v", err)
	}
	release()
	other, err := acquire(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	other()
}
func TestLockEndpointKeyAndCleanup(t *testing.T) {
	if lockPath("http://localhost:1/") != lockPath("http://localhost:1") {
		t.Fatal("equivalent endpoints use different locks")
	}
	if lockPath("http://localhost:1") == lockPath("http://localhost:2") {
		t.Fatal("independent browsers share a lock")
	}
	endpoint := "test://" + t.TempDir()
	t.Run("owner", func(t *testing.T) { Lock(t, endpoint) })
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	release, err := acquire(ctx, lockPath(endpoint))
	if err != nil {
		t.Fatal("test cleanup failed", err)
	}
	release()
}
