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
	if err == nil || ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("Redis ignored caller deadline: %v, %v", err, ctx.Err())
	}
	if elapsed := time.Since(began); elapsed > time.Second {
		t.Fatalf("Redis blocked past request budget: %v", elapsed)
	}
	client.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Redis connection not released")
	}
}
