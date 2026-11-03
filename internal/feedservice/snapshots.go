package feedservice

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/soulteary/RSS-Can/internal/access"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/generator"
	"github.com/soulteary/RSS-Can/internal/limits"
	"github.com/soulteary/RSS-Can/internal/telemetry"
)

// Representation is returned only after the caller's authorization middleware.
// It contains no authorization decision: every request must authorize again.
type Representation struct {
	Body, ETag           string
	ChangedAt, FetchedAt time.Time
	Stale                bool
}
type SnapshotOptions struct {
	TTL, MaxStale, RetryInterval time.Duration
	MaxEntries                   int
	MaxBytes                     int64
}
type snapshotEntry struct {
	value            Representation
	done             chan struct{}
	err              error
	retryAt, touched time.Time
	waiters          int
	failures         int
	cancel           context.CancelFunc
}

// Snapshots owns bounded in-memory representations and refresh lifetimes.
// Only completed successful output is published. Keys include the captured
// rule, evaluated configuration and authorization epoch, not a download token.
type Snapshots struct {
	service *Service
	options SnapshotOptions
	life    context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	closed  bool
	entries map[[32]byte]*snapshotEntry
	bytes   int64
	work    sync.WaitGroup
	fair    *fairAdmission
	prepare *fairAdmission
	now     func() time.Time
}

func NewSnapshots(parent context.Context, service *Service, options SnapshotOptions) (*Snapshots, error) {
	if service == nil || options.TTL <= 0 || options.MaxStale < 0 || options.RetryInterval < 0 || options.MaxEntries < 0 || options.MaxBytes < 0 {
		return nil, errors.New("invalid feed snapshot configuration")
	}
	if options.MaxStale > time.Duration(1<<63-1)-options.TTL {
		return nil, errors.New("feed snapshot lifetime overflows duration")
	}
	if options.RetryInterval == 0 {
		options.RetryInterval = time.Second * 5
	}
	if options.MaxEntries == 0 {
		options.MaxEntries = 256
	}
	if options.MaxBytes == 0 {
		options.MaxBytes = 64 << 20
	}
	life, cancel := context.WithCancel(parent)
	return &Snapshots{service: service, options: options, life: life, cancel: cancel, entries: make(map[[32]byte]*snapshotEntry), fair: newFairAdmission(GenerationConcurrency, GenerationQueueLimit, 2, 8), prepare: newFairAdmission(4, GenerationQueueLimit, 2, 8), now: time.Now}, nil
}

func (s *Snapshots) Generate(ctx context.Context, id, format string) (string, error) {
	v, err := s.Representation(ctx, id, format)
	return v.Body, err
}

func (s *Snapshots) Representation(ctx context.Context, id, format string) (Representation, error) {
	return s.representation(ctx, id, format, false)
}

// Refresh requests a new complete representation while retaining all scope,
// queue and retry limits. Callers must authorize the write action separately.
func (s *Snapshots) Refresh(ctx context.Context, id, format string) (Representation, error) {
	return s.representation(ctx, id, format, true)
}

