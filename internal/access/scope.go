package access

import (
	"context"
	"strings"
)

type Scope struct {
	Key     string
	Version uint64
}
type scopeKey struct{}

func WithScope(ctx context.Context, key string, version uint64) context.Context {
	return context.WithValue(ctx, scopeKey{}, Scope{Key: strings.Clone(key), Version: version})
}
func ScopeFromContext(ctx context.Context) Scope { s, _ := ctx.Value(scopeKey{}).(Scope); return s }
