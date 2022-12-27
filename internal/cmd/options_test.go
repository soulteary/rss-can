package cmd_test

import (
	"fmt"
	"os"
	"strconv"
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

func TestUpdateNumberOption(t *testing.T) {
	// env: empty, args:1, default:0, allowZero:true
	ret := cmd.UpdateNumberOption("TEST_KEY", 1, 0, true)
	if ret != 1 {
		t.Fatal("UpdateNumberOption failed")
	}
	// env: empty, args:0, default:0, allowZero:true
	ret = cmd.UpdateNumberOption("TEST_KEY", 0, 0, true)
	if ret != 0 {
		t.Fatal("UpdateNumberOption failed")
	}
	// env: empty, args:0, default:1, allowZero:true
	ret = cmd.UpdateNumberOption("TEST_KEY", 0, 1, true)
	if ret != 0 {
		t.Fatal("UpdateNumberOption failed")
	}
	// env: empty, args:0, default:1, allowZero:false
	ret = cmd.UpdateNumberOption("TEST_KEY", 0, 1, false)
	if ret != 1 {
		t.Fatal("UpdateNumberOption failed")
	}

	os.Setenv("TEST_KEY", "2")
	// env: "2", args:1, default:0, allowZero:true
	ret = cmd.UpdateNumberOption("TEST_KEY", 1, 0, true)
	fmt.Println(ret)
	if ret != 1 {
		t.Fatal("UpdateNumberOption failed")
	}
	// env: "2", args:0, default:0, allowZero:true
	ret = cmd.UpdateNumberOption("TEST_KEY", 0, 0, true)
	if ret != 2 {
		t.Fatal("UpdateNumberOption failed")
	}
	// env: "2", args:0, default:1, allowZero:true
	ret = cmd.UpdateNumberOption("TEST_KEY", 0, 1, true)
	if ret != 0 {
		t.Fatal("UpdateNumberOption failed")
	}
	// env: "2", args:0, default:1, allowZero:false
	ret = cmd.UpdateNumberOption("TEST_KEY", 0, 1, false)
	if ret != 2 {
		t.Fatal("UpdateNumberOption failed")
	}
	// env: "2", args:3, default:1, allowZero:false
	ret = cmd.UpdateNumberOption("TEST_KEY", 3, 1, false)
	if ret != 3 {
		t.Fatal("UpdateNumberOption failed")
	}
	os.Setenv("TEST_KEY", "")
}

func TestUpdateStringOption(t *testing.T) {
	// env: empty, args:"a", default:"d"
	ret := cmd.UpdateStringOption("TEST_KEY", "a", "d")
	if ret != "a" {
		t.Fatal("UpdateStringOption failed")
	}
	// env: empty, args:"", default:"d"
	ret = cmd.UpdateStringOption("TEST_KEY", "", "d")
	if ret != "d" {
		t.Fatal("UpdateStringOption failed")
	}
	// env: empty, args:"a", default:""
	ret = cmd.UpdateStringOption("TEST_KEY", "a", "")
	if ret != "a" {
		t.Fatal("UpdateStringOption failed")
	}
	// env: empty, args:"", default:""
	ret = cmd.UpdateStringOption("TEST_KEY", "", "")
	if ret != "" {
		t.Fatal("UpdateStringOption failed")
	}

	os.Setenv("TEST_KEY", "e")
	// env: "e", args:"a", default:"d"
	ret = cmd.UpdateStringOption("TEST_KEY", "a", "d")
	if ret != "a" {
		t.Fatal("UpdateStringOption failed")
	}
	// env: "e", args:"", default:"d"
	ret = cmd.UpdateStringOption("TEST_KEY", "", "d")
	if ret != "e" {
		t.Fatal("UpdateStringOption failed")
	}
	// env: "e", args:"a", default:""
	ret = cmd.UpdateStringOption("TEST_KEY", "a", "")
	if ret != "a" {
		t.Fatal("UpdateStringOption failed")
	}
	// env: "e", args:"", default:""
	ret = cmd.UpdateStringOption("TEST_KEY", "", "")
	if ret != "e" {
		t.Fatal("UpdateStringOption failed")
	}
	os.Setenv("TEST_KEY", "")
}

func TestUpdateLogOption(t *testing.T) {
	// env: empty, args: "", default: "info"
	ret := cmd.UpdateLogOption("TEST_KEY", "", "info")
	if ret != "info" {
		t.Fatal("UpdateLogOption failed")
	}
	// env: empty, args: "error", default: "info"
	ret = cmd.UpdateLogOption("TEST_KEY", "error", "info")
	if ret != "error" {
		t.Fatal("UpdateLogOption failed")
	}

	os.Setenv("TEST_KEY", "warn")
	// env: "warn", args: "", default: "info"
	ret = cmd.UpdateLogOption("TEST_KEY", "", "info")
	if ret != "warn" {
		t.Fatal("UpdateLogOption failed")
	}
	// env: "warn", args: "error", default: "info"
	ret = cmd.UpdateLogOption("TEST_KEY", "error", "info")
	if ret != "error" {
		t.Fatal("UpdateLogOption failed")
	}
	os.Setenv("TEST_KEY", "")

	ret = cmd.UpdateLogOption("TEST_KEY", "errorA", "info")
	if ret != "info" {
		t.Fatal("UpdateLogOption failed")
	}

	ret = cmd.UpdateLogOption("TEST_KEY", "ErrOR", "info")
	if ret != "error" {
		t.Fatal("UpdateLogOption failed")
	}
}

func TestUpdateFeedPathOption(t *testing.T) {
	// env: empty, args: "", default: "/feed"
	ret := cmd.UpdateFeedPathOption("TEST_KEY", "", define.DEFAULT_HTTP_FEED_PATH)
	if ret != "/feed" {
		t.Fatal("UpdateFeedPathOption failed")
	}
	// env: empty, args: "/new-feed-path", default: "/feed"
	ret = cmd.UpdateFeedPathOption("TEST_KEY", "/new-feed-path", define.DEFAULT_HTTP_FEED_PATH)
	if ret != "/new-feed-path" {
		t.Fatal("UpdateFeedPathOption failed")
	}

	os.Setenv("TEST_KEY", "/new-feed-path")
	// env: empty, args: "/new-feed-path", default: "/feed"
	ret = cmd.UpdateFeedPathOption("TEST_KEY", "", define.DEFAULT_HTTP_FEED_PATH)
	if ret != "/new-feed-path" {
		t.Fatal("UpdateFeedPathOption failed")
	}
	os.Setenv("TEST_KEY", "")
}

func TestUpdatePortOption(t *testing.T) {
	// env: empty, args: 0, default: 8080
	ret := cmd.UpdatePortOption("TEST_KEY", 0, define.DEFAULT_HTTP_PORT)
	if ret != define.DEFAULT_HTTP_PORT {
		t.Fatal("UpdatePortOption failed")
	}
	// env: empty, args: -1, default: 8080
	ret = cmd.UpdatePortOption("TEST_KEY", -1, define.DEFAULT_HTTP_PORT)
	if ret != define.DEFAULT_HTTP_PORT {
		t.Fatal("UpdatePortOption failed")
	}
	// env: empty, args: 8090, default: 8080
	ret = cmd.UpdatePortOption("TEST_KEY", 8090, define.DEFAULT_HTTP_PORT)
	if ret != 8090 {
		t.Fatal("UpdatePortOption failed")
	}

	os.Setenv("TEST_KEY", "9090")
	// env: "9090", args: 0, default: 8080
	ret = cmd.UpdatePortOption("TEST_KEY", 0, define.DEFAULT_HTTP_PORT)
	if ret != 9090 {
		t.Fatal("UpdatePortOption failed")
	}
	// env: "9090", args: -1, default: 8080
	ret = cmd.UpdatePortOption("TEST_KEY", -1, define.DEFAULT_HTTP_PORT)
	if ret != 9090 {
		t.Fatal("UpdatePortOption failed")
	}
	// env: "9090", args: 8090, default: 8080
	ret = cmd.UpdatePortOption("TEST_KEY", 8090, define.DEFAULT_HTTP_PORT)
	if ret != 8090 {
		t.Fatal("UpdatePortOption failed")
	}
	os.Setenv("TEST_KEY", "0")
	// env: "0", args: 0, default: 8080
	ret = cmd.UpdatePortOption("TEST_KEY", 0, define.DEFAULT_HTTP_PORT)
	if ret != define.DEFAULT_HTTP_PORT {
		t.Fatal("UpdatePortOption failed")
	}
	os.Setenv("TEST_KEY", "-1")
	// env: "-1", args: 0, default: 8080
	ret = cmd.UpdatePortOption("TEST_KEY", 0, define.DEFAULT_HTTP_PORT)
	if ret != define.DEFAULT_HTTP_PORT {
		t.Fatal("UpdatePortOption failed")
	}
	os.Setenv("TEST_KEY", "")
}

func TestUpdateHostOption(t *testing.T) {
	// env: empty, args: "", default: "0.0.0.0"
	ret := cmd.UpdateHostOption("TEST_KEY", "", define.DEFAULT_HTTP_HOST)
	if ret != define.DEFAULT_HTTP_HOST {
		t.Fatal("UpdateHostOption failed")
	}
	// env: empty, args: "127.0.0.1", default: "0.0.0.0"
	ret = cmd.UpdateHostOption("TEST_KEY", "127.0.0.1", define.DEFAULT_HTTP_HOST)
	if ret != "127.0.0.1" {
		t.Fatal("UpdateHostOption failed")
	}
	// env: empty, args: "1127.0.0.1", default: "0.0.0.0"
	ret = cmd.UpdateHostOption("TEST_KEY", "1127.0.0.1", define.DEFAULT_HTTP_HOST)
	if ret != define.DEFAULT_HTTP_HOST {
		t.Fatal("UpdateHostOption failed")
	}

	os.Setenv("TEST_KEY", "127.0.0.2")
	// env: "127.0.0.2", args: "", default: "0.0.0.0"
	ret = cmd.UpdateHostOption("TEST_KEY", "", define.DEFAULT_HTTP_HOST)
	if ret != "127.0.0.2" {
		t.Fatal("UpdateHostOption failed")
	}
	// env: "127.0.0.2", args: "127.0.0.1", default: "0.0.0.0"
	ret = cmd.UpdateHostOption("TEST_KEY", "127.0.0.1", define.DEFAULT_HTTP_HOST)
	if ret != "127.0.0.1" {
		t.Fatal("UpdateHostOption failed")
	}
	// env: "127.0.0.2", args: "1127.0.0.1", default: "0.0.0.0"
	ret = cmd.UpdateHostOption("TEST_KEY", "1127.0.0.1", define.DEFAULT_HTTP_HOST)
	if ret != "127.0.0.2" {
		t.Fatal("UpdateHostOption failed")
	}

	os.Setenv("TEST_KEY", "1277.0.0.2")
	// env: "1277.0.0.2", args: "", default: "0.0.0.0"
	ret = cmd.UpdateHostOption("TEST_KEY", "", define.DEFAULT_HTTP_HOST)
	if ret != define.DEFAULT_HTTP_HOST {
		t.Fatal("UpdateHostOption failed")
	}
	// env: "1277.0.0.2", args: "127.0.0.1", default: "0.0.0.0"
	ret = cmd.UpdateHostOption("TEST_KEY", "127.0.0.1", define.DEFAULT_HTTP_HOST)
	if ret != "127.0.0.1" {
		t.Fatal("UpdateHostOption failed")
	}
	// env: "1277.0.0.2", args: "1127.0.0.1", default: "0.0.0.0"
	ret = cmd.UpdateHostOption("TEST_KEY", "1127.0.0.1", define.DEFAULT_HTTP_HOST)
	if ret != define.DEFAULT_HTTP_HOST {
		t.Fatal("UpdateHostOption failed")
	}
	os.Setenv("TEST_KEY", "")
}

func TestUpdateAddrOption(t *testing.T) {
	defaults := define.DEFAULT_HTTP_HOST + ":" + strconv.Itoa(define.DEFAULT_HTTP_PORT)

	// env: empty, args: "", default: "0.0.0.0:8080"
	ret := cmd.UpdateAddrOption("TEST_KEY", "", defaults)
	if ret != defaults {
		t.Fatal("UpdateAddrOption failed")
	}
	// env: empty, args: "127.0.0.1:9090", default: "0.0.0.0:8080"
	ret = cmd.UpdateAddrOption("TEST_KEY", "127.0.0.1:9090", defaults)
	if ret != "127.0.0.1:9090" {
		t.Fatal("UpdateAddrOption failed")
	}
	// env: empty, args: "1127.0.0.1:9090", default: "0.0.0.0:8080"
	ret = cmd.UpdateAddrOption("TEST_KEY", "1127.0.0.1", defaults)
	if ret != defaults {
		t.Fatal("UpdateAddrOption failed")
	}

	os.Setenv("TEST_KEY", "127.0.0.2:9999")
	// env: "127.0.0.2", args: "", default: "0.0.0.0:8080"
	ret = cmd.UpdateAddrOption("TEST_KEY", "", defaults)
	if ret != "127.0.0.2:9999" {
		t.Fatal("UpdateAddrOption failed")
	}
	// env: "127.0.0.2", args: "127.0.0.1:1122", default: "0.0.0.0:8080"
	ret = cmd.UpdateAddrOption("TEST_KEY", "127.0.0.1:1122", defaults)
	if ret != "127.0.0.1:1122" {
		t.Fatal("UpdateAddrOption failed")
	}
	// env: "127.0.0.2", args: "1127.0.0.1:1123", default: "0.0.0.0:8080"
	ret = cmd.UpdateAddrOption("TEST_KEY", "1127.0.0.1:1123", defaults)
	if ret != "127.0.0.2:9999" {
		t.Fatal("UpdateAddrOption failed")
	}

	os.Setenv("TEST_KEY", "1277.0.0.2:2345")
	// env: "1277.0.0.2:2345", args: "", default: "0.0.0.0:8080"
	ret = cmd.UpdateAddrOption("TEST_KEY", "", defaults)
	if ret != defaults {
		t.Fatal("UpdateAddrOption failed")
	}
	// env: "1277.0.0.2:2345", args: "127.0.0.1", default: "0.0.0.0:8080"
	ret = cmd.UpdateAddrOption("TEST_KEY", "127.0.0.1", defaults)
	if ret != "127.0.0.1" {
		t.Fatal("UpdateAddrOption failed")
	}
	// env: "1277.0.0.2:2345", args: "1127.0.0.1", default: "0.0.0.0:8080"
	ret = cmd.UpdateAddrOption("TEST_KEY", "1127.0.0.1", defaults)
	if ret != defaults {
		t.Fatal("UpdateAddrOption failed")
	}
	os.Setenv("TEST_KEY", "")
}

func TestUpdateHeadlessOptions(t *testing.T) {
	defaults := define.DEFAULT_HTTP_HOST + ":" + strconv.Itoa(define.DEFAULT_HTTP_PORT)

	host := "http://" + defaults
	// env: empty, args: "", default: "http://0.0.0.0:8080"
	ret := cmd.UpdateHeadlessOptions("TEST_KEY", "", host)
	if ret != host {
		t.Fatal("UpdateHeadlessOptions failed")
	}
	// env: empty, args: "http://127.0.0.1:9090", default: "http://0.0.0.0:8080"
	ret = cmd.UpdateHeadlessOptions("TEST_KEY", "http://127.0.0.1:9090", host)
	if ret != "http://127.0.0.1:9090" {
		t.Fatal("UpdateHeadlessOptions failed")
	}
	// env: empty, args: "http://1127.0.0.1:9090", default: "http://0.0.0.0:8080"
	ret = cmd.UpdateHeadlessOptions("TEST_KEY", "http://1127.0.0.1", host)
	if ret != host {
		t.Fatal("UpdateHeadlessOptions failed")
	}

	// env: empty, args: "http://redis:9090", default: "http://0.0.0.0:8080"
	ret = cmd.UpdateHeadlessOptions("TEST_KEY", "http://redis:9090", host)
	if ret != "http://redis:9090" {
		t.Fatal("UpdateHeadlessOptions failed")
	}
	// env: empty, args: "http://redis.domain.ltd:9090", default: "http://0.0.0.0:8080"
	ret = cmd.UpdateHeadlessOptions("TEST_KEY", "http://redis.domain.ltd:9090", host)
	if ret != "http://redis.domain.ltd:9090" {
		t.Fatal("UpdateHeadlessOptions failed")
	}
	// env: empty, args: "https://redis.domain.ltd:9090", default: "http://0.0.0.0:8080"
	ret = cmd.UpdateHeadlessOptions("TEST_KEY", "https://redis.domain.ltd:9090", host)
	if ret != "https://redis.domain.ltd:9090" {
		t.Fatal("UpdateHeadlessOptions failed")
	}
	// env: empty, args: "ws://redis.domain.ltd:9090", default: "http://0.0.0.0:8080"
	ret = cmd.UpdateHeadlessOptions("TEST_KEY", "ws://redis.domain.ltd:9090", host)
	if ret != "ws://redis.domain.ltd:9090" {
		t.Fatal("UpdateHeadlessOptions failed")
	}
	// env: empty, args: "wss://redis.domain.ltd:9090", default: "http://0.0.0.0:8080"
	ret = cmd.UpdateHeadlessOptions("TEST_KEY", "wss://redis.domain.ltd:9090", host)
	if ret != "wss://redis.domain.ltd:9090" {
		t.Fatal("UpdateHeadlessOptions failed")
	}

	os.Setenv("TEST_KEY", "http://127.0.0.2:9999")
	// env: "http://127.0.0.2", args: "", default: "http://0.0.0.0:8080"
	ret = cmd.UpdateHeadlessOptions("TEST_KEY", "", host)
	if ret != "http://127.0.0.2:9999" {
		t.Fatal("UpdateHeadlessOptions failed")
	}
	// env: "http://127.0.0.2", args: "http://127.0.0.1:1122", default: "http://0.0.0.0:8080"
	ret = cmd.UpdateHeadlessOptions("TEST_KEY", "http://127.0.0.1:1122", host)
	if ret != "http://127.0.0.1:1122" {
		t.Fatal("UpdateHeadlessOptions failed")
	}
	// env: "http://127.0.0.2", args: "http://1127.0.0.1:1123", default: "http://0.0.0.0:8080"
	ret = cmd.UpdateHeadlessOptions("TEST_KEY", "http://1127.0.0.1:1123", host)
	if ret != "http://127.0.0.2:9999" {
		t.Fatal("UpdateHeadlessOptions failed")
	}

	os.Setenv("TEST_KEY", "http://1277.0.0.2:2345")
	// env: "http://1277.0.0.2:2345", args: "", default: "http://0.0.0.0:8080"
	ret = cmd.UpdateHeadlessOptions("TEST_KEY", "", host)
	if ret != host {
		t.Fatal("UpdateHeadlessOptions failed")
	}
	// env: "http://1277.0.0.2:2345", args: "http://127.0.0.1", default: "http://0.0.0.0:8080"
	ret = cmd.UpdateHeadlessOptions("TEST_KEY", "http://127.0.0.1", host)
	if ret != "http://127.0.0.1" {
		t.Fatal("UpdateHeadlessOptions failed")
	}
	// env: "http://1277.0.0.2:2345", args: "http://1127.0.0.1", default: "http://0.0.0.0:8080"
	ret = cmd.UpdateHeadlessOptions("TEST_KEY", "http://1127.0.0.1", host)
	if ret != host {
		t.Fatal("UpdateHeadlessOptions failed")
	}
	os.Setenv("TEST_KEY", "")
}
