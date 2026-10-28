package server

import (
	"context"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/rule"
)

func GetFeedPath() string {
	return define.HTTP_FEED_PATH
}

func StartWebServer() {
	StartWebServerWithFeedService(nil)
}

func StartWebServerWithFeedService(service FeedGenerator) {
	rule.InitRules()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	route := NewAppWithFeedService(service)

	addr := define.HTTP_HOST + ":" + strconv.Itoa(define.HTTP_PORT)

	go func() {
		if err := route.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
			logger.Instance.Fatalf("Program start error: %s\n", err)
		}
	}()
	logger.Instance.Infoln("RSS CAN has started 🚀")

	<-ctx.Done()

	stop()
	logger.Instance.Infoln("The program is closing, if you want to end it immediately, please press `CTRL+C`")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := route.ShutdownWithContext(ctx); err != nil {
		logger.Instance.Fatalf("Program was forced to close: %s\n", err)
	}

	logger.Instance.Infoln("Look forward to meeting you again ❤️")
}

// NewApp builds production routes without binding a listening socket.
func NewApp() *fiber.App {
	return NewAppWithFeedService(nil)
}

// NewAppWithFeedService accepts the application service assembled at startup.
func NewAppWithFeedService(service FeedGenerator) *fiber.App {
	timeout := fn.ExpireBySecond(define.SERVER_TIMEOUT)
	route := fiber.New(fiber.Config{
		ReadTimeout: timeout,
		IdleTimeout: timeout,
	})

	if !define.DEBUG_MODE {
		route.Use(compress.New())
	}

	route.Use(Logger(logger.Instance), recover.New())
	if service == nil {
		route.Get(GetFeedPath()+"/:id/:type/", apiRSS())
	} else {
		route.Get(GetFeedPath()+"/:id/:type/", apiRSSWithService(service))
	}
	route.Get(GetFeedPath(), listPage())
	route.Get("/config/:type/:value/", adminOnly(), apiConfig())

	route.Get("/inspector/:url/", adminOnly(), inspectorProxy())
	route.Get("/inspector", adminOnly(), inspectorHome())
	route.Post("/inspector", adminOnly(), inspectorPrepare())
	route.Get("/_/1x1.gif", PixelImage())

	route.Get("/_/health/", apiHealth())
	route.Get("/_/ready/", apiReady())
	route.Get("/_/metrics/", adminOnly(), apiMetrics())
	route.Use("/assets", static.New("/", static.Config{FS: ServerAssets()}))
	route.Get("/", welcomePage())

	return route
}
