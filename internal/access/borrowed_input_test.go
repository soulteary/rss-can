package access

import (
	"context"
	"testing"
	"time"
	"unsafe"
)

func borrowed(buffer []byte) string { return unsafe.String(unsafe.SliceData(buffer), len(buffer)) }

// Fiber route parameters can borrow a request buffer that is reused after the
// handler. This models that lifetime without depending on a particular pool.
func TestGrantOwnsBorrowedRouteParameters(t *testing.T) {
	s := testStore(t)
	put(t, s, Source{ID: "private", Owner: "alice"})
	buf := []byte("private")
	borrowed := unsafe.String(unsafe.SliceData(buf), len(buf))
	g, secret, err := s.CreateGrant("alice", borrowed, []string{"rss"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	copy(buf, []byte("changed"))
	if _, _, err := s.ResolveGrant(g.ID, secret, "rss", time.Now()); err != nil {
		t.Fatalf("request buffer changed stored grant source: %v", err)
	}
	if err := s.RevokeGrant("alice", g.ID); err != nil {
		t.Fatalf("request reuse made grant impossible to revoke: %v", err)
	}
}

func TestPersistedIdentityAndScopeOwnInputBuffers(t *testing.T) {
	s := testStore(t)
	owner := []byte("alice")
	id := []byte("source")
	reader := []byte("bob")
	if err := s.PutSource(Source{ID: borrowed(id), Owner: borrowed(owner), Readers: []string{borrowed(reader)}}); err != nil {
		t.Fatal(err)
	}
	copy(owner, "other")
	copy(id, "xxxxxx")
	copy(reader, "zzz")
	if _, ok := s.CanRead("alice", "source"); !ok {
		t.Fatal("source owner or ID borrowed caller memory")
	}
	if _, ok := s.CanRead("bob", "source"); !ok {
		t.Fatal("reader borrowed caller memory")
	}
	subject := []byte("eve")
	if err := s.SetPrincipal(borrowed(subject), false); err != nil {
		t.Fatal(err)
	}
	copy(subject, "bad")
	if !s.Active("eve") {
		t.Fatal("principal key borrowed caller memory")
	}
	key := []byte("alice/source")
	ctx := WithScope(context.Background(), borrowed(key), 1)
	copy(key, "badxx/xxxxxx")
	if ScopeFromContext(ctx).Key != "alice/source" {
		t.Fatal("asynchronous scope borrowed caller memory")
	}
}
