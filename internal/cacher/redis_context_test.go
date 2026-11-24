package cacher

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
)

func TestRedisRespectsCallerDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Accept the connection but never reply to the Redis handshake.
		_, _ = io.Copy(io.Discard, conn)
	}()
	previous := define.REDIS_SERVER
	define.REDIS_SERVER = listener.Addr().String()
	defer func() { define.REDIS_SERVER = previous }()
	client := InitializeRedis(true)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	began := time.Now()
	_, _, err = newRedisBackend(client).Get(ctx, "deadline-test")
	finished := time.Now()
	deadline, _ := ctx.Deadline()
	if err == nil || finished.Before(deadline) {
		t.Fatalf("Redis returned before the caller deadline: %v, elapsed %v", err, finished.Sub(began))
	}
	if elapsed := finished.Sub(began); elapsed > time.Second {
		t.Fatalf("Redis blocked past request budget: %v", elapsed)
	}
	// The socket deadline and context timer expire at the same instant, but
	// their callbacks may run in either order. Check context completion after
	// checking that the Redis operation honored the actual deadline.
	select {
	case <-ctx.Done():
		if ctx.Err() != context.DeadlineExceeded {
			t.Fatalf("Unexpected context result: %v", ctx.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("Caller context did not expire")
	}
	client.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Redis connection not released")
	}
}
