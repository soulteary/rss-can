package feedservice

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/limits"
)

func TestGenerationAdmissionRejectsOverloadAndCancelsWaiter(t *testing.T) {
	a := newAdmission(1, 1)
	release, err := a.acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { _, err := a.acquire(ctx); done <- err }()
	deadline := time.Now().Add(time.Second)
	for len(a.admitted) != 2 {
		if time.Now().After(deadline) {
			t.Fatal("waiter not admitted")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := a.acquire(t.Context()); !errors.Is(err, ErrOverloaded) {
		t.Fatalf("overload: %v", err)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	release()
	release()
	again, err := a.acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	again()
	if len(a.admitted) != 0 || len(a.active) != 0 {
		t.Fatal("admission leaked")
	}
}

func TestServiceAdmissionCoversWholeGeneration(t *testing.T) {
	var calls []string
	d := fixture(&calls)
	d.Serialize = func(context.Context, define.JavaScriptConfig, define.BodyParsed, string) (string, error) {
		return strings.Repeat("x", limits.MaxFeedBytes+1), nil
	}
	service := New(time.Second, d)
	service.admission = newAdmission(1, 0)
	release, _ := service.admission.acquire(t.Context())
	_, err := service.Generate(t.Context(), "site", "rss")
	var failure *Error
	if !errors.As(err, &failure) || failure.Stage != Admission || len(calls) != 0 {
		t.Fatalf("%v %v", err, calls)
	}
	release()
	if body, err := service.Generate(t.Context(), "site", "rss"); body != "" || !errors.As(err, &failure) || failure.Stage != Serialization {
		t.Fatalf("%q %v", body, err)
	}
	if len(service.admission.active) != 0 {
		t.Fatal("serialization failure leaked slot")
	}
}

func TestServiceRejectsOversizedFetchBeforeSerialization(t *testing.T) {
	var calls []string
	d := fixture(&calls)
	d.Fetch = func(context.Context, define.JavaScriptConfig) define.BodyParsed {
		return define.BodyParsed{Body: make([]define.InfoItem, limits.MaxItems+1)}
	}
	_, err := New(time.Second, d).Generate(t.Context(), "site", "rss")
	var failure *Error
	if !errors.As(err, &failure) || failure.Stage != Fetch || len(calls) != 2 {
		t.Fatalf("%v %v", err, calls)
	}
}
