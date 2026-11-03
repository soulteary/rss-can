package feedservice

import (
	"context"
	"errors"
	"testing"
	"time"
)

func waitQueued(t *testing.T, a *fairAdmission, n int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		a.mu.Lock()
		queued := a.queued
		a.mu.Unlock()
		if queued == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("queue=%d want=%d", queued, n)
		}
		time.Sleep(time.Millisecond)
	}
}
func TestFairAdmissionLetsOtherOwnersProgress(t *testing.T) {
	a := newFairAdmission(2, 4, 1, 2)
	first, err := a.acquire(t.Context(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	alice := make(chan func(), 1)
	go func() { release, _ := a.acquire(t.Context(), "alice"); alice <- release }()
	waitQueued(t, a, 1)
	bob, err := a.acquire(t.Context(), "bob")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-alice:
		t.Fatal("one owner exceeded its active limit")
	default:
	}
	bob()
	first()
	next := <-alice
	next()
	next()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.active != 0 || a.queued != 0 || len(a.owners) != 0 || len(a.order) != 0 {
		t.Fatal("fair admission leaked")
	}
}
func TestFairAdmissionBoundsOwnerQueueAndCancellation(t *testing.T) {
	a := newFairAdmission(1, 2, 1, 1)
	first, _ := a.acquire(t.Context(), "alice")
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() { _, err := a.acquire(ctx, "alice"); result <- err }()
	waitQueued(t, a, 1)
	if _, err := a.acquire(t.Context(), "alice"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("owner flood=%v", err)
	}
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	waitQueued(t, a, 0)
	first()
	again, err := a.acquire(t.Context(), "bob")
	if err != nil {
		t.Fatal(err)
	}
	again()
}
func TestFairAdmissionRoundRobinDoesNotRewardFlooding(t *testing.T) {
	a := newFairAdmission(1, 8, 1, 4)
	active, _ := a.acquire(t.Context(), "blocker")
	type grant struct {
		owner   string
		release func()
	}
	out := make(chan grant, 4)
	for i, owner := range []string{"alice", "alice", "bob", "bob"} {
		go func(owner string) {
			release, err := a.acquire(t.Context(), owner)
			if err != nil {
				return
			}
			out <- grant{owner, release}
		}(owner)
		waitQueued(t, a, i+1)
	}
	waitQueued(t, a, 4)
	active()
	previous := ""
	for range 4 {
		g := <-out
		if previous == g.owner {
			t.Fatal("owner received consecutive turns while another waited")
		}
		previous = g.owner
		g.release()
	}
}
