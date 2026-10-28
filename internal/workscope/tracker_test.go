package workscope

import (
	"testing"
	"time"
)

func TestTrackersOwnOnlyTheirCallbacks(t *testing.T) {
	var first, second Tracker
	entered, release := make(chan struct{}), make(chan struct{})
	Go(WithTracker(t.Context(), &first), func() { close(entered); <-release })
	<-entered
	Go(WithTracker(t.Context(), &second), func() {})
	secondDone := make(chan struct{})
	go func() { second.Wait(); close(secondDone) }()
	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("independent tracker blocked")
	}
	firstDone := make(chan struct{})
	go func() { first.Wait(); close(firstDone) }()
	select {
	case <-firstDone:
		t.Fatal("tracker ignored active callback")
	default:
	}
	close(release)
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("tracker failed to join")
	}
}

func TestLegacyAndNestedCallbacks(t *testing.T) {
	legacy := make(chan struct{})
	Go(t.Context(), func() { close(legacy) })
	select {
	case <-legacy:
	case <-time.After(time.Second):
		t.Fatal("legacy callback did not run")
	}
	var owner Tracker
	ctx := WithTracker(t.Context(), &owner)
	childDone := make(chan struct{})
	Go(ctx, func() { Go(ctx, func() { close(childDone) }) })
	owner.Wait()
	select {
	case <-childDone:
	default:
		t.Fatal("nested callback was not joined")
	}
}
