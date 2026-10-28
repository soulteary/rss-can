// Package application assembles and owns one independently configured server.
package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/feedservice"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/generator"
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"github.com/soulteary/RSS-Can/internal/network"
	"github.com/soulteary/RSS-Can/internal/parser"
	"github.com/soulteary/RSS-Can/internal/rule"
	"github.com/soulteary/RSS-Can/internal/server"
	"github.com/soulteary/RSS-Can/internal/workscope"
)

type Config struct {
	Server         server.Options
	Cache          cacher.Config
	JavaScript     jssdk.RuntimeConfig
	Network        network.Config
	BrowserTimeout time.Duration
	Browser        parser.BrowserOptions
	ConfigCacheTTL time.Duration
}

// ConfigFromGlobals is a startup-only compatibility boundary for CLI/env options.
// The resulting Config is copied into the application; requests do not reread it.
func ConfigFromGlobals() (Config, error) {
	workers := 1
	if raw := os.Getenv("RSS_JS_WORKERS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 16 {
			return Config{}, errors.New("RSS_JS_WORKERS must be an integer between 1 and 16")
		}
		workers = n
	}
	ttl := 0
	if n, err := strconv.Atoi(os.Getenv("RSS_CONFIG_CACHE_TTL")); err == nil && n > 0 {
		ttl = min(n, 3600)
	}
	return Config{
		Server:     server.DefaultOptions(),
		Cache:      cacher.Config{RedisEnabled: define.REDIS, MemoryEnabled: define.IN_MEMORY_CACHE, RedisAddr: define.REDIS_SERVER, RedisPassword: define.REDIS_PASS, RedisDB: define.REDIS_DB, MemoryTTL: fn.ExpireBySecond(define.IN_MEMORY_EXPIRATION)},
		JavaScript: jssdk.RuntimeConfig{Binary: os.Getenv("RSS_CAN_DENO"), Workers: workers, Timeout: time.Duration(define.JS_EXECUTE_TIMEOUT) * time.Millisecond},
		Network:    network.Config{RequestTimeout: fn.ExpireBySecond(define.REQUEST_TIMEOUT), ServerTimeout: fn.ExpireBySecond(define.SERVER_TIMEOUT), DefaultTTL: fn.ExpireBySecond(define.IN_MEMORY_EXPIRATION), UserAgent: define.USER_AGENT, Proxy: define.PROXY_SERVER},
		Browser:    parser.BrowserOptions{Debug: define.CSR_DEBUG, IgnoreCertificateErrors: define.CSR_IGNORE_CERT_ERRORS}, BrowserTimeout: fn.ExpireBySecond(define.HEADLESS_EXCUTE_TIMEOUT), ConfigCacheTTL: time.Duration(ttl) * time.Second,
	}, nil
}

type App struct {
	config    Config
	cache     *cacher.Cache
	runtime   *jssdk.Runtime
	client    *network.Client
	rules     *rule.Repository
	feeds     *feedservice.Service
	routes    *fiber.App
	life      context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	stopping  bool
	requests  sync.WaitGroup
	work      workscope.Tracker
	closeOnce sync.Once
	closed    chan struct{}
	closeErr  error
}

