package fn_test

import (
	"testing"

	"github.com/soulteary/RSS-Can/internal/fn"
)

func TestBase64Encode(t *testing.T) {
	if fn.Base64Encode("a") != "YQ==" {
		t.Fatal("base64 encode failed")
	}
}

func TestBase64Decode(t *testing.T) {
	if fn.Base64Decode("YQ==") != "a" {
		t.Fatal("base64 decode failed")
	}

	// test error input
	if fn.Base64Decode("YQ=!=") != "" {
		t.Fatal("base64 decode failed")
	}
}
