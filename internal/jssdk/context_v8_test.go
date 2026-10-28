//go:build v8go

package jssdk_test

import (
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"testing"
)

func TestGetCtxWithJS(t *testing.T) {
	ctx, err := jssdk.GetCtxWithJS("1", "test.js")
	if err != nil {
		t.Fatal("GetCtxWithJS failed")
	}

	jssdk.CloseContext(ctx)
	_, err = jssdk.GetCtxWithJS("!not-found-command", "test.js")
	if err == nil {
		t.Fatal("GetCtxWithJS failed")
	}
}
