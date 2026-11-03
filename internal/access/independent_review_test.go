package access

import (
	"github.com/soulteary/RSS-Can/internal/source"
	"strings"
	"testing"
)

func TestIndependentReviewRejectsOriginalCPUInputsAtAllBoundaries(t *testing.T) {
	store := testStore(t)
	if err := store.EnsurePrincipal("alice"); err != nil {
		t.Fatal(err)
	}
	good := managedSpec()
	existing, err := store.CreateManaged("alice", good)
	if err != nil {
		t.Fatal(err)
	}
	attacks := []string{strings.Repeat("div:has(", 10) + "span.never" + strings.Repeat(")", 10), "span.never " + strings.Repeat("div ", 10)}
	setters := []func(*source.Spec, string){
		func(s *source.Spec, v string) { s.ItemSelector = v }, func(s *source.Spec, v string) { s.TitleSelector = v }, func(s *source.Spec, v string) { s.LinkSelector = v }, func(s *source.Spec, v string) { s.DateSelector = v }, func(s *source.Spec, v string) { s.DescriptionSelector = v }, func(s *source.Spec, v string) { s.PagerSelector = v },
	}
	for _, attack := range attacks {
		for index, set := range setters {
			candidate := good
			set(&candidate, attack)
			if _, err := candidate.Config(); err == nil {
				t.Fatalf("Config accepted attack in field %d", index)
			}
			if _, err := store.CreateManaged("alice", candidate); err == nil {
				t.Fatalf("CreateManaged accepted field %d", index)
			}
			if _, err := store.UpdateManaged("alice", existing.ID, candidate, existing.Revision); err == nil {
				t.Fatalf("UpdateManaged accepted field %d", index)
			}
			if err := store.PutSource(Source{ID: "unsafe-review", Owner: "alice", Spec: &candidate, Revision: 1}); err == nil {
				t.Fatalf("PutSource accepted field %d", index)
			}
		}
	}
	current, ok := store.Managed(existing.ID)
	if !ok || current.Revision != existing.Revision || *current.Spec != good.Normalized() {
		t.Fatal("rejected inputs changed persisted valid source")
	}
}
