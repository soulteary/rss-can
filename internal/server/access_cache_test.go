package server

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestTeamDeniedConditionalRequestsAreNotCacheable(t *testing.T) {
	app, controller, gen := accessApp(t)
	for _, tc := range []struct {
		name, path, subject string
		directoryError      error
		status              int
	}{
		{name: "missing identity", path: "/feed/private-a/rss", status: 401},
		{name: "hidden source", path: "/feed/private-a/rss", subject: "bob", status: 404},
		{name: "unknown source", path: "/feed/missing/rss", subject: "alice", status: 404},
		{name: "invalid reader", path: "/s/unknown/unknown/rss", status: 401},
		{name: "directory failure", path: "/feed/private-a/rss", subject: "alice", directoryError: errors.New("offline"), status: 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			controller.Directory.(*testDirectory).err = tc.directoryError
			request := requestAs(http.MethodGet, tc.path, tc.subject, "")
			request.Header.Set("If-None-Match", "*")
			request.Header.Set("If-Modified-Since", "Wed, 21 Oct 2099 07:28:00 GMT")
			before := gen.calls
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != tc.status {
				t.Fatalf("status=%d want=%d", response.StatusCode, tc.status)
			}
			if response.Header.Get("Cache-Control") != "private, no-store" {
				t.Fatalf("cache policy=%q", response.Header.Get("Cache-Control"))
			}
			if response.StatusCode == http.StatusNotModified || gen.calls != before {
				t.Fatal("conditional request bypassed authorization")
			}
		})
	}
}
func TestTeamAuthorizedFeedRetainsPrivateRevalidation(t *testing.T) {
	app, _, _ := accessApp(t)
	response, err := app.Test(requestAs(http.MethodGet, "/feed/private-a/rss", "alice", ""))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status %d", response.StatusCode)
	}
	if value := response.Header.Get("Cache-Control"); value != "private, no-cache" || strings.Contains(value, "public") {
		t.Fatalf("cache policy %q", value)
	}
}
