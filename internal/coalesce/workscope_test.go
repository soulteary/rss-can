package coalesce

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/workscope"
)

func TestCanceledWaiterLeavesCleanupJoinedByOwner(t *testing.T) {
	var group Group[int]
	var owner workscope.Tracker
	ctx, cancel := context.WithCancel(workscope.WithTracker(t.Context(), &owner))
	entered, cleanup, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	callerDone := make(chan error, 1)
	go func() {
		_, err, _ := group.Do(ctx, "key", time.Second, func(work context.Context) (int, error) {
			close(entered)
			<-work.Done()
			close(cleanup)
			<-release
			return 0, work.Err()
		})
		callerDone <- err
	}()
	<-entered
	cancel()
	select {
	case err := <-callerDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("caller error %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("waiter blocked on callback cleanup")
	}
	<-cleanup
	ownerDone := make(chan struct{})
	go func() { owner.Wait(); close(ownerDone) }()
	select {
	case <-ownerDone:
		t.Fatal("owner closed before cleanup finished")
	default:
	}
	close(release)
	select {
	case <-ownerDone:
	case <-time.After(time.Second):
		t.Fatal("owner failed to join shared cleanup")
	}
}
