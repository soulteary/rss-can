//go:build darwin || linux

package rulefile

import (
	"context"
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadOwnsOnlyBoundedRegularFiles(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "feed.js")
	if err := os.WriteFile(path, []byte("normal"), 0600); err != nil {
		t.Fatal(err)
	}
	body, err := Read(t.Context(), path)
	if err != nil || string(body) != "normal" {
		t.Fatalf("%q %v", body, err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", MaxBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(t.Context(), path); err == nil {
		t.Fatal("oversize rule accepted")
	}
	if _, err := Read(t.Context(), path+"missing"); err == nil {
		t.Fatal("missing rule accepted")
	}
	if _, err := Read(t.Context(), filepath.Join(path, "child.js")); err == nil {
		t.Fatal("file parent accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Read(ctx, path); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	dir, err := OpenDirectory(directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dir.ReadFile(ctx, "feed.js", MaxBytes); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := dir.OpenDirectory("missing"); err == nil {
		t.Fatal("missing group accepted")
	}
	dir.Close()
	if _, err := dir.ReadFile(t.Context(), "feed.js", MaxBytes); err == nil {
		t.Fatal("closed directory reused")
	}
}

func TestReadRejectsFIFOAndSymlinksWithoutWaiting(t *testing.T) {
	root := t.TempDir()
	fifo := filepath.Join(root, "blocked.js")
	if err := unix.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := Read(t.Context(), fifo); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("FIFO open blocked")
	}
	target := filepath.Join(root, "target.js")
	if err := os.WriteFile(target, []byte("body"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.js")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(t.Context(), link); err == nil {
		t.Fatal("symlink rule followed")
	}
	dirlink := filepath.Join(t.TempDir(), "group")
	if err := os.Symlink(root, dirlink); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(t.Context(), filepath.Join(dirlink, "target.js")); err == nil {
		t.Fatal("symlink group followed")
	}
	if _, err := Read(t.Context(), root); err == nil {
		t.Fatal("directory rule accepted")
	}
}

func TestOpenedDirectoryStaysAnchoredAfterReplacement(t *testing.T) {
	root := t.TempDir()
	groupPath := filepath.Join(root, "group")
	if err := os.Mkdir(groupPath, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(groupPath, "feed.js"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	directory, err := OpenDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	entries, err := directory.ReadDir(64)
	if err != nil && len(entries) == 0 {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "group" {
		t.Fatalf("unexpected directory entries: %v", entries)
	}
	group, err := directory.OpenDirectory("group")
	if err != nil {
		t.Fatal(err)
	}
	defer group.Close()
	if err := os.Rename(groupPath, groupPath+"-old"); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "feed.js"), []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, groupPath); err != nil {
		t.Fatal(err)
	}
	body, err := group.ReadFile(t.Context(), "feed.js", MaxBytes)
	if err != nil || string(body) != "original" {
		t.Fatalf("anchored descriptor followed replacement: %q %v", body, err)
	}
	if _, err := directory.OpenDirectory("group"); err == nil {
		t.Fatal("new open followed replaced group symlink")
	}
}

func TestDirectorySymlinkTrailingSlashCannotBypassNoFollow(t *testing.T) {
	target := t.TempDir()
	root := t.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{link, link + string(os.PathSeparator), link + string(os.PathSeparator) + "."} {
		directory, err := OpenDirectory(path)
		if err == nil {
			directory.Close()
			t.Fatalf("followed symlink directory %q", path)
		}
	}
	directory, err := OpenDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	for _, name := range []string{"link/", "link/.", "link/child", "..", ".", ""} {
		if group, err := directory.OpenDirectory(name); err == nil {
			group.Close()
			t.Fatalf("group accepted multi-component name %q", name)
		}
		if _, err := directory.ReadFile(t.Context(), name, MaxBytes); err == nil {
			t.Fatalf("file accepted multi-component name %q", name)
		}
	}
}
