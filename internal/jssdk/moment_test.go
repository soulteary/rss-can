package jssdk_test

import (
	"fmt"
	"testing"

	"github.com/soulteary/RSS-Can/internal/jssdk"
)

func TestConvertAgoToUnix(t *testing.T) {
	timeUnix, err := jssdk.ConvertAgoToUnix("10天前")
	if err != nil {
		t.Fatal("ConvertAgoToUnix failed: ", err)
	}

	fmt.Println(timeUnix)
}
