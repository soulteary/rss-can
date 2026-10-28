package cacher

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/soulteary/RSS-Can/internal/telemetry"
)

// Backend is the storage contract used by Service. A missing key is reported
// separately from an empty value so callers can safely cache empty strings.
type Backend interface {
	Get(context.Context, string) (string, bool, error)
	Set(context.Context, string, string, time.Duration) error
	Delete(context.Context, string) error
	Close() error
}

// PrimaryOperationTimeout leaves most of the feed deadline for fetching when
// Redis is slow. PrimaryRetryInterval bounds repeat attempts during an outage.
const PrimaryOperationTimeout = 100 * time.Millisecond
const PrimaryRetryInterval = time.Second

var errPrimaryCircuitOpen = errors.New("primary cache circuit is open")

// Service applies the application's Redis-first, memory-fallback policy.
// The enabled callbacks deliberately remain dynamic for compatibility with the
// existing runtime configuration and tests.
type Service struct {
	primaryEnabled  func() bool
	fallbackEnabled func() bool
	primary         Backend
	fallback        Backend

	breakerMu      sync.Mutex
	openUntil      time.Time
	probing        bool
	epoch          uint64
	now            func() time.Time
	primaryTimeout time.Duration
	retryInterval  time.Duration
}

func NewService(primary Backend, fallback Backend, primaryEnabled, fallbackEnabled func() bool) *Service {
	return &Service{
		primaryEnabled:  primaryEnabled,
		fallbackEnabled: fallbackEnabled,
		primary:         primary,
		fallback:        fallback,
		now:             time.Now,
		primaryTimeout:  PrimaryOperationTimeout,
		retryInterval:   PrimaryRetryInterval,
	}
}

// primaryCall permits one half-open probe after the retry interval. A caller
// cancellation is not evidence of an unhealthy Redis server. Epochs prevent an
// older concurrent success from closing a newer failed circuit.
func (s *Service) primaryCall(ctx context.Context, fn func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.breakerMu.Lock()
	if !s.openUntil.IsZero() && (s.now().Before(s.openUntil) || s.probing) {
		s.breakerMu.Unlock()
		return errPrimaryCircuitOpen
	}
	probe := !s.openUntil.IsZero()
	if probe {
		s.probing = true
	}
	epoch := s.epoch
	s.breakerMu.Unlock()
	work, cancel := context.WithTimeout(ctx, s.primaryTimeout)
	defer cancel()
	err := fn(work)
	if err == nil {
		err = work.Err()
	}
	s.breakerMu.Lock()
	defer s.breakerMu.Unlock()
	if epoch == s.epoch {
		if ctx.Err() != nil {
			if probe {
				s.probing = false
			}
		} else if err != nil {
			s.epoch++
			s.openUntil = s.now().Add(s.retryInterval)
			s.probing = false
		} else if probe {
			s.epoch++
			s.openUntil = time.Time{}
			s.probing = false
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
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
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	if s.primaryEnabled() {
		var value string
		var found bool
		err := s.primaryCall(ctx, func(work context.Context) error {
			var err error
			value, found, err = s.primary.Get(work, key)
			return err
		})
		if err == nil {
			return value, found, nil
		}
		if ctx.Err() != nil {
			return "", false, ctx.Err()
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
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.primaryEnabled() {
		if err := s.primaryCall(ctx, func(work context.Context) error { return s.primary.Set(work, key, value, ttl) }); err == nil {
			// Memory is an outage cache, not a warmed second level. Invalidate
			// any value left by an earlier outage after a successful Redis write.
			if s.fallbackEnabled() {
				return s.fallback.Delete(ctx, key)
			}
			return nil
		} else if ctx.Err() != nil {
			return ctx.Err()
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
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.primaryEnabled() {
		if err := s.primaryCall(ctx, func(work context.Context) error { return s.primary.Delete(work, key) }); err == nil {
			if s.fallbackEnabled() {
				return s.fallback.Delete(ctx, key)
			}
			return nil
		} else if ctx.Err() != nil {
			return ctx.Err()
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
