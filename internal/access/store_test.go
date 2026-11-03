package access

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := Open(filepath.Join(dir, "access.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
func put(t *testing.T, s *Store, src Source) {
	t.Helper()
	if err := s.PutSource(src); err != nil {
		t.Fatal(err)
	}
}

func TestStoreAuthorizationAndPersistence(t *testing.T) {
	s := testStore(t)
	put(t, s, Source{ID: "private", Owner: "alice", Readers: []string{"bob"}})
	put(t, s, Source{ID: "public", Owner: "alice", Public: true})
	for _, tc := range []struct {
		sub, src string
		want     bool
	}{{"alice", "private", true}, {"bob", "private", true}, {"mallory", "private", false}, {"", "private", false}, {"", "public", true}, {"alice", "missing", false}} {
		if _, got := s.CanRead(tc.sub, tc.src); got != tc.want {
			t.Fatalf("%+v got %v", tc, got)
		}
	}
	before, _ := s.CanRead("alice", "private")
	g, secret, err := s.CreateGrant("bob", "private", []string{"rss"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(secret) != 43 || g.Digest != "" {
		t.Fatal("bad exported grant")
	}
	body, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), secret) {
		t.Fatal("plaintext secret persisted")
	}
	info, _ := os.Stat(s.path)
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}
	if _, _, err = s.ResolveGrant(g.ID, secret, "rss", time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id, secret, format string
		now                time.Time
	}{{g.ID, secret, "json", time.Now()}, {g.ID, "wrong", "rss", time.Now()}, {"missing", secret, "rss", time.Now()}, {g.ID, secret, "rss", time.Now().Add(2 * time.Hour)}} {
		if _, _, err := s.ResolveGrant(tc.id, tc.secret, tc.format, tc.now); !errors.Is(err, ErrDenied) {
			t.Fatalf("accepted %+v", tc)
		}
	}
	path := s.path
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if _, v, err := s2.ResolveGrant(g.ID, secret, "rss", time.Now()); err != nil || v <= before {
		t.Fatalf("reopen: version %d err %v", v, err)
	}
	if err = s2.RevokeGrant("alice", g.ID); !errors.Is(err, ErrDenied) {
		t.Fatal("other user revoked grant")
	}
	if err = s2.RevokeGrant("bob", g.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s2.ResolveGrant(g.ID, secret, "rss", time.Now()); !errors.Is(err, ErrDenied) {
		t.Fatal("revoked grant valid")
	}
}

func TestDisableAndOwnershipChangeRevokeGrants(t *testing.T) {
	s := testStore(t)
	put(t, s, Source{ID: "source", Owner: "alice", Readers: []string{"bob"}})
	g, secret, _ := s.CreateGrant("bob", "source", []string{"rss"}, time.Now().Add(time.Hour))
	if err := s.SetPrincipal("bob", true); err != nil {
		t.Fatal(err)
	}
	if s.Active("bob") {
		t.Fatal("disabled remains active")
	}
	if err := s.SetPrincipal("bob", false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.ResolveGrant(g.ID, secret, "rss", time.Now()); err == nil {
		t.Fatal("reenabling resurrects revoked grant")
	}
	g, secret, _ = s.CreateGrant("bob", "source", []string{"rss"}, time.Now().Add(time.Hour))
	put(t, s, Source{ID: "source", Owner: "mallory", Readers: []string{"bob"}})
	if _, _, err := s.ResolveGrant(g.ID, secret, "rss", time.Now()); err == nil {
		t.Fatal("ownership transfer kept grant")
	}
}

func TestACLChangeImmediatelyInvalidatesGrant(t *testing.T) {
	s := testStore(t)
	put(t, s, Source{ID: "source", Owner: "alice", Readers: []string{"bob"}})
	g, secret, _ := s.CreateGrant("bob", "source", []string{"rss"}, time.Now().Add(time.Hour))
	put(t, s, Source{ID: "source", Owner: "alice"})
	if _, _, err := s.ResolveGrant(g.ID, secret, "rss", time.Now()); err == nil {
		t.Fatal("removed reader retained access")
	}
	put(t, s, Source{ID: "source", Owner: "alice", Readers: []string{"bob"}})
	if _, _, err := s.ResolveGrant(g.ID, secret, "rss", time.Now()); err == nil {
		t.Fatal("re-adding reader resurrected grant")
	}
}

func TestStoreRejectsCorruptionPermissionsAndConcurrentOwner(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "access.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Open(path); err == nil {
		t.Fatal("second writer accepted")
	}
	_ = s.Close()
	if err = os.WriteFile(path, []byte(`{"version":1,"principals":{},"sources":{},"grants":{},"unknown":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Open(path); err == nil {
		t.Fatal("unknown schema accepted")
	}
	if err = os.WriteFile(path, []byte(`{"version":1,"principals":{},"sources":{},"grants":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = Open(path); err == nil {
		t.Fatal("publicly readable file accepted")
	}
	_ = os.Chmod(path, 0600)
	if err = os.WriteFile(path, []byte(`{"version":1,"principals":{},"sources":{},"grants":{}} {}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Open(path); err == nil {
		t.Fatal("trailing JSON accepted")
	}
}

func TestStoreFailedTransactionAndConcurrentReaders(t *testing.T) {
	s := testStore(t)
	put(t, s, Source{ID: "source", Owner: "alice"})
	version, _ := s.CanRead("alice", "source")
	if err := s.PutSource(Source{ID: "source", Owner: "bad user"}); err == nil {
		t.Fatal("invalid record accepted")
	}
	if v, _ := s.CanRead("alice", "source"); v != version {
		t.Fatal("failed transaction advanced")
	}
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 30 {
				s.CanRead("alice", "source")
				s.Sources("alice")
			}
		}()
	}
	for range 5 {
		if err := s.SetPrincipal("bob", false); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
	var state state
	body, _ := os.ReadFile(s.path)
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatal(err)
	}
	if err := validate(state); err != nil {
		t.Fatal(err)
	}
	// Readers receive copies rather than slices that can mutate authorization.
	put(t, s, Source{ID: "source", Owner: "alice", Readers: []string{"bob"}})
	list := s.Sources("alice")
	list[0].Readers[0] = "mallory"
	if _, ok := s.CanRead("mallory", "source"); ok {
		t.Fatal("mutable permission slice escaped")
	}
}

func TestGrantInputBoundaries(t *testing.T) {
	s := testStore(t)
	put(t, s, Source{ID: "source", Owner: "alice"})
	for _, formats := range [][]string{nil, {"rss", "rss"}, {"xml"}} {
		if _, _, err := s.CreateGrant("alice", "source", formats, time.Now().Add(time.Hour)); err == nil {
			t.Fatal("invalid formats accepted")
		}
	}
	if _, _, err := s.CreateGrant("mallory", "source", []string{"rss"}, time.Now().Add(time.Hour)); !errors.Is(err, ErrDenied) {
		t.Fatal("unauthorized grant created")
	}
	if _, _, err := s.CreateGrant("alice", "source", []string{"rss"}, time.Now().Add(400*24*time.Hour)); err == nil {
		t.Fatal("unbounded token accepted")
	}
}

func TestIssuerBindingAndImmutableInput(t *testing.T) {
	s := testStore(t)
	if err := s.BindIssuer("https://warden.example"); err != nil {
		t.Fatal(err)
	}
	if err := s.BindIssuer("https://another.example"); err == nil {
		t.Fatal("directory issuer rebound")
	}
	src := Source{ID: "source", Owner: "alice", Readers: []string{"bob"}}
	put(t, s, src)
	src.Readers[0] = "mallory"
	if _, ok := s.CanRead("mallory", "source"); ok {
		t.Fatal("mutable ACL input escaped")
	}
	g, secret, err := s.CreateGrant("alice", "source", []string{"rss"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	g.Formats[0] = "json"
	if _, _, err := s.ResolveGrant(g.ID, secret, "json", time.Now()); err == nil {
		t.Fatal("mutable grant formats escaped")
	}
	path := s.path
	_ = s.Close()
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.BindIssuer("https://another.example"); err == nil {
		t.Fatal("issuer binding lost on reopen")
	}
}
