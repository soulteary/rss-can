package server

import (
	"context"
	"errors"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/soulteary/RSS-Can/internal/access"
	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/network"
	"github.com/soulteary/RSS-Can/internal/rule"
	"github.com/soulteary/RSS-Can/internal/source"
)

func GetFeedPath() string {
	return define.HTTP_FEED_PATH
}

func StartWebServer() {
	StartWebServerWithFeedService(nil)
}

// StartWebServerWithFeedService is retained for standalone embedders. Production
// uses application.Run so dependency cleanup happens after requests have stopped.
func StartWebServerWithFeedService(service FeedGenerator) {
	rule.InitRules()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	options := DefaultOptions()
	if err := Serve(ctx, NewAppWithFeedService(service), options); err != nil {
		logger.Instance.Errorf("HTTP server stopped: %v", err)
	}
}

// Options is immutable after route construction.
type Options struct {
	Host            string
	Port            int
	FeedPath        string
	RuleDirectory   string
	Timeout         time.Duration
	ShutdownTimeout time.Duration
	BrowserTimeout  time.Duration
	Debug           bool
	AdminToken      string
	Headless        string
	Proxy           string
	Access          *access.Config
}

func DefaultOptions() Options {
	timeout := fn.ExpireBySecond(define.SERVER_TIMEOUT)
	return Options{Host: define.HTTP_HOST, Port: define.HTTP_PORT, FeedPath: define.HTTP_FEED_PATH, RuleDirectory: define.RULES_DIRECTORY, Timeout: timeout, ShutdownTimeout: timeout + time.Second, BrowserTimeout: fn.ExpireBySecond(define.HEADLESS_EXCUTE_TIMEOUT), Debug: define.DEBUG_MODE, AdminToken: os.Getenv("RSS_ADMIN_TOKEN"), Headless: define.HEADLESS_SERVER, Proxy: define.PROXY_SERVER}
}

type Dependencies struct {
	Feeds        FeedGenerator
	Snapshot     func() map[string]define.RuleCache
	Metadata     func() map[string]rule.Metadata
	Configure    func(context.Context, define.RuleCache) (define.JavaScriptConfig, error)
	Reload       func(context.Context) error
	RuntimeCheck func(context.Context) error
	CacheCheck   func(context.Context) cacher.Status
	BindContext  func(context.Context) (context.Context, func(), error)
	Access       *AccessController
	Preview      func(context.Context, source.Spec) ([]define.InfoItem, error)
}

// Serve reports listener/shutdown failures to its owner instead of calling
// Fatalf, which would skip the owner's deferred dependency cleanup.
func Serve(ctx context.Context, route *fiber.App, options Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(options.Host, strconv.Itoa(options.Port)))
	if err != nil {
		return err
	}
	defer listener.Close()
	result := make(chan error, 1)
	go func() { result <- route.Listener(listener, fiber.ListenConfig{DisableStartupMessage: true}) }()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
	}
	grace := options.ShutdownTimeout
	if grace <= 0 {
		grace = options.Timeout + time.Second
	}
	shutdown, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()
	err = route.ShutdownWithContext(shutdown)
	if err == nil {
		err = <-result
	}
	return err
}

// NewApp builds production routes without binding a listening socket.
func NewApp() *fiber.App {
	return NewAppWithFeedService(nil)
}

// NewAppWithFeedService accepts the application service assembled at startup.
func NewAppWithFeedService(service FeedGenerator) *fiber.App {
	if service == nil {
		service = defaultFeedGenerator()
	}
	return NewAppWithDependencies(DefaultOptions(), Dependencies{
		Feeds: service, Snapshot: rule.Snapshot, Configure: rule.GenerateConfigByRuleContext,
		Reload: rule.Reload, RuntimeCheck: func(ctx context.Context) error {
			_, err := jssdk.RunCodeContext(ctx, "", "0", "readiness.js")
			return err
		}, CacheCheck: cacher.Check,
	})
}

