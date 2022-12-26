package cmd_test

import (
	"fmt"
	"os"
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

func TestUpdateBoolOption(t *testing.T) {

	// env: empty, args: false, default: false
	ret := cmd.UpdateBoolOption("TEST_KEY", false, false)
	if ret {
		t.Fatal("UpdateBoolOption failed")
	}
	// env: empty, args: false, default: true
	ret = cmd.UpdateBoolOption("TEST_KEY", false, true)
	if ret {
		t.Fatal("UpdateBoolOption failed")
	}
	// env: empty, args: true, default: true
	ret = cmd.UpdateBoolOption("TEST_KEY", true, true)
	if !ret {
		t.Fatal("UpdateBoolOption failed")
	}
	// env: empty, args: false, default: true
	ret = cmd.UpdateBoolOption("TEST_KEY", false, true)
	if ret {
		t.Fatal("UpdateBoolOption failed")
	}

	// env: on, args: false, default: false
	os.Setenv("TEST_KEY", "on")
	ret = cmd.UpdateBoolOption("TEST_KEY", false, false)
	if !ret {
		t.Fatal("UpdateBoolOption failed")
	}
	// env: on, args: true, default: false
	os.Setenv("TEST_KEY", "on")
	ret = cmd.UpdateBoolOption("TEST_KEY", true, false)
	if !ret {
		t.Fatal("UpdateBoolOption failed")
	}
	// env: on, args: true, default: true
	os.Setenv("TEST_KEY", "on")
	ret = cmd.UpdateBoolOption("TEST_KEY", true, true)
	if !ret {
		t.Fatal("UpdateBoolOption failed")
	}
	// env: on, args: true, default: false
	os.Setenv("TEST_KEY", "on")
	ret = cmd.UpdateBoolOption("TEST_KEY", true, false)
	if !ret {
		t.Fatal("UpdateBoolOption failed")
	}

	// env: off, args: false, default: false
	os.Setenv("TEST_KEY", "off")
	ret = cmd.UpdateBoolOption("TEST_KEY", false, false)
	if ret {
		t.Fatal("UpdateBoolOption failed")
	}
	// env: off, args: true, default: false
	os.Setenv("TEST_KEY", "on")
	ret = cmd.UpdateBoolOption("TEST_KEY", true, false)
	if !ret {
		t.Fatal("UpdateBoolOption failed")
	}
	// env: off, args: true, default: true
	os.Setenv("TEST_KEY", "on")
	ret = cmd.UpdateBoolOption("TEST_KEY", true, true)
	if !ret {
		t.Fatal("UpdateBoolOption failed")
	}
	// env: on, args: true, default: false
	os.Setenv("TEST_KEY", "on")
	ret = cmd.UpdateBoolOption("TEST_KEY", true, false)
	if !ret {
		t.Fatal("UpdateBoolOption failed")
	}

	os.Setenv("TEST_KEY", "")
}
