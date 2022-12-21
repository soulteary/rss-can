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
	flag.IntVar(&appFlags.TIMEOUT_HEADLESS_EXECUTION, "timeout-headless", define.DEFAULT_HEADLESS_EXCUTE_TIMEOUT, "headless timeout")

	flag.BoolVar(&appFlags.Redis, "redis", define.DEFAULT_REDIS, "Enable Redis")
	flag.StringVar(&appFlags.RedisAddr, "redis-addr", define.DEFAULT_REDIS_SERVER, "Redis server address")
	flag.StringVar(&appFlags.RedisPass, "redis-pass", define.DEFAULT_REDIS_PASS, "Redis password")
	flag.IntVar(&appFlags.RedisDB, "redis-db", define.DEFAULT_REDIS_DB, "Redis DB")

	flag.BoolVar(&appFlags.InMemoryCache, "in-memory", define.DEFAULT_IN_MEMORY_CACHE, "Enable in-memory cache")
	flag.IntVar(&appFlags.InMemoryCacheExpiration, "in-memory-expiration", define.DEFAULT_IN_MEMORY_CACHE_EXPIRATION, "set in-memory cache expiration")

	flag.StringVar(&appFlags.HeadlessAddr, "headless-addr", define.DEFAULT_HEADLESS_SERVER, "Headless server address")
	flag.IntVar(&appFlags.HeadlessSlowMotion, "headless-slow-motion", define.DEFAULT_HEADLESS_SLOW_MOTION, "Headless slow motion")

	flag.StringVar(&appFlags.Rules, "rules-dir", define.DEFAULT_RULES_DIRECTORY, "Rule directory")
	flag.StringVar(&appFlags.ProxyAddr, "proxy-addr", define.DEFAULT_PROXY_ADDRESS, "Proxy Addr")

	flag.Parse()

	return appFlags
}
