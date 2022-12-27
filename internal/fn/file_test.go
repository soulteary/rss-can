package fn_test

import (
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
