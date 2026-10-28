package jssdk

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

// Pool size is read once, so runtime configuration cannot race with execution.
// The default retains the memory footprint of the original single worker.
type denoPool struct {
	mu        sync.RWMutex
	once      sync.Once
	err       error
	workers   []*denoWorker
	available chan *denoWorker
}

func (p *denoPool) configure() error {
	p.once.Do(func() {
		size := 1
		if raw := os.Getenv("RSS_JS_WORKERS"); raw != "" {
			var err error
			size, err = strconv.Atoi(raw)
			if err != nil || size < 1 || size > 16 {
				p.err = fmt.Errorf("RSS_JS_WORKERS must be an integer between 1 and 16")
				return
			}
		}
		p.available = make(chan *denoWorker, size)
		for range size {
			w := &denoWorker{}
			p.workers = append(p.workers, w)
			p.available <- w
		}
	})
	return p.err
}

func (p *denoPool) runContext(ctx context.Context, base, source, filename string, timeout time.Duration) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if err := p.configure(); err != nil {
		return "", err
	}
	return runDeno(ctx, base, source, filename, timeout, func(ctx context.Context) (*denoWorker, func(), error) {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		select {
		case w := <-p.available:
			if err := ctx.Err(); err != nil {
				p.available <- w
				return nil, nil, err
			}
			return w, func() { p.available <- w }, nil
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	})
}

func (p *denoPool) initialize(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.configure(); err != nil {
		return err
	}
	for _, w := range p.workers {
		if _, err := w.runContext(ctx, "", "0", "runtime-init.js", time.Second); err != nil {
			for _, started := range p.workers {
				started.close()
			}
			return fmt.Errorf("initialize Deno worker (install Deno or set RSS_CAN_DENO): %w", err)
		}
	}
	return nil
}

func (p *denoPool) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, w := range p.workers {
		w.close()
	}
}
