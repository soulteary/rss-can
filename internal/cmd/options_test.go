package cmd_test

import (
	"fmt"
	"testing"

	"github.com/soulteary/RSS-Can/internal/cmd"
	"github.com/soulteary/RSS-Can/internal/define"
)

func TestSantizeFeedPath(t *testing.T) {
	feedPath := cmd.SantizeFeedPath("////feedpath")
	if feedPath != "/feedpath" {
		t.Fatal("TestSantizeFeedPath failed")
	}

	feedPath = cmd.SantizeFeedPath("feedpath//")
	if feedPath != "/feedpath" {
		t.Fatal("TestSantizeFeedPath failed")
	}

	feedPath = cmd.SantizeFeedPath("////feedpath///")
	if feedPath != "/feedpath" {
		t.Fatal("TestSantizeFeedPath failed")
	}

	feedPath = cmd.SantizeFeedPath("//// feedpath ///")
	if feedPath != "/feedpath" {
		t.Fatal("TestSantizeFeedPath failed")
	}

	feedPath = cmd.SantizeFeedPath("//// feed path ///")
	fmt.Println(feedPath)
	if feedPath != "/feed path" {
		t.Fatal("TestSantizeFeedPath failed")
	}

	feedPath = cmd.SantizeFeedPath("//// fee$%^&*d p!ath /!//")
	fmt.Println(feedPath)
	if feedPath != define.DEFAULT_HTTP_FEED_PATH {
		t.Fatal("TestSantizeFeedPath failed")
	}
}
