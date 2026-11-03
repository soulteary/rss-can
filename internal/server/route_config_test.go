package server

import (
	"encoding/json"
	"github.com/soulteary/RSS-Can/internal/access"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/source"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestValidateFeedPath(t *testing.T) {
	for _, team := range []bool{false, true} {
		for _, p := range []string{"/config", "/CONFIG/extra/", "/inspector", "/assets", "/_", "/:id", "/*", "/api/../feed", "/", "relative"} {
			if ValidateFeedPath(p, team) == nil {
				t.Errorf("accepted %q team=%v", p, team)
			}
		}
		for _, p := range []string{"", "/feed", "/feeds/", "/custom/list", "/test feed"} {
			if err := ValidateFeedPath(p, team); err != nil {
				t.Errorf("rejected %q: %v", p, err)
			}
		}
	}
	for _, p := range []string{"/sources", "/SOURCES/child", "/api", "/s"} {
		if ValidateFeedPath(p, true) == nil {
			t.Errorf("accepted %q", p)
		}
		if err := ValidateFeedPath(p, false); err != nil {
			t.Error(err)
		}
	}
}
func TestReservedFeedPathFailsClosedForEmbedder(t *testing.T) {
	logger.Initialize()
	app := NewAppWithDependencies(Options{FeedPath: "/sources", Access: &access.Config{}}, Dependencies{})
	resp, err := app.Test(httptest.NewRequest("GET", "http://example.org/sources", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Fatal(resp.StatusCode)
	}
}

func TestAdminACLUpdatePreservesManagedSource(t *testing.T) {
	_, controller, gen := accessApp(t)
	src, err := controller.Store.CreateManaged("alice", source.Spec{Name: "News", URL: "https://example.org", ItemSelector: "article", TitleSelector: "h2", LinkSelector: "a"})
	if err != nil {
		t.Fatal(err)
	}
	spec := *src.Spec
	spec.Name = "Updated"
	src, err = controller.Store.UpdateManaged("alice", src.ID, spec, src.Revision)
	if err != nil {
		t.Fatal(err)
	}
	app := NewAppWithDependencies(Options{FeedPath: "/feed", Access: &controller.Config, AdminToken: strings.Repeat("a", 32)}, Dependencies{Feeds: gen, Access: controller, Snapshot: func() map[string]define.RuleCache { return map[string]define.RuleCache{src.ID: {}} }})
	body, _ := json.Marshal(map[string]any{"id": src.ID, "owner": "alice", "public": true})
	req := requestAs("POST", "/api/access/sources", "", string(body))
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 32))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatal(resp.StatusCode)
	}
	got, ok := controller.Store.Managed(src.ID)
	if !ok || !got.Public || got.Revision != src.Revision || !reflect.DeepEqual(got.Spec, src.Spec) || !reflect.DeepEqual(got.Previous, src.Previous) {
		t.Fatalf("ACL endpoint destroyed managed source: %+v", got)
	}
}
