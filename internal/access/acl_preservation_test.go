package access

import (
	"errors"
	"fmt"
	"github.com/soulteary/RSS-Can/internal/source"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestACLUpdatesPreserveManagedContent(t *testing.T) {
	s := testStore(t)
	for _, u := range []string{"alice", "bob"} {
		if err := s.EnsurePrincipal(u); err != nil {
			t.Fatal(err)
		}
	}
	src, err := s.CreateManaged("alice", managedSpec())
	if err != nil {
		t.Fatal(err)
	}
	spec := managedSpec()
	spec.Name = "Edited"
	src, err = s.UpdateManaged("alice", src.ID, spec, src.Revision)
	if err != nil {
		t.Fatal(err)
	}
	grant, secret, err := s.CreateGrant("alice", src.ID, []string{"rss"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	epoch, _ := s.CanRead("alice", src.ID)
	assertContent := func(store *Store) {
		t.Helper()
		got, ok := store.Managed(src.ID)
		if !ok || got.Revision != src.Revision || !reflect.DeepEqual(got.Spec, src.Spec) || !reflect.DeepEqual(got.Previous, src.Previous) {
			t.Fatalf("lost managed content: %+v", got)
		}
	}
	if err := s.PutSource(Source{ID: src.ID, Owner: "alice", Public: true, Readers: []string{"bob"}}); err != nil {
		t.Fatal(err)
	}
	assertContent(s)
	if next, ok := s.CanRead("bob", src.ID); !ok || next <= epoch {
		t.Fatal("ACL epoch did not advance")
	}
	if _, _, err := s.ResolveGrant(grant.ID, secret, "rss", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.PutSource(Source{ID: src.ID, Owner: "bob"}); err != nil {
		t.Fatal(err)
	}
	assertContent(s)
	if _, _, err := s.ResolveGrant(grant.ID, secret, "rss", time.Now()); !errors.Is(err, ErrDenied) {
		t.Fatal("old grant remains valid", err)
	}
	if _, err := s.UpdateManaged("alice", src.ID, spec, src.Revision); !errors.Is(err, ErrDenied) {
		t.Fatal("old owner can edit", err)
	}
	path := s.path
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	assertContent(reopened)
	if _, err := reopened.UpdateManaged("bob", src.ID, spec, src.Revision); err != nil {
		t.Fatal(err)
	}
}
func TestACLTransferCapacityFailureKeepsStoreHealthy(t *testing.T) {
	s := testStore(t)
	spec := managedSpec().Normalized()
	spec.URL = "https://example.org/?q=" + strings.Repeat("x", 8000)
	if err := s.update(func(st *state) error {
		st.Principals["alice"] = Principal{}
		st.Principals["bob"] = Principal{}
		for i := 0; i < 6; i++ {
			owner := "bob"
			if i == 5 {
				owner = "alice"
			}
			cp := spec
			id := fmt.Sprintf("source%d", i)
			st.Sources[id] = Source{ID: id, Owner: owner, Spec: &cp, Previous: []source.Spec{spec, spec, spec, spec, spec}, Revision: 6}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutSource(Source{ID: "source5", Owner: "bob"}); !errors.Is(err, ErrSourceLimit) {
		t.Fatal(err)
	}
	got, ok := s.Managed("source5")
	if !ok || got.Owner != "alice" {
		t.Fatal("failed transfer poisoned store")
	}
	if err := s.PutSource(Source{ID: "source5", Owner: "alice", Public: true}); err != nil {
		t.Fatal(err)
	}
}
func TestPutSourceRejectsManagedMetadata(t *testing.T) {
	s := testStore(t)
	spec := managedSpec()
	for _, src := range []Source{{ID: "test", Owner: "alice", Spec: &spec}, {ID: "test", Owner: "alice", Previous: []source.Spec{spec}}, {ID: "test", Owner: "alice", Revision: 1}} {
		if err := s.PutSource(src); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
}

func TestACLTransferHonorsManagedSourceCount(t *testing.T) {
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
	if err := s.update(func(st *state) error {
		spec := managedSpec().Normalized()
		for i := 0; i < MaxManagedSourcesPerSubject; i++ {
			id := fmt.Sprintf("bob%d", i)
			st.Sources[id] = Source{ID: id, Owner: "bob", Spec: &spec, Revision: 1}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutSource(Source{ID: src.ID, Owner: "bob"}); !errors.Is(err, ErrSourceLimit) {
		t.Fatalf("source count limit bypassed: %v", err)
	}
	if got, ok := s.Managed(src.ID); !ok || got.Owner != "alice" {
		t.Fatal("rejected transfer changed ownership")
	}
}
