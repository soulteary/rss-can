package parser

import "testing"

func TestBrowserOptionsAreScoped(t *testing.T) {
	first := WithBrowserOptions(t.Context(), BrowserOptions{Debug: true})
	second := WithBrowserOptions(t.Context(), BrowserOptions{IgnoreCertificateErrors: true})
	if got := browserOptions(first); !got.Debug || got.IgnoreCertificateErrors {
		t.Fatalf("first scope %v", got)
	}
	if got := browserOptions(second); got.Debug || !got.IgnoreCertificateErrors {
		t.Fatalf("second scope %v", got)
	}
	if browserOptions(t.Context()) != (BrowserOptions{IgnoreCertificateErrors: true}) {
		t.Fatal("legacy defaults changed")
	}
}
