// Package browsertest coordinates tests that share a remote Chrome instance.
// It is imported only by test files; it does not configure production browsers.
package browsertest

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Lock serializes browser fixtures across Go test package processes. Register
// this before browser cleanup so testing's LIFO cleanup releases it last.
func Lock(t testing.TB, endpoint string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	release, err := acquire(ctx, lockPath(endpoint))
	if err != nil {
		t.Fatalf("acquire shared browser fixture: %v", err)
	}
	t.Cleanup(release)
}

func lockPath(endpoint string) string {
	digest := sha256.Sum256([]byte(strings.TrimRight(endpoint, "/")))
	return filepath.Join(os.TempDir(), fmt.Sprintf("rss-can-browser-test-%x.lock", digest))
}

func acquire(ctx context.Context, path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			file.Close()
			return nil, err
		}
		err = unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			var once sync.Once
			return func() { once.Do(func() { _ = unix.Flock(int(file.Fd()), unix.LOCK_UN); _ = file.Close() }) }, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			file.Close()
			return nil, err
		}
		timer := time.NewTimer(20 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			file.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	// Keep the lock file: unlinking would permit waiters to lock a different inode.
}
