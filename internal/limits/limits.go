// Package limits defines fixed resource budgets shared across generation stages.
package limits

const (
	MaxPageBytes      = 10 << 20
	MaxFeedBytes      = 10 << 20
	MaxItems          = 1000
	MaxPages          = 20
	MaxJSMessageBytes = 16 << 20
)
