package access

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGrantQuotaIsPerSubjectAndConcurrentSafe(t *testing.T) {
	s := testStore(t)
	put(t, s, Source{ID: "shared", Owner: "alice", Readers: []string{"bob"}})
	var mu sync.Mutex
	var grants []Grant
	failures := 0
	var wg sync.WaitGroup
	for i := 0; i < MaxGrantsPerSubject+8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			grant, _, err := s.CreateGrant("alice", "shared", []string{"rss"}, time.Now().Add(time.Hour))
			mu.Lock()
			defer mu.Unlock()
			if errors.Is(err, ErrGrantLimit) {
				failures++
				return
			}
			if err != nil {
				t.Errorf("create: %v", err)
				return
			}
			grants = append(grants, grant)
		}()
	}
	wg.Wait()
	if len(grants) != MaxGrantsPerSubject || failures != 8 {
		t.Fatalf("issued %d rejected %d", len(grants), failures)
	}
	if _, _, err := s.CreateGrant("bob", "shared", []string{"rss"}, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("another subject lost capacity: %v", err)
	}
	if err := s.RevokeGrant("alice", grants[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.CreateGrant("alice", "shared", []string{"rss"}, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("revocation did not free capacity: %v", err)
	}
	s.mu.RLock()
	_, retained := s.state.Grants[grants[0].ID]
	s.mu.RUnlock()
	if retained {
		t.Fatal("revoked record was retained")
	}
}

func TestInactiveGrantsArePrunedWithoutResurrectingTokens(t *testing.T) {
	s := testStore(t)
	put(t, s, Source{ID: "source", Owner: "alice"})
	revoked, revokedSecret, err := s.CreateGrant("alice", "source", []string{"rss"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	expired, expiredSecret, err := s.CreateGrant("alice", "source", []string{"rss"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeGrant("alice", revoked.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.updateState(false, func(st *state) error {
		g := st.Grants[expired.ID]
		g.ExpiresAt = time.Now().Add(-time.Second)
		st.Grants[expired.ID] = g
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// An administrative mutation also reclaims stale credentials, even when no
	// new reader grant is created.
	if err := s.SetPrincipal("bob", false); err != nil {
		t.Fatal(err)
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
	for _, pair := range []struct{ id, secret string }{{revoked.ID, revokedSecret}, {expired.ID, expiredSecret}} {
		if _, _, err := reopened.ResolveGrant(pair.id, pair.secret, "rss", time.Now()); !errors.Is(err, ErrDenied) {
			t.Fatal("inactive token recovered")
		}
		if _, ok := reopened.state.Grants[pair.id]; ok {
			t.Fatal("inactive record persisted")
		}
	}
}

func TestGrantGlobalLimitReservesAdministrationCapacity(t *testing.T) {
	s := testStore(t)
	put(t, s, Source{ID: "source", Owner: "alice", Readers: []string{"bob"}})
	if err := s.updateState(false, func(st *state) error {
		for i := 0; i < maxGrantRecords; i++ {
			id := fmt.Sprintf("legacy-%d", i)
			st.Grants[id] = Grant{ID: id, Source: "source", Subject: "alice", Digest: strings.Repeat("0", 64), Formats: []string{"rss"}, ExpiresAt: time.Now().Add(time.Hour)}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.CreateGrant("bob", "source", []string{"rss"}, time.Now().Add(time.Hour)); !errors.Is(err, ErrGrantLimit) {
		t.Fatalf("global quota: %v", err)
	}
	if err := s.SetPrincipal("operator-provisioned", false); err != nil {
		t.Fatalf("grants occupied reserved principal capacity: %v", err)
	}
	if err := s.PutSource(Source{ID: "another", Owner: "operator-provisioned"}); err != nil {
		t.Fatalf("grants occupied reserved source capacity: %v", err)
	}
}

func TestReaderCredentialChangesKeepAuthorizationVersionStable(t *testing.T) {
	s := testStore(t)
	put(t, s, Source{ID: "source", Owner: "alice"})
	initial, _ := s.CanRead("alice", "source")
	grant, secret, err := s.CreateGrant("alice", "source", []string{"rss"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, version, err := s.ResolveGrant(grant.ID, secret, "rss", time.Now()); err != nil || version != initial {
		t.Fatalf("creation changed source version: %d %v", version, err)
	}
	if err := s.RevokeGrant("alice", grant.ID); err != nil {
		t.Fatal(err)
	}
	if version, _ := s.CanRead("alice", "source"); version != initial {
		t.Fatal("revocation invalidated unrelated content")
	}
	if _, _, err := s.ResolveGrant(grant.ID, secret, "rss", time.Now()); !errors.Is(err, ErrDenied) {
		t.Fatal("stable version allowed revoked credential")
	}
	if err := s.SetPrincipal("alice", true); err != nil {
		t.Fatal(err)
	}
	if version, _ := s.CanRead("alice", "source"); version <= initial {
		t.Fatal("permission change did not advance version")
	}
}
