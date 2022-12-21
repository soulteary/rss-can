package cmd

import (
	"flag"
	"strings"

	"github.com/soulteary/RSS-Can/internal/define"
)

type AppFlags struct {
	DEBUG_MODE  bool
	DEBUG_LEVEL string

	Host      string
	HTTP_PORT int

	REQUEST_TIMEOUT         int
	SERVER_TIMEOUT          int
	JS_EXECUTE_TIMEOUT      int
	HEADLESS_EXCUTE_TIMEOUT int

	REDIS        bool
	REDIS_SERVER string
	REDIS_PASS   string
	REDIS_DB     int

	IN_MEMORY_CACHE      bool
	IN_MEMORY_EXPIRATION int

	HEADLESS_SERVER      string
	HEADLESS_SLOW_MOTION int

	RULES_DIRECTORY string
	PROXY_SERVER    string
}

func ParseFlags() (appFlags AppFlags) {
	flag.BoolVar(&appFlags.DEBUG_MODE, "debug", define.DEFAULT_DEBUG_MODE, "whether to output debugging logging")
	flag.StringVar(&appFlags.DEBUG_LEVEL, "debug-level", define.DEFAULT_DEBUG_LEVEL, "set debug log printing level")

	flag.StringVar(&appFlags.Host, "host", "0.0.0.0", "web service listening address")
	flag.IntVar(&appFlags.HTTP_PORT, "port", define.DEFAULT_HTTP_PORT, "web service listening port")

	flag.IntVar(&appFlags.REQUEST_TIMEOUT, "timeout-request", define.DEFAULT_REQ_TIMEOUT, "set request timeout")
	flag.IntVar(&appFlags.SERVER_TIMEOUT, "timeout-server", define.DEFAULT_SERVER_TIMEOUT, "set web server response timeout")
	flag.IntVar(&appFlags.JS_EXECUTE_TIMEOUT, "timeout-js", define.DEFAULT_JS_EXECUTE_TIMEOUT, "set js sandbox code execution timeout")
	flag.IntVar(&appFlags.HEADLESS_EXCUTE_TIMEOUT, "timeout-headless", define.DEFAULT_HEADLESS_EXCUTE_TIMEOUT, "set headless execution timeout")

	flag.BoolVar(&appFlags.REDIS, "redis", define.DEFAULT_REDIS, "using Redis as a cache service")
	flag.StringVar(&appFlags.REDIS_SERVER, "redis-addr", define.DEFAULT_REDIS_SERVER, "set Redis server address")
	flag.StringVar(&appFlags.REDIS_PASS, "redis-pass", define.DEFAULT_REDIS_PASS, "set Redis password")
	flag.IntVar(&appFlags.REDIS_DB, "redis-db", define.DEFAULT_REDIS_DB, "set Redis db")

	flag.BoolVar(&appFlags.IN_MEMORY_CACHE, "memory", define.DEFAULT_IN_MEMORY_CACHE, "using Memory(build-in) as a cache service")
	flag.IntVar(&appFlags.IN_MEMORY_EXPIRATION, "memory-expiration", define.DEFAULT_IN_MEMORY_CACHE_EXPIRATION, "set Memory cache expiration")

	flag.StringVar(&appFlags.HEADLESS_SERVER, "headless-addr", define.DEFAULT_HEADLESS_SERVER, "set Headless server address")
	flag.IntVar(&appFlags.HEADLESS_SLOW_MOTION, "headless-slow-motion", define.DEFAULT_HEADLESS_SLOW_MOTION, "set Headless slow motion")

	flag.StringVar(&appFlags.RULES_DIRECTORY, "rule", define.DEFAULT_RULES_DIRECTORY, "set Rule directory")
	flag.StringVar(&appFlags.PROXY_SERVER, "proxy", define.DEFAULT_PROXY_ADDRESS, "Proxy")

	flag.Parse()

	return appFlags
}

func ApplyFlags() {
	args := ParseFlags()

	define.DEBUG_MODE = args.DEBUG_MODE

	args.DEBUG_LEVEL = strings.ToLower(args.DEBUG_LEVEL)
	if args.DEBUG_LEVEL == "info" || args.DEBUG_LEVEL == "error" || args.DEBUG_LEVEL == "warn" || args.DEBUG_LEVEL == "debug" {
		define.DEBUG_LEVEL = args.DEBUG_LEVEL
	}

	if args.REQUEST_TIMEOUT > 0 {
		define.REQUEST_TIMEOUT = args.REQUEST_TIMEOUT
	}

	if args.SERVER_TIMEOUT > 0 {
		define.SERVER_TIMEOUT = args.SERVER_TIMEOUT
	}

	define.RULES_DIRECTORY = args.RULES_DIRECTORY

	if args.HTTP_PORT > 0 && args.HTTP_PORT < 65535 {
		define.HTTP_PORT = args.HTTP_PORT
	}

	define.REDIS = args.REDIS
	if args.REDIS {
		// todo check `addr:port` is vaild
		define.REDIS_SERVER = args.REDIS_SERVER
		define.REDIS_PASS = args.REDIS_PASS
		define.REDIS_DB = args.REDIS_DB
	}

	define.IN_MEMORY_CACHE = args.IN_MEMORY_CACHE
	if args.IN_MEMORY_CACHE {
		define.IN_MEMORY_EXPIRATION = args.IN_MEMORY_EXPIRATION
	}

	// todo check `addr:port` is vaild
	define.HEADLESS_SERVER = args.HEADLESS_SERVER
	// todo check `addr:port` is vaild
	define.PROXY_SERVER = args.PROXY_SERVER

	if args.JS_EXECUTE_TIMEOUT > 0 {
		define.JS_EXECUTE_TIMEOUT = args.JS_EXECUTE_TIMEOUT
	}

	if args.HEADLESS_SLOW_MOTION > 0 {
		define.HEADLESS_SLOW_MOTION = args.HEADLESS_SLOW_MOTION
	}

	if args.HEADLESS_EXCUTE_TIMEOUT > 0 {
		define.HEADLESS_EXCUTE_TIMEOUT = args.HEADLESS_EXCUTE_TIMEOUT
	}

}
