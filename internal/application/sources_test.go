package application

import (
	"github.com/soulteary/RSS-Can/internal/source"
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
