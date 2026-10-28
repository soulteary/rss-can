package cacher

import (
	"context"
	"errors"
	"github.com/soulteary/RSS-Can/internal/telemetry"
	"time"
)

// Backend is the storage contract used by Service. A missing key is reported
// separately from an empty value so callers can safely cache empty strings.
type Backend interface {
	Get(context.Context, string) (string, bool, error)
	Set(context.Context, string, string, time.Duration) error
	Delete(context.Context, string) error
	Close() error
}

// Service applies the application's Redis-first, memory-fallback policy.
// The enabled callbacks deliberately remain dynamic for compatibility with the
// existing runtime configuration and tests.
type Service struct {
	primaryEnabled  func() bool
	fallbackEnabled func() bool
	primary         Backend
	fallback        Backend
}

func NewService(primary Backend, fallback Backend, primaryEnabled, fallbackEnabled func() bool) *Service {
	return &Service{
		primaryEnabled:  primaryEnabled,
		fallbackEnabled: fallbackEnabled,
		primary:         primary,
		fallback:        fallback,
	}
}

func (s *Service) Enabled() bool {
	return s.primaryEnabled() || s.fallbackEnabled()
}

func (s *Service) Get(ctx context.Context, key string) (value string, found bool, err error) {
	defer func() {
		if err != nil {
			telemetry.Default.Add("cache_errors_total", 1)
		} else if found {
			telemetry.Default.Add("cache_hits_total", 1)
		} else {
			telemetry.Default.Add("cache_misses_total", 1)
		}
	}()
	if s.primaryEnabled() {
		value, found, err := s.primary.Get(ctx, key)
		if err == nil {
			return value, found, nil
		}
		if !s.fallbackEnabled() {
			return "", false, err
		}
	}
	if s.fallbackEnabled() {
		if s.primaryEnabled() {
			telemetry.Default.Add("cache_fallbacks_total", 1)
			telemetry.Default.Add("cache_errors_total", 1)
		}
		return s.fallback.Get(ctx, key)
	}
	return "", false, noAlivedBackend
}

func (s *Service) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if ttl < 0 {
		return errors.New("cache TTL must not be negative")
	}
	if s.primaryEnabled() {
		if err := s.primary.Set(ctx, key, value, ttl); err == nil {
			return nil
		} else if !s.fallbackEnabled() {
			return err
		}
	}
	if s.fallbackEnabled() {
		return s.fallback.Set(ctx, key, value, ttl)
	}
	return noAlivedBackend
}

func (s *Service) Delete(ctx context.Context, key string) error {
	if s.primaryEnabled() {
		if err := s.primary.Delete(ctx, key); err == nil {
			return nil
		} else if !s.fallbackEnabled() {
			return err
		}
	}
	if s.fallbackEnabled() {
		return s.fallback.Delete(ctx, key)
	}
	return noAlivedBackend
}

// Close releases both backends. errors.Join ensures that a failure closing one
// backend never prevents the other backend from being released.
func (s *Service) Close() error {
	return errors.Join(s.primary.Close(), s.fallback.Close())
}
