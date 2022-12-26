package cmd

import (
	"flag"
	"fmt"

	"github.com/soulteary/RSS-Can/internal/define"
)

func ParseFlags() (appFlags AppFlags) {
	flag.BoolVar(&appFlags.DEBUG_MODE, "debug", define.DEFAULT_DEBUG_MODE, fmt.Sprintf("whether to output debugging logging, env: `%s`", ENV_KEY_DEBUG))
	flag.StringVar(&appFlags.DEBUG_LEVEL, "debug-level", define.DEFAULT_DEBUG_LEVEL, fmt.Sprintf("set debug log printing level, env: `%s`", ENV_KEY_DEBUG_LEVEL))

	// flag.StringVar(&appFlags.Host, "host", "0.0.0.0", "web service listening address")
	flag.IntVar(&appFlags.HTTP_PORT, "port", define.DEFAULT_HTTP_PORT, fmt.Sprintf("web service listening port, env: `%s`", ENV_KEY_PORT))

	flag.IntVar(&appFlags.REQUEST_TIMEOUT, "timeout-request", define.DEFAULT_REQUEST_TIMEOUT, fmt.Sprintf("set request timeout, env: `%s`", ENV_KEY_REQUEST_TIMEOUT))
	flag.IntVar(&appFlags.SERVER_TIMEOUT, "timeout-server", define.DEFAULT_SERVER_TIMEOUT, fmt.Sprintf("set web server response timeout, env: `%s`", ENV_KEY_SERVER_TIMEOUT))
	flag.IntVar(&appFlags.JS_EXECUTE_TIMEOUT, "timeout-js", define.DEFAULT_JS_EXECUTE_TIMEOUT, fmt.Sprintf("set js sandbox code execution timeout, env: `%s`", ENV_KEY_JS_EXEC_TIMEOUT))
	flag.IntVar(&appFlags.HEADLESS_EXCUTE_TIMEOUT, "timeout-headless", define.DEFAULT_HEADLESS_EXCUTE_TIMEOUT, fmt.Sprintf("set headless execution timeout, env: `%s`", ENV_KEY_HEADLESS_EXEC_TIMEOUT))

	flag.BoolVar(&appFlags.REDIS, "redis", define.DEFAULT_REDIS, fmt.Sprintf("using Redis as a cache service, env: `%s`", ENV_KEY_REDIS))
	flag.StringVar(&appFlags.REDIS_SERVER, "redis-addr", define.DEFAULT_REDIS_SERVER, fmt.Sprintf("set Redis server address, env: `%s`", ENV_KEY_REDIS_SERVER))
	flag.StringVar(&appFlags.REDIS_PASS, "redis-pass", define.DEFAULT_REDIS_PASS, fmt.Sprintf("set Redis password, env: `%s`", ENV_KEY_REDIS_PASSWD))
	flag.IntVar(&appFlags.REDIS_DB, "redis-db", define.DEFAULT_REDIS_DB, fmt.Sprintf("set Redis db, env: `%s`", ENV_KEY_REDIS_DB))

	flag.BoolVar(&appFlags.IN_MEMORY_CACHE, "memory", define.DEFAULT_IN_MEMORY_CACHE, fmt.Sprintf("using Memory(build-in) as a cache service, env: `%s`", ENV_MEMORY))
	flag.IntVar(&appFlags.IN_MEMORY_EXPIRATION, "memory-expiration", define.DEFAULT_IN_MEMORY_CACHE_EXPIRATION, fmt.Sprintf("set Memory cache expiration, env: `%s`", ENV_MEMORY_EXPIRATION))

	flag.StringVar(&appFlags.HEADLESS_SERVER, "headless-addr", define.DEFAULT_HEADLESS_SERVER, fmt.Sprintf("set Headless server address, env: `%s`", ENV_KEY_HEADLESS_SERVER))
	flag.IntVar(&appFlags.HEADLESS_SLOW_MOTION, "headless-slow-motion", define.DEFAULT_HEADLESS_SLOW_MOTION, fmt.Sprintf("set Headless slow motion, env: `%s`", ENV_KEY_HEADLESS_SLOW_MOTION))

	flag.StringVar(&appFlags.RULES_DIRECTORY, "rule", define.DEFAULT_RULES_DIRECTORY, fmt.Sprintf("set Rule directory, env: `%s`", ENV_KEY_RULE))
	flag.StringVar(&appFlags.PROXY_SERVER, "proxy", define.DEFAULT_PROXY_ADDRESS, fmt.Sprintf("Proxy, env: `%s`", ENV_KEY_PROXY))

	flag.StringVar(&appFlags.HTTP_FEED_PATH, "feed-path", define.DEFAULT_HTTP_FEED_PATH, fmt.Sprintf("http feed path, env: `%s`", ENV_KEY_HTTP_FEED_PATH))

	flag.Parse()

	return appFlags
}

