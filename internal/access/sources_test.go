package access

import (
	"errors"
	"github.com/soulteary/RSS-Can/internal/source"
	"testing"
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
