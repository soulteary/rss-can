package fn_test

import (
	"testing"

	"github.com/soulteary/RSS-Can/internal/fn"
)

func TestStringToPositiveInteger(t *testing.T) {
	ret := fn.StringToPositiveInteger("1")
	if ret != 1 {
		t.Fatal("StringToPositiveInteger failed")
	}

	ret = fn.StringToPositiveInteger("0")
	if ret != 0 {
		t.Fatal("StringToPositiveInteger failed")
	}

	ret = fn.StringToPositiveInteger("-10")
	if ret != -1 {
		t.Fatal("StringToPositiveInteger failed")
	}

	ret = fn.StringToPositiveInteger("A")
	if ret != -1 {
		t.Fatal("StringToPositiveInteger failed")
	}
}
