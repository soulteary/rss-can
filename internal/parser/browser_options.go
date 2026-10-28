package parser

import (
	"context"

	"github.com/soulteary/RSS-Can/internal/define"
)

// BrowserOptions is captured by the application; public inspector pages always
// ignore these privilege-expanding options.
type BrowserOptions struct {
	Debug                   bool
	IgnoreCertificateErrors bool
}
type browserOptionsKey struct{}

func WithBrowserOptions(ctx context.Context, options BrowserOptions) context.Context {
	return context.WithValue(ctx, browserOptionsKey{}, options)
}
func browserOptions(ctx context.Context) BrowserOptions {
	if options, ok := ctx.Value(browserOptionsKey{}).(BrowserOptions); ok {
		return options
	}
	return BrowserOptions{Debug: define.CSR_DEBUG, IgnoreCertificateErrors: define.CSR_IGNORE_CERT_ERRORS}
}
