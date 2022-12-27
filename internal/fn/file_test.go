package fn_test

import (
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/fn"
)

func TestScanDirFiles(t *testing.T) {
	files := fn.ScanDirFiles("./")
	if len(files) == 0 {
		t.Fatal("ScanDirFiles failed")
	}

	files = fn.ScanDirFiles("./not-exists/")
	if len(files) > 0 {
		t.Fatal("ScanDirFiles failed")
	}
}

func TestIsDir(t *testing.T) {
	files := fn.ScanDirFiles("./")

	hasCheckDir := false
	for _, file := range files {
		if fn.IsDir(file) {
			hasCheckDir = true
		} else if fn.IsDir(file) && strings.HasSuffix(file, ".go") {
			t.Fatal("IsDir failed")
		}
	}

	if !hasCheckDir {
		files = fn.ScanDirFiles("../")
		for _, file := range files {
			if fn.IsDir(file) {
				hasCheckDir = true
			} else if fn.IsDir(file) && strings.HasSuffix(file, ".go") {
				t.Fatal("IsDir failed")
			}
		}
	}

	ret := fn.IsDir("/not-exist-directory-maybe-long-")
	if ret {
		t.Fatal("IsDir failed")
	}
}

func TestIsFile(t *testing.T) {
	files := fn.ScanDirFiles("./")

	for _, file := range files {
		if fn.IsFile(file) && fn.IsDir(file) {
			t.Fatalf("IsFile failed")
		}
	}

	ret := fn.IsFile("/not-exist-directory-maybe-long-")
	if ret {
		t.Fatal("IsFile failed")
	}
}
