package cmd

import (
	"flag"

	"github.com/soulteary/RSS-Can/internal/define"
)

type AppFlags struct {
	DebugMode  bool
	DebugLevel string

	Host string
	Port int

	TIMEOUT_REQUEST            int
	TIMEOUT_SERVER             int
	TIMEOUT_JS_EXECUTION       int
	TIMEOUT_HEADLESS_EXECUTION int

	Redis     bool
	RedisAddr string
	RedisPass string
	RedisDB   int

	InMemoryCache           bool
	InMemoryCacheExpiration int

	HeadlessAddr       string
	HeadlessSlowMotion int

	Rules     string
	ProxyAddr string
}

func ParseFlags() (appFlags AppFlags) {
	flag.BoolVar(&appFlags.DebugMode, "debug", define.DEFAULT_DEBUG_MODE, "whether to output debugging logging")
	flag.StringVar(&appFlags.DebugLevel, "debuglevel", define.DEFAULT_DEBUG_LEVEL, "set debug level")

	flag.StringVar(&appFlags.Host, "host", "0.0.0.0", "the host to bind to")
	flag.IntVar(&appFlags.Port, "port", define.DEFAULT_HTTP_PORT, "the port to bind to")

	flag.IntVar(&appFlags.TIMEOUT_REQUEST, "timeout-request", define.DEFAULT_REQ_TIMEOUT, "request timeout")
	flag.IntVar(&appFlags.TIMEOUT_SERVER, "timeout-server", define.DEFAULT_SERVER_TIMEOUT, "server timeout")
	flag.IntVar(&appFlags.TIMEOUT_JS_EXECUTION, "timeout-js", define.DEFAULT_JS_EXECUTE_TIMEOUT, "js execute timeout")
	flag.IntVar(&appFlags.TIMEOUT_HEADLESS_EXECUTION, "timeout-headless", define.HEADLESS_EXCUTE_TIMEOUT, "headless timeout")

	flag.BoolVar(&appFlags.Redis, "redis", define.REDIS_ENABLED, "Enable Redis")
	flag.StringVar(&appFlags.RedisAddr, "redis-addr", define.PROD_REDIS_ADDRESS, "Redis address")
	flag.StringVar(&appFlags.RedisPass, "redis-pass", define.PROD_REDIS_PASSWORD, "Redis password")
	flag.IntVar(&appFlags.RedisDB, "redis-db", define.PROD_REDIS_DB, "Redis DB")

	flag.BoolVar(&appFlags.InMemoryCache, "in-memory", define.MEMORY_CACHE_ENABLED, "Enable In-Memory Cache")
	flag.IntVar(&appFlags.InMemoryCacheExpiration, "in-memory-expiration", define.IN_MEMORY_CACHE_EXPIRATION, "In-Memory Cache Expiration")

	flag.StringVar(&appFlags.HeadlessAddr, "headless-addr", define.DEFAULT_HEADLESS_CHROME, "Headless Chrome Addr")
	flag.IntVar(&appFlags.HeadlessSlowMotion, "headless-slow-motion", define.HEADLESS_SLOW_MOTION, "Headless Slow Motion")

	flag.StringVar(&appFlags.Rules, "rules-dir", define.DEFAULT_RULES_DIRECTORY, "Rule directory")
	flag.StringVar(&appFlags.ProxyAddr, "proxy-addr", define.DEFAULT_PROXY_ADDRESS, "Proxy Addr")

	flag.Parse()

	return appFlags
}