func New(ctx context.Context, config Config) (*App, error) {
	if config.Server.Timeout <= 0 {
		return nil, errors.New("server timeout must be positive")
	}
	if config.Server.FeedPath == "" || config.Server.RuleDirectory == "" {
		return nil, errors.New("feed path and rule directory are required")
	}
	if config.BrowserTimeout <= 0 {
		config.BrowserTimeout = 5 * time.Second
	}
	if config.Network.DefaultTTL <= 0 {
		config.Network.DefaultTTL = 10 * time.Minute
	}
	if config.Cache.MemoryTTL <= 0 {
		config.Cache.MemoryTTL = config.Network.DefaultTTL
	}
	if config.Server.ShutdownTimeout <= 0 {
		config.Server.ShutdownTimeout = config.Server.Timeout + time.Second
	}
	config.Server.BrowserTimeout = config.BrowserTimeout
	config.Network.ServerTimeout = config.Server.Timeout
	proxy, err := network.NormalizeProxy(config.Network.Proxy)
	if err != nil {
		return nil, err
	}
	config.Network.Proxy = proxy
	config.Server.Proxy = proxy
	runtime, err := jssdk.NewRuntime(ctx, config.JavaScript)
	if err != nil {
		return nil, err
	}
	life, cancel := context.WithCancel(ctx)
	app := &App{config: config, runtime: runtime, cache: cacher.New(config.Cache), client: network.NewClient(config.Network), life: life, cancel: cancel, closed: make(chan struct{})}
	evaluator := func(ctx context.Context, snapshot define.RuleCache) (define.JavaScriptConfig, error) {
		return rule.EvaluateConfigContext(app.resources(ctx), snapshot)
	}
	app.rules = rule.NewRepositoryWithOptions(config.Server.RuleDirectory, evaluator, config.ConfigCacheTTL, config.Server.Timeout)
	if err := app.rules.Reload(app.resources(ctx)); err != nil {
		cancel()
		_ = app.runtime.Close()
		_ = app.cache.Close()
		return nil, fmt.Errorf("load rules: %w", err)
	}
	app.feeds = feedservice.New(config.Server.Timeout, feedservice.Dependencies{
		Lookup: app.rules.Lookup,
		Configure: func(ctx context.Context, snapshot define.RuleCache) (define.JavaScriptConfig, error) {
			return app.rules.Configure(app.resources(ctx), snapshot)
		},
		Fetch: func(ctx context.Context, config define.JavaScriptConfig) define.BodyParsed {
			if !fn.IsVaildHeadlessAddr(config.Headless) {
				config.Headless = app.config.Server.Headless
			}
			if config.Timeout <= 0 {
				config.Timeout = max(1, int(app.config.BrowserTimeout/time.Second))
			}
			if config.Expire <= 0 {
				config.Expire = max(1, int(app.config.Network.DefaultTTL/time.Second))
			}
			return rule.GetWebsiteDataWithConfigContext(app.resources(ctx), config)
		},
		Serialize: func(ctx context.Context, config define.JavaScriptConfig, data define.BodyParsed, format string) (string, error) {
			return generator.GenerateFeedsByTypeContext(app.resources(ctx), config, data, format)
		},
	})
	app.routes = server.NewAppWithDependencies(config.Server, server.Dependencies{
		Feeds: app.feeds, Snapshot: app.rules.Snapshot, Configure: app.rules.Configure,
		Reload: app.rules.Reload, RuntimeCheck: app.runtime.Check, CacheCheck: app.cache.Check, BindContext: app.bindRequest,
	})
	return app, nil
}

func (a *App) resources(ctx context.Context) context.Context {
	return workscope.WithTracker(parser.WithBrowserOptions(network.WithClient(cacher.WithCache(jssdk.WithRuntime(ctx, a.runtime), a.cache), a.client), a.config.Browser), &a.work)
}
func (a *App) bindRequest(ctx context.Context) (context.Context, func(), error) {
	a.mu.Lock()
	if a.stopping || a.life.Err() != nil {
		a.mu.Unlock()
		return nil, nil, errors.New("application is stopping")
	}
	a.requests.Add(1)
	a.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, a.config.Server.Timeout)
	stop := context.AfterFunc(a.life, cancel)
	return a.resources(ctx), func() { stop(); cancel(); a.requests.Done() }, nil
}
func (a *App) Routes() *fiber.App { return a.routes }

// Close stops admission, cancels in-flight work, waits for handlers to release
// owned dependencies, then closes the JS pool and cache exactly once.
func (a *App) Close() error {
	a.closeOnce.Do(func() {
		a.mu.Lock()
		a.stopping = true
		a.cancel()
		a.mu.Unlock()
		a.requests.Wait()
		a.work.Wait()
		a.closeErr = errors.Join(a.runtime.Close(), a.cache.Close())
		close(a.closed)
	})
	<-a.closed
	return a.closeErr
}
func (a *App) Run(ctx context.Context) error {
	defer a.Close()
	stop := context.AfterFunc(ctx, a.cancel)
	defer stop()
	if ctx.Err() != nil {
		a.cancel()
	}
	return server.Serve(a.life, a.routes, a.config.Server)
}
