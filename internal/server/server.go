package server

import (
	"context"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/rule"
)

func StartWebServer() {
	rule.InitRules()

	if !define.GLOBAL_DEBUG_MODE {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	route := gin.Default()

	if !define.GLOBAL_DEBUG_MODE {
		route.Use(gzip.Gzip(gzip.DefaultCompression))
	}

	route.Use(Logger(logger.Instance), gin.Recovery())
	route.GET("/feed/:id/:type/", apiRSS())
	route.GET("/config/:type/:value/", apiConfig())
	route.GET("/_/health/", apiHealth())
	route.GET("/", welcomePage())

	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(define.DEFAULT_HTTP_PORT),
		Handler:           route,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Instance.Fatalf("Program start error: %s\n", err)
		}
	}()
	logger.Instance.Infoln("RSS CAN has started 🚀")

	<-ctx.Done()

	stop()
	logger.Instance.Infoln("The program is closing, if you want to end it immediately, please press `CTRL+C`")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Instance.Fatalf("Program was forced to close: %s\n", err)
	}

	logger.Instance.Infoln("Look forward to meeting you again ❤️")
}
