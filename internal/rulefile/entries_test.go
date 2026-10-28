//go:build darwin || linux

package rulefile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestDirectoryEntriesUseOpenedDescriptorAfterPathReplacement(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "group")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	stamp := time.Unix(1700000000, 0)
	file := filepath.Join(path, "feed.js")
	if err := os.WriteFile(file, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(file, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("feed.js", filepath.Join(path, "link.js")); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(filepath.Join(path, "pipe.js"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(path, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	parent, err := OpenDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	group, err := parent.OpenDirectory("group")
	if err != nil {
		t.Fatal(err)
	}
	defer group.Close()
	if err := os.Rename(path, path+"-old"); err != nil {
		t.Fatal(err)
	}
	replacement := t.TempDir()
	if err := os.WriteFile(filepath.Join(replacement, "feed.js"), []byte("wrong replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(replacement, path); err != nil {
		t.Fatal(err)
	}
	entries, err := group.ReadDir(-1)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("directory entries disappeared: %v", entries)
	}
	expected := map[string]os.FileMode{"feed.js": 0, "link.js": os.ModeSymlink, "pipe.js": os.ModeNamedPipe, "nested": os.ModeDir}
	for _, entry := range entries {
		mode, ok := expected[entry.Name()]
		if !ok {
			t.Fatalf("unexpected entry %s", entry.Name())
		}
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		if entry.Type() != mode || info.Mode().Type() != mode || entry.IsDir() != (mode == os.ModeDir) {
			t.Fatalf("incorrect metadata for %s", entry.Name())
		}
		if info.Name() != entry.Name() || info.Sys() == nil {
			t.Fatal("incomplete file info")
		}
		if entry.Name() == "feed.js" && (info.Size() != 8 || !info.ModTime().Equal(stamp) || info.Mode().Perm() != 0600) {
			t.Fatalf("metadata followed pathname replacement: %+v", info)
		}
	}
	if entries, err := group.ReadDir(1); len(entries) != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("EOF contract: %v %v", entries, err)
	}
}

func TestDirectoryEntryMetadataFailureIsNotSilentlySkipped(t *testing.T) {
	root := t.TempDir()
	directory, err := OpenDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := directoryEntry(directory.file, "missing.js"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing entry: %v", err)
	}
	directory.Close()
	if _, err := directory.ReadDir(1); err == nil {
		t.Fatal("closed descriptor accepted")
	}
}

func TestDirectoryEntrySpecialModes(t *testing.T) {
	for _, tc := range []struct {
		raw  uint32
		want os.FileMode
	}{
		{unix.S_IFSOCK, os.ModeSocket}, {unix.S_IFBLK, os.ModeDevice}, {unix.S_IFCHR, os.ModeDevice | os.ModeCharDevice}, {0, os.ModeIrregular},
	} {
		var stat unix.Stat_t
		// Stat.Mode has different widths across supported platforms.
		switch tc.raw {
		case unix.S_IFSOCK:
			stat.Mode = unix.S_IFSOCK
		case unix.S_IFBLK:
			stat.Mode = unix.S_IFBLK
		case unix.S_IFCHR:
			stat.Mode = unix.S_IFCHR
		}
		stat.Mode |= unix.S_ISUID | unix.S_ISGID | unix.S_ISVTX | 0700
		mode := entryInfo{stat: stat}.Mode()
		if mode.Type() != tc.want || mode.Perm() != 0700 || mode&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != (os.ModeSetuid|os.ModeSetgid|os.ModeSticky) {
			t.Fatalf("mode conversion %v", mode)
		}
	}
}
