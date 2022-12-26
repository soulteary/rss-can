package fn_test

import (
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/fn"
)

func TestI2T(t *testing.T) {
	const target = 10 * time.Second
	ret := fn.I2T(10) * time.Second

	if target != ret {
		t.Fatal("I2T test failed")
	}
}