func (s *Snapshots) representation(ctx context.Context, id, format string, force bool) (Representation, error) {
	if err := ctx.Err(); err != nil {
		return Representation{}, &Error{Stage: Timeout, Cause: err}
	}
	if s.life.Err() != nil {
		return Representation{}, errors.New("feed snapshots are closed")
	}
	ctx, requestCancel := context.WithTimeout(ctx, s.service.timeout)
	defer requestCancel()
	format = strings.ToLower(format)
	if format != define.FEED_TYPE_RSS && format != define.FEED_TYPE_ATOM && format != define.FEED_TYPE_JSON {
		return Representation{}, &Error{Stage: InvalidFormat, Cause: errors.New("unsupported feed type")}
	}
	// Resolve before reading a snapshot. Dynamic rules and credentials still
	// take effect immediately, including turning DisableCache on.
	resolve, cancel := context.WithTimeout(ctx, s.service.timeout)
	defer cancel()
	scope := access.ScopeFromContext(ctx)
	owner := scope.Key
	if before, _, ok := strings.Cut(scope.Key, "\x00"); ok && before != "" {
		owner = before
	}
	if owner == "" {
		owner = "public:" + id
	}
	began := time.Now()
	// Configuration admission is separate: a slow refresh must not prevent
	// an owner from reading already-generated representations.
	release, err := s.prepare.acquire(resolve, owner)
	telemetry.Default.Observe("snapshot_queue_seconds", time.Since(began))
	if err != nil {
		if resolve.Err() != nil {
			return Representation{}, &Error{Stage: Timeout, Cause: resolve.Err()}
		}
		stage := Admission
		if errors.Is(err, ErrRateLimited) {
			stage = RateLimited
		}
		return Representation{}, &Error{Stage: stage, Cause: err}
	}
	rule, found := s.service.dependencies.Lookup(id)
	if !found {
		release()
		return Representation{}, &Error{Stage: MissingRule, Cause: errors.New("rule not found")}
	}
	config, err := s.service.dependencies.Configure(resolve, rule)
	release()
	if resolve.Err() != nil {
		return Representation{}, &Error{Stage: Timeout, Cause: resolve.Err()}
	}
	if err != nil {
		return Representation{}, &Error{Stage: Configuration, Cause: err}
	}
	if config.DisableCache {
		body, err := s.run(resolve, owner, config, format, time.Time{}, "")
		return Representation{Body: body}, err
	}
	encoded, err := json.Marshal(struct {
		ID, Format string
		Scope      access.Scope
		RuleHash   [32]byte
		Config     define.JavaScriptConfig
	}{id, format, scope, sha256.Sum256(rule.Body), config})
	if err != nil {
		return Representation{}, &Error{Stage: Configuration, Cause: err}
	}
	key := sha256.Sum256(encoded)
	ttl := s.options.TTL
	if config.Expire > 0 && time.Duration(config.Expire) <= ttl/time.Second {
		ttl = min(ttl, time.Duration(config.Expire)*time.Second)
	}
	now := s.now()
	s.mu.Lock()
	if s.closed || s.life.Err() != nil {
		s.mu.Unlock()
		return Representation{}, errors.New("feed snapshots are closed")
	}
	e := s.entries[key]
	if e == nil {
		if !s.makeRoom(1, 0, nil) {
			s.mu.Unlock()
			return Representation{}, &Error{Stage: Admission, Cause: ErrOverloaded}
		}
		e = &snapshotEntry{touched: now}
		s.entries[key] = e
	}
	e.touched = now
	hasValue := !e.value.FetchedAt.IsZero()
	fresh := hasValue && now.Before(e.value.FetchedAt.Add(ttl))
	staleAllowed := hasValue && now.Before(e.value.FetchedAt.Add(ttl+s.options.MaxStale))
	if fresh && !force {
		value := e.value
		s.mu.Unlock()
		telemetry.Default.Add("snapshot_hits_total", 1)
		return value, nil
	}
	if e.done == nil && e.waiters == 0 && !now.Before(e.retryAt) {
		e.done = make(chan struct{})
		e.err = nil
		work, stop := context.WithCancel(context.WithoutCancel(ctx))
		stopLife := context.AfterFunc(s.life, stop)
		e.cancel = stop
		s.work.Add(1)
		go func() {
			defer s.work.Done()
			defer stop()
			defer stopLife()
			s.refresh(work, key, e, owner, config, format)
		}()
	}
	if staleAllowed && !force {
		value := e.value
		value.Stale = true
		s.mu.Unlock()
		telemetry.Default.Add("snapshot_stale_total", 1)
		return value, nil
	}
	if e.done == nil {
		err := e.err
		s.mu.Unlock()
		if err == nil {
			err = ErrOverloaded
		}
		return Representation{}, err
	}
	done := e.done
	e.waiters++
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		e.waiters--
		if e.waiters == 0 && e.done != nil && e.value.FetchedAt.IsZero() {
			e.cancel()
		}
		s.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return Representation{}, &Error{Stage: Timeout, Cause: ctx.Err()}
	case <-done:
		if ctx.Err() != nil {
			return Representation{}, &Error{Stage: Timeout, Cause: ctx.Err()}
		}
		s.mu.Lock()
		value, err := e.value, e.err
		s.mu.Unlock()
		if err != nil {
			return Representation{}, err
		}
		return value, nil
	}
}

// makeRoom is called with mu held and never evicts running work.
func (s *Snapshots) makeRoom(extraEntries int, extraBytes int64, keep *snapshotEntry) bool {
	for len(s.entries)+extraEntries > s.options.MaxEntries || s.bytes+extraBytes > s.options.MaxBytes {
		var oldest *snapshotEntry
		var oldestKey [32]byte
		for key, e := range s.entries {
			if e == keep || e.done != nil || e.waiters > 0 {
				continue
			}
			if oldest == nil || e.touched.Before(oldest.touched) {
				oldest = e
				oldestKey = key
			}
		}
		if oldest == nil {
			return false
		}
		s.bytes -= int64(len(oldest.value.Body))
		delete(s.entries, oldestKey)
	}
	return true
}

