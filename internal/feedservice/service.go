// Package feedservice orchestrates feed generation independently of HTTP.
package feedservice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/generator"
	"github.com/soulteary/RSS-Can/internal/rule"
)

type Stage string

const (
	InvalidFormat Stage = "format"
	MissingRule   Stage = "lookup"
	Configuration Stage = "configuration"
	Fetch         Stage = "fetch"
	Serialization Stage = "serialization"
	Timeout       Stage = "timeout"
)

// Error preserves an underlying cause while keeping transport policy outside the service.
type Error struct {
	Stage Stage
	Code  define.ErrorCode
	Cause error
}

func (e *Error) Error() string { return fmt.Sprintf("feed %s: %v", e.Stage, e.Cause) }
func (e *Error) Unwrap() error { return e.Cause }

// Dependencies are fixed when the service is constructed. Tests can substitute
// each boundary without a JavaScript process, cache server or listening socket.
type Dependencies struct {
	Lookup    func(string) (define.RuleCache, bool)
	Configure func(context.Context, define.RuleCache) (define.JavaScriptConfig, error)
	Fetch     func(context.Context, define.JavaScriptConfig) define.BodyParsed
	Serialize func(context.Context, define.JavaScriptConfig, define.BodyParsed, string) (string, error)
}
type Service struct {
	dependencies Dependencies
	timeout      time.Duration
}

func New(timeout time.Duration, dependencies Dependencies) *Service {
	if timeout <= 0 {
		panic("feedservice: timeout must be positive")
	}
	if dependencies.Lookup == nil || dependencies.Configure == nil || dependencies.Fetch == nil || dependencies.Serialize == nil {
		panic("feedservice: missing dependency")
	}
	return &Service{dependencies: dependencies, timeout: timeout}
}

// NewDefault adapts the existing runtime modules during gradual migration.
// Callers own initialization and shutdown of those modules.
func NewDefault(timeout time.Duration) *Service {
	return New(timeout, Dependencies{Lookup: rule.Lookup, Configure: rule.GenerateConfigByRuleContext, Fetch: rule.GetWebsiteDataWithConfigContext, Serialize: generator.GenerateFeedsByTypeContext})
}

func (s *Service) Generate(ctx context.Context, id, format string) (string, error) {
	format = strings.ToLower(format)
	switch format {
	case define.FEED_TYPE_RSS, define.FEED_TYPE_ATOM, define.FEED_TYPE_JSON:
	default:
		return "", &Error{Stage: InvalidFormat, Cause: errors.New("unsupported feed type")}
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	deadline := func() error {
		if err := ctx.Err(); err != nil {
			return &Error{Stage: Timeout, Cause: err}
		}
		return nil
	}
	if err := deadline(); err != nil {
		return "", err
	}
	snapshot, found := s.dependencies.Lookup(id)
	if !found {
		return "", &Error{Stage: MissingRule, Cause: errors.New("rule not found")}
	}
	config, err := s.dependencies.Configure(ctx, snapshot)
	if canceled := deadline(); canceled != nil {
		return "", canceled
	}
	if err != nil {
		return "", &Error{Stage: Configuration, Cause: err}
	}
	data := s.dependencies.Fetch(ctx, config)
	if err := deadline(); err != nil {
		return "", err
	}
	if data.Code != define.ERROR_CODE_NULL {
		return "", &Error{Stage: Fetch, Code: data.Code, Cause: errors.New(data.Status)}
	}
	body, err := s.dependencies.Serialize(ctx, config, data, format)
	if canceled := deadline(); canceled != nil {
		return "", canceled
	}
	if err != nil {
		return "", &Error{Stage: Serialization, Cause: err}
	}
	return body, nil
}