func ApplyFlags() {
	args := ParseFlags()

	define.DEBUG_MODE = UpdateBoolOption(ENV_KEY_DEBUG, args.DEBUG_MODE, define.DEFAULT_DEBUG_MODE)
	define.DEBUG_LEVEL = updateLogOption(ENV_KEY_DEBUG_LEVEL, args.DEBUG_LEVEL, define.DEFAULT_DEBUG_LEVEL)
	define.REQUEST_TIMEOUT = UpdateNumberOption(ENV_KEY_REQUEST_TIMEOUT, args.REQUEST_TIMEOUT, define.DEFAULT_REQUEST_TIMEOUT, false)
	define.SERVER_TIMEOUT = UpdateNumberOption(ENV_KEY_SERVER_TIMEOUT, args.SERVER_TIMEOUT, define.DEFAULT_SERVER_TIMEOUT, false)
	define.RULES_DIRECTORY = updateStringOption(ENV_KEY_RULE, args.RULES_DIRECTORY, define.DEFAULT_RULES_DIRECTORY)
	define.HTTP_PORT = updatePortOption(ENV_KEY_PORT, args.HTTP_PORT, define.DEFAULT_HTTP_PORT)
	define.HTTP_FEED_PATH = updateFeedPathOption(ENV_KEY_HTTP_FEED_PATH, args.HTTP_FEED_PATH, define.DEFAULT_HTTP_FEED_PATH)
	define.REDIS = UpdateBoolOption(ENV_KEY_REDIS, args.REDIS, define.DEFAULT_REDIS)
	if define.REDIS {
		// todo check `addr:port` is vaild
		define.REDIS_SERVER = updateStringOption(ENV_KEY_REDIS_SERVER, args.REDIS_SERVER, define.DEFAULT_REDIS_SERVER)
		define.REDIS_PASS = updateStringOption(ENV_KEY_REDIS_PASSWD, args.REDIS_PASS, define.DEFAULT_REDIS_PASS)
		define.REDIS_DB = UpdateNumberOption(ENV_KEY_REDIS_DB, args.REDIS_DB, define.DEFAULT_REDIS_DB, true)
	}
	define.IN_MEMORY_CACHE = UpdateBoolOption(ENV_MEMORY, args.IN_MEMORY_CACHE, define.DEFAULT_IN_MEMORY_CACHE)
	if define.IN_MEMORY_CACHE {
		define.IN_MEMORY_EXPIRATION = UpdateNumberOption(ENV_MEMORY_EXPIRATION, args.IN_MEMORY_EXPIRATION, define.DEFAULT_IN_MEMORY_CACHE_EXPIRATION, true)
	}
	// todo check `addr:port` is vaild
	define.HEADLESS_SERVER = updateStringOption(ENV_KEY_HEADLESS_SERVER, args.HEADLESS_SERVER, define.DEFAULT_HEADLESS_SERVER)
	// todo check `addr:port` is vaild
	define.PROXY_SERVER = updateStringOption(ENV_KEY_PROXY, args.PROXY_SERVER, define.DEFAULT_PROXY_ADDRESS)
	define.JS_EXECUTE_TIMEOUT = UpdateNumberOption(ENV_KEY_JS_EXEC_TIMEOUT, args.JS_EXECUTE_TIMEOUT, define.DEFAULT_JS_EXECUTE_TIMEOUT, true)
	define.HEADLESS_SLOW_MOTION = UpdateNumberOption(ENV_KEY_HEADLESS_SLOW_MOTION, args.HEADLESS_SLOW_MOTION, define.DEFAULT_HEADLESS_SLOW_MOTION, true)
	define.HEADLESS_EXCUTE_TIMEOUT = UpdateNumberOption(ENV_KEY_HEADLESS_EXEC_TIMEOUT, args.HEADLESS_EXCUTE_TIMEOUT, define.DEFAULT_HEADLESS_EXCUTE_TIMEOUT, false)
}
