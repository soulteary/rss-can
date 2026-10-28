package network

import (
	"context"
	"fmt"
	"time"

	"github.com/soulteary/RSS-Can/internal/coalesce"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
)

// Config captures application network settings. In-flight work is owned by the
// Client, so separate applications cannot coalesce requests across boundaries.
type Config struct {
	RequestTimeout, ServerTimeout, DefaultTTL time.Duration
	UserAgent                                 string
	Proxy                                     string
}
type Client struct {
	config  Config
	flights coalesce.Group[define.RemoteBodySanitized]
}

func NewClient(config Config) *Client {
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 5 * time.Second
	}
	if config.ServerTimeout <= 0 {
		config.ServerTimeout = 8 * time.Second
	}
	if config.DefaultTTL <= 0 {
		config.DefaultTTL = 10 * time.Minute
	}
	if config.UserAgent == "" {
		config.UserAgent = define.DEFAULT_USER_AGENT
	}
	return &Client{config: config}
}

type clientContextKey struct{}

// WithClient is the gradual migration bridge used by existing parser functions.
func WithClient(ctx context.Context, client *Client) context.Context {
	return context.WithValue(ctx, clientContextKey{}, client)
}
func scopedClient(ctx context.Context) *Client {
	client, _ := ctx.Value(clientContextKey{}).(*Client)
	return client
}
func configForContext(ctx context.Context) Config {
	if client := scopedClient(ctx); client != nil {
		return client.config
	}
	return Config{RequestTimeout: fn.ExpireBySecond(define.REQUEST_TIMEOUT), ServerTimeout: fn.ExpireBySecond(define.SERVER_TIMEOUT), DefaultTTL: fn.ExpireBySecond(define.IN_MEMORY_EXPIRATION), UserAgent: define.USER_AGENT}
}

// Scope separates browser and other shared-work keys across client instances.
func Scope(ctx context.Context) string {
	if client := scopedClient(ctx); client != nil {
		return fmt.Sprintf("%p", client)
	}
	return "legacy"
}