func NewAppWithDependencies(options Options, dependencies Dependencies) *fiber.App {
	route := fiber.New(fiber.Config{ReadTimeout: options.Timeout, IdleTimeout: options.Timeout})
	if !options.Debug {
		route.Use(compress.New())
	}
	route.Use(Logger(logger.Instance), recover.New())
	if dependencies.BindContext != nil {
		route.Use(func(c fiber.Ctx) error {
			ctx, done, err := dependencies.BindContext(c.Context())
			if err != nil {
				return c.Status(503).JSON(fiber.Map{"msg": "application is shutting down"})
			}
			defer done()
			c.SetContext(ctx)
			return c.Next()
		})
	}
	if options.Access != nil {
		route.Use(func(c fiber.Ctx) error {
			c.SetContext(network.WithFetchPolicy(c.Context(), network.PublicPolicy()))
			return c.Next()
		})
	}
	view := pageView{options: options, snapshot: dependencies.Snapshot, configure: dependencies.Configure, metadata: dependencies.Metadata}
	admin := adminOnlyWithToken(options.AdminToken)
	if options.Access != nil {
		controller := dependencies.Access
		if controller == nil {
			// Misconfigured embedders must fail closed, never silently expose the
			// old unprotected feed routes when team mode was requested.
			route.Use(func(c fiber.Ctx) error { return accessFailure(c, errors.New("access controller missing")) })
		} else {
			controller.Preview = dependencies.Preview
			view.access = controller
			route.Get(options.FeedPath+"/:id/:type/", controller.requireFeed(false), apiRSSWithService(dependencies.Feeds))
			route.Get("/s/:selector/:secret/:type", controller.requireFeed(true), apiRSSWithService(dependencies.Feeds))
			route.Get(options.FeedPath, controller.requireSubject(), view.listPage())
			route.Get("/api/sources", controller.requireSubject(), controller.listSources())
			route.Get("/api/grants", controller.requireSubject(), controller.listGrants())
			route.Get("/sources", controller.requireSubject(), sourceWizard())
			route.Post("/api/sources/preview", controller.requireSubject(), controller.previewManaged())
			route.Post("/api/sources", controller.requireSubject(), controller.saveManaged(false))
			route.Post("/api/sources/:id", controller.requireSubject(), controller.saveManaged(true))
			route.Post("/api/sources/:id/rollback", controller.requireSubject(), controller.rollbackManaged())
			route.Post("/api/sources/:id/grants", controller.requireSubject(), controller.createGrant())
			route.Post("/api/grants/:id/revoke", controller.requireSubject(), controller.revokeGrant())
			route.Post("/api/access/sources", admin, accessMutation(), controller.putSource(func(id string) bool { _, ok := dependencies.Snapshot()[id]; return ok }))
			route.Post("/api/access/principals", admin, accessMutation(), controller.setPrincipal())
		}
	} else {
		route.Get(options.FeedPath+"/:id/:type/", apiRSSWithService(dependencies.Feeds))
		route.Get(options.FeedPath, view.listPage())
	}
	route.Get("/config/:type/:value/", admin, func(c fiber.Ctx) error {
		c.Set("Allow", "POST")
		return c.SendStatus(405)
	})
	route.Post("/config/:type/:value/", admin, adminMutation(), apiConfigWithReload(dependencies.Reload))
	route.Get("/inspector/:url/", admin, inspectorProxyWithOptions(options))
	route.Get("/inspector", admin, inspectorHome())
	route.Post("/inspector", admin, inspectorPrepare())
	route.Get("/_/1x1.gif", PixelImage())
	route.Get("/_/health/", apiHealth())
	route.Get("/_/ready/", readiness(dependencies.RuntimeCheck, dependencies.CacheCheck))
	route.Get("/_/metrics/", admin, apiMetrics())
	route.Use("/assets", static.New("/", static.Config{FS: ServerAssets()}))
	route.Get("/", view.welcomePage())
	return route
}