func (s *Snapshots) refresh(ctx context.Context, key [32]byte, entry *snapshotEntry, owner string, config define.JavaScriptConfig, format string) {
	s.mu.Lock()
	previous := entry.value
	s.mu.Unlock()
	now := s.now()
	body, err := s.runSafely(ctx, owner, config, format, previous.ChangedAt, previous.Body)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil && (s.closed || s.life.Err() != nil || ctx.Err() != nil) {
		err = context.Canceled
	}
	if err == nil {
		cost := int64(len(body) - len(entry.value.Body))
		if !s.makeRoom(0, cost, entry) {
			err = &Error{Stage: Admission, Cause: errors.New("feed snapshot capacity exhausted")}
		} else {
			changed := now
			if body == previous.Body && !previous.ChangedAt.IsZero() {
				changed = previous.ChangedAt
			}
			s.bytes += cost
			entry.value = Representation{Body: body, ETag: fmt.Sprintf("W/\"%x\"", sha256.Sum256([]byte(body))), ChangedAt: changed, FetchedAt: s.now()}
			entry.retryAt = time.Time{}
			entry.failures = 0
			telemetry.Default.Add("snapshot_refreshes_total", 1)
		}
	}
	if err != nil {
		if errors.Is(err, context.Canceled) {
			entry.retryAt = time.Time{}
		} else {
			entry.failures++
			backoff := min(s.options.RetryInterval, time.Hour) * time.Duration(1<<min(entry.failures-1, 5))
			// Stable key jitter spreads retries without requiring a shared RNG.
			backoff = backoff * time.Duration(800+int(key[0])) / 1000
			entry.retryAt = s.now().Add(backoff)
		}
		telemetry.Default.Add("snapshot_failures_total", 1)
	}
	entry.err = err
	close(entry.done)
	entry.done = nil
	entry.cancel = nil
}

// Refresh executes outside the HTTP recovery middleware. A dependency panic
// must finish this task, release its waiters and leave the process available.
// Panic values are not exposed: they may contain source data or credentials.
func (s *Snapshots) runSafely(ctx context.Context, owner string, config define.JavaScriptConfig, format string, previousTime time.Time, previousBody string) (body string, err error) {
	defer func() {
		if recover() != nil {
			body = ""
			err = &Error{Stage: Serialization, Cause: errors.New("feed refresh failed unexpectedly")}
		}
	}()
	return s.run(ctx, owner, config, format, previousTime, previousBody)
}

func (s *Snapshots) run(ctx context.Context, owner string, config define.JavaScriptConfig, format string, previousTime time.Time, previousBody string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, s.service.timeout)
	defer cancel()
	began := time.Now()
	release, err := s.fair.acquire(ctx, owner)
	telemetry.Default.Observe("snapshot_queue_seconds", time.Since(began))
	if err != nil {
		if ctx.Err() != nil {
			return "", &Error{Stage: Timeout, Cause: ctx.Err()}
		}
		stage := Admission
		if errors.Is(err, ErrRateLimited) {
			stage = RateLimited
		}
		return "", &Error{Stage: stage, Cause: err}
	}
	defer release()
	data := s.service.dependencies.Fetch(ctx, config)
	if ctx.Err() != nil {
		return "", &Error{Stage: Timeout, Cause: ctx.Err()}
	}
	if !limits.ItemsWithinBudget(data.Body) {
		return "", &Error{Stage: Fetch, Cause: errors.New("feed exceeds item or content size limit")}
	}
	if data.Code != define.ERROR_CODE_NULL {
		return "", &Error{Stage: Fetch, Code: data.Code, Cause: errors.New(data.Status)}
	}
	at := previousTime
	if at.IsZero() {
		at = s.now()
	}
	serialize := func(at time.Time) (string, error) {
		body, err := s.service.dependencies.Serialize(generator.WithFeedTime(ctx, at), config, data, format)
		if ctx.Err() != nil {
			return "", &Error{Stage: Timeout, Cause: ctx.Err()}
		}
		if err != nil {
			return "", &Error{Stage: Serialization, Cause: err}
		}
		if len(body) > limits.MaxFeedBytes {
			return "", &Error{Stage: Serialization, Cause: errors.New("feed exceeds output size limit")}
		}
		return body, nil
	}
	body, err := serialize(at)
	if err == nil && !previousTime.IsZero() && body != previousBody {
		return serialize(s.now())
	}
	return body, err
}

func (s *Snapshots) Close() error {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	s.work.Wait()
	return nil
}
