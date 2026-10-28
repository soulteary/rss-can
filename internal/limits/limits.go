// Package limits defines fixed resource budgets shared across generation stages.
package limits

const (
	MaxPageBytes = 10 << 20
	// JSON strings can expand one body byte into six escaped bytes.
	MaxDocumentCacheBytes = 6*MaxPageBytes + (1 << 20)
	MaxDocumentURLBytes   = 64 << 10
	MaxFeedBytes          = 10 << 20
	MaxItems              = 1000
	MaxPages              = 20
	MaxJSMessageBytes     = 16 << 20
)
