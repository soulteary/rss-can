package application

import (
	"context"
	"errors"
	"github.com/soulteary/RSS-Can/internal/source"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
)

func TestManagedSnapshotsStayPinnedAcrossEdits(t *testing.T) {
	cfg := teamConfig(t)
	app, err := New(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if err := app.access.EnsurePrincipal("alice"); err != nil {
		t.Fatal(err)
	}
	spec := source.Spec{Name: "Original", URL: "https://example.org", ItemSelector: "article", TitleSelector: "h2", LinkSelector: "a"}
	src, err := app.access.CreateManaged("alice", spec)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, ok := app.lookupSource(src.ID)
	if !ok || !snapshot.Managed {
		t.Fatal("managed lookup missing")
	}
	spec.Name = "Changed"
	if _, err := app.access.UpdateManaged("alice", src.ID, spec, 1); err != nil {
		t.Fatal(err)
	}
	old, err := app.configureSource(t.Context(), snapshot)
	if err != nil || old.Name != "Original" || !old.TextOnly {
		t.Fatalf("snapshot was not pinned: %+v %v", old, err)
	}
	latest := app.sourceSnapshots()[src.ID]
	current, err := app.configureSource(t.Context(), latest)
	if err != nil || current.Name != "Changed" {
		t.Fatal("latest source missing")
	}
	if app.sourceMetadata()[src.ID].Name != "Changed" {
		t.Fatal("managed metadata missing")
	}
	if _, ok := app.lookupSource("site"); !ok {
		t.Fatal("legacy source lost")
	}
	if _, err := app.previewSource(t.Context(), source.Spec{Name: "Blocked", URL: "http://127.0.0.1", ItemSelector: "article", TitleSelector: "h2", LinkSelector: "a"}); err == nil {
		t.Fatal("private preview target allowed")
	}
}

func TestTeamCanStartWithoutLegacyJavaScriptRules(t *testing.T) {
	cfg := teamConfig(t)
	cfg.Server.RuleDirectory = t.TempDir()
	app, err := New(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if len(app.sourceSnapshots()) != 0 {
		t.Fatal("unexpected startup source")
	}
}

func TestManagedPreviewKeepsPublicPolicyAndReportsDocumentBudget(t *testing.T) {
	var deep atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if deep.Load() {
			_, _ = w.Write([]byte(strings.Repeat("<div>", 60) + strings.Repeat("</div>", 60)))
			return
		}
		_, _ = w.Write([]byte(`<article><h2>Title</h2><a href="/post">Read</a></article>`))
	}))
	defer upstream.Close()
	cfg := teamConfig(t)
	cfg.Network.LookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}
	cfg.Network.DialContext = func(ctx context.Context, kind, address string) (net.Conn, error) {
		if address != "8.8.8.8:80" {
			t.Fatalf("unvalidated destination %s", address)
		}
		return (&net.Dialer{}).DialContext(ctx, kind, strings.TrimPrefix(upstream.URL, "http://"))
	}
	app, err := New(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	spec := source.Spec{Name: "News", URL: "http://source.example", ItemSelector: "article", TitleSelector: "h2", LinkSelector: "a"}
	items, err := app.previewSource(t.Context(), spec)
	if err != nil || len(items) != 1 || items[0].Link != "http://source.example/post" {
		t.Fatalf("preview failed: %+v %v", items, err)
	}
	deep.Store(true)
	if _, err := app.previewSource(t.Context(), spec); !errors.Is(err, source.ErrDocumentBudget) {
		t.Fatalf("document budget type lost: %v", err)
	}
}
