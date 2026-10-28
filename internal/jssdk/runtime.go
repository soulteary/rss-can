package jssdk

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// RuntimeConfig is captured at application construction, never read per request.
type RuntimeConfig struct {
	Binary  string
	Workers int
	Timeout time.Duration
}

type Runtime struct {
	mu     sync.RWMutex
	closed bool
	pool   denoPool
	config RuntimeConfig
}

type runtimeContextKey struct{}

// WithRuntime bridges the legacy context-aware SDK functions to an owned runtime.
// Requests never change a process-global default or share workers across owners.
func WithRuntime(ctx context.Context, runtime *Runtime) context.Context {
	return context.WithValue(ctx, runtimeContextKey{}, runtime)
}
func runtimeFromContext(ctx context.Context) *Runtime {
	runtime, _ := ctx.Value(runtimeContextKey{}).(*Runtime)
	return runtime
}
func runtimeBinary(ctx context.Context) string {
	if runtime := runtimeFromContext(ctx); runtime != nil {
		return runtime.config.Binary
	}
	return denoExecutable()
}

func NewRuntime(ctx context.Context, config RuntimeConfig) (*Runtime, error) {
	if config.Workers < 1 || config.Workers > 16 {
		return nil, fmt.Errorf("JavaScript workers must be between 1 and 16")
	}
	if config.Timeout <= 0 {
		return nil, errors.New("JavaScript timeout must be positive")
	}
	if config.Binary == "" {
		config.Binary = denoExecutable()
	}
	binary, err := exec.LookPath(config.Binary)
	if err != nil {
		return nil, fmt.Errorf("locate Deno: %w", err)
	}
	config.Binary = binary
	runtime := &Runtime{config: config}
	// Configure this pool explicitly so it does not consult RSS_JS_WORKERS.
	runtime.pool.once.Do(func() {
		runtime.pool.available = make(chan *denoWorker, config.Workers)
		for range config.Workers {
			worker := &denoWorker{}
			runtime.pool.workers = append(runtime.pool.workers, worker)
			runtime.pool.available <- worker
		}
	})
	startup, cancel := context.WithTimeout(WithRuntime(ctx, runtime), 10*time.Second)
	defer cancel()
	if err := runtime.pool.initialize(startup); err != nil {
		runtime.pool.close()
		return nil, err
	}
	return runtime, nil
}

func (r *Runtime) runCode(ctx context.Context, base, source, file string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		return "", errors.New("JavaScript runtime is closed")
	}
	return r.pool.runContext(WithRuntime(ctx, r), base, source, file, r.config.Timeout)
}
func (r *Runtime) Check(ctx context.Context) error {
	_, err := r.runCode(ctx, "", "0", "readiness.js")
	return err
}
func (r *Runtime) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed {
		r.closed = true
		r.pool.close()
	}
	return nil
}
