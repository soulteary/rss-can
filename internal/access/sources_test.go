package access

import (
	"errors"
	"fmt"
	"github.com/soulteary/RSS-Can/internal/source"
	"strings"
	"testing"
	"time"
)

func managedSpec() source.Spec {
	return source.Spec{Name: "News", URL: "https://example.org", ItemSelector: "article", TitleSelector: "h2", LinkSelector: "a"}
}
func TestManagedSourceOwnerRevisionAndRollback(t *testing.T) {
	s := testStore(t)
	if err := s.EnsurePrincipal("alice"); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsurePrincipal("bob"); err != nil {
		t.Fatal(err)
	}
	src, err := s.CreateManaged("alice", managedSpec())
	if err != nil {
		t.Fatal(err)
	}
	if src.Public || src.Owner != "alice" || src.Revision != 1 {
		t.Fatal("source is not private by default")
	}
	epoch, _ := s.CanRead("alice", src.ID)
	if _, err = s.UpdateManaged("bob", src.ID, managedSpec(), 1); !errors.Is(err, ErrDenied) {
		t.Fatal("cross-owner modification accepted")
	}
	changed := managedSpec()
	changed.Name = "Changed"
	updated, err := s.UpdateManaged("alice", src.ID, changed, 1)
	if err != nil || updated.Revision != 2 {
		t.Fatalf("update: %+v %v", updated, err)
	}
	if _, err = s.UpdateManaged("alice", src.ID, changed, 1); !errors.Is(err, ErrConflict) {
		t.Fatal("lost update accepted")
	}
	rolled, err := s.RollbackManaged("alice", src.ID, 2)
	if err != nil || rolled.Spec.Name != "News" || rolled.Revision != 3 {
		t.Fatalf("rollback %+v %v", rolled, err)
	}
	if current, _ := s.CanRead("alice", src.ID); current != epoch {
		t.Fatal("content edit invalidated other tenants' authorization epochs")
	}
	rolled.Spec.Name = "Mutated"
	rolled.Previous[0].Name = "Mutated"
	read, _ := s.Managed(src.ID)
	if read.Spec.Name == "Mutated" || read.Previous[0].Name == "Mutated" {
		t.Fatal("mutable spec escaped")
	}
	path := s.path
	_ = s.Close()
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	read, ok := reopened.Managed(src.ID)
	if !ok || read.Spec.Name != "News" {
		t.Fatal("managed source not durable")
	}
}

func TestManagedStorageCapacityIsPerSubject(t *testing.T) {
	st := state{Sources: map[string]Source{}}
	spec := managedSpec().Normalized()
	spec.URL = "https://example.org/?q=" + strings.Repeat("x", 8000)
	for i := 0; i < 5; i++ {
		copySpec := spec
		st.Sources[fmt.Sprint(i)] = Source{ID: fmt.Sprint(i), Owner: "alice", Spec: &copySpec, Previous: []source.Spec{spec, spec, spec, spec, spec}, Revision: 6}
	}
	if !errors.Is(managedCapacity(st), ErrSourceLimit) {
		t.Fatal("one owner can fill shared store")
	}
	for id, src := range st.Sources {
		src.Owner = id
		st.Sources[id] = src
	}
	if err := managedCapacity(st); err != nil {
		t.Fatal("independent owners share a per-user budget", err)
	}
}

func TestManagedOnboardingNeverReenablesDisabledUser(t *testing.T) {
	s := testStore(t)
	if err := s.EnsurePrincipal("alice"); err != nil {
		t.Fatal(err)
	}
	v, _ := s.CanRead("alice", "none")
	if err := s.EnsurePrincipal("alice"); err != nil {
		t.Fatal(err)
	}
	v2, _ := s.CanRead("alice", "none")
	if v != v2 {
		t.Fatal("existing login mutated authorization version")
	}
	_ = s.SetPrincipal("alice", true)
	if !errors.Is(s.EnsurePrincipal("alice"), ErrDenied) {
		t.Fatal("disabled user reenrolled")
	}
	if _, err := s.CreateManaged("alice", managedSpec()); !errors.Is(err, ErrDenied) {
		t.Fatal("disabled user created source")
	}
}

func TestManagedDeleteRevokesGrantsAndRejectsStaleEditors(t *testing.T) {
	s := testStore(t)
	_ = s.EnsurePrincipal("alice")
	_ = s.EnsurePrincipal("bob")
	src, err := s.CreateManaged("alice", managedSpec())
	if err != nil {
		t.Fatal(err)
	}
	g, secret, err := s.CreateGrant("alice", src.ID, []string{"rss"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteManaged("bob", src.ID, 1); !errors.Is(err, ErrDenied) {
		t.Fatal("cross-owner deletion accepted")
	}
	if err := s.DeleteManaged("alice", src.ID, 2); !errors.Is(err, ErrConflict) {
		t.Fatal("stale deletion accepted")
	}
	if err := s.DeleteManaged("alice", src.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Source(src.ID); ok {
		t.Fatal("deleted source remains")
	}
	if _, _, err := s.ResolveGrant(g.ID, secret, "rss", time.Now()); !errors.Is(err, ErrDenied) {
		t.Fatal("deleted source grant remains valid")
	}
	if err := s.update(func(st *state) error {
		for i := 0; i < MaxManagedSourcesPerSubject; i++ {
			spec := managedSpec().Normalized()
			id := fmt.Sprintf("src_%d", i)
			st.Sources[id] = Source{ID: id, Owner: "alice", Spec: &spec, Revision: 1}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateManaged("alice", managedSpec()); !errors.Is(err, ErrSourceLimit) {
		t.Fatal("source quota not applied")
	}
	if _, err := s.CreateManaged("bob", managedSpec()); err != nil {
		t.Fatal("alice consumed bob's source quota", err)
	}
}
