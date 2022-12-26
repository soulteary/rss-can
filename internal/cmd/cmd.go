package cmd

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
)

func ParseFlags() (appFlags AppFlags) {
	flag.BoolVar(&appFlags.DEBUG_MODE, "debug", define.DEFAULT_DEBUG_MODE, fmt.Sprintf("whether to output debugging logging, env: `%s`", ENV_KEY_DEBUG))
	flag.StringVar(&appFlags.DEBUG_LEVEL, "debug-level", define.DEFAULT_DEBUG_LEVEL, fmt.Sprintf("set debug log printing level, env: `%s`", ENV_KEY_DEBUG_LEVEL))

	// flag.StringVar(&appFlags.Host, "host", "0.0.0.0", "web service listening address")
	flag.IntVar(&appFlags.HTTP_PORT, "port", define.DEFAULT_HTTP_PORT, fmt.Sprintf("web service listening port, env: `%s`", ENV_KEY_PORT))

	flag.IntVar(&appFlags.REQUEST_TIMEOUT, "timeout-request", define.DEFAULT_REQ_TIMEOUT, fmt.Sprintf("set request timeout, env: `%s`", ENV_KEY_REQUEST_TIMEOUT))
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

func SantizeFeedPath(feedpath string) string {
	s := "/" + strings.TrimRight(strings.TrimLeft(feedpath, "/"), "/")
	var re = regexp.MustCompile(`^\/[\w\d\-\_]+$`)
	match := re.FindAllStringSubmatch(s, -1)
	if len(match) == 0 {
		return define.DEFAULT_HTTP_FEED_PATH
	}
	return strings.ToLower(s)
}

func ApplyFlags() {
	args := ParseFlags()

	envDebugMode := os.Getenv(ENV_KEY_DEBUG)
	if envDebugMode != "" {
		define.DEBUG_MODE = fn.IsBoolString(envDebugMode)
	}
	if args.DEBUG_MODE != define.DEFAULT_DEBUG_MODE {
		define.DEBUG_MODE = args.DEBUG_MODE
	}

	envDebugLevel := os.Getenv(ENV_KEY_DEBUG_LEVEL)
	if fn.IsVaildLogLevel(envDebugLevel) {
		define.DEBUG_LEVEL = envDebugLevel
	}
	args.DEBUG_LEVEL = strings.ToLower(args.DEBUG_LEVEL)
	if fn.IsVaildLogLevel(args.DEBUG_LEVEL) && args.DEBUG_LEVEL != define.DEFAULT_DEBUG_LEVEL {
		define.DEBUG_LEVEL = args.DEBUG_LEVEL
	}

	envRequestTimeout := fn.StringToPositiveInteger(os.Getenv(ENV_KEY_REQUEST_TIMEOUT))
	if envRequestTimeout > 0 {
		define.REQUEST_TIMEOUT = envRequestTimeout
	}
	if args.REQUEST_TIMEOUT > 0 && args.REQUEST_TIMEOUT != define.REQUEST_TIMEOUT {
		define.REQUEST_TIMEOUT = args.REQUEST_TIMEOUT
	}

	envServerTimeout := fn.StringToPositiveInteger(os.Getenv(ENV_KEY_SERVER_TIMEOUT))
	if envServerTimeout > 0 {
		define.SERVER_TIMEOUT = envServerTimeout
	}
	if args.SERVER_TIMEOUT > 0 && args.SERVER_TIMEOUT != define.SERVER_TIMEOUT {
		define.SERVER_TIMEOUT = args.SERVER_TIMEOUT
	}

	envRuleDir := os.Getenv(ENV_KEY_RULE)
	if fn.IsNotEmptyAndNotDefaultString(envRuleDir, define.DEFAULT_RULES_DIRECTORY) {
		define.RULES_DIRECTORY = envRuleDir
	}
	if fn.IsNotEmptyAndNotDefaultString(args.RULES_DIRECTORY, define.DEFAULT_RULES_DIRECTORY) {
		define.RULES_DIRECTORY = args.RULES_DIRECTORY
	}

	envPort := fn.StringToPositiveInteger(os.Getenv(ENV_KEY_PORT))
	if fn.IsVaildPortRange(envPort) {
		define.HTTP_PORT = envPort
	}
	if fn.IsVaildPortRange(args.HTTP_PORT) && args.HTTP_PORT != define.HTTP_PORT {
		define.HTTP_PORT = args.HTTP_PORT
	}

	envHttpFeedPath := SantizeFeedPath(os.Getenv(ENV_KEY_HTTP_FEED_PATH))
	if fn.IsNotEmptyAndNotDefaultString(envHttpFeedPath, define.DEFAULT_HTTP_FEED_PATH) {
		define.HTTP_FEED_PATH = envHttpFeedPath
	}
	argHttpFeedPath := SantizeFeedPath(args.HTTP_FEED_PATH)
	if fn.IsNotEmptyAndNotDefaultString(argHttpFeedPath, define.DEFAULT_HTTP_FEED_PATH) {
		define.HTTP_FEED_PATH = argHttpFeedPath
	}

	envRedis := os.Getenv(ENV_KEY_REDIS)
	if envRedis != "" {
		define.REDIS = fn.IsBoolString(envRedis)
	}
	if args.REDIS != define.REDIS {
		define.REDIS = args.REDIS
	}

	if define.REDIS {
		// todo check `addr:port` is vaild
		envRedisServer := os.Getenv(ENV_KEY_REDIS_SERVER)
		if fn.IsNotEmptyAndNotDefaultString(envRedisServer, define.DEFAULT_REDIS_SERVER) {
			define.REDIS_SERVER = envRedisServer
		}
		if fn.IsNotEmptyAndNotDefaultString(args.REDIS_SERVER, define.DEFAULT_REDIS_SERVER) {
			define.REDIS_SERVER = args.REDIS_SERVER
		}

		envRedisPass := os.Getenv(ENV_KEY_REDIS_PASSWD)
		if envRedisPass != "" {
			if fn.IsNotEmptyAndNotDefaultString(envRedisPass, define.DEFAULT_REDIS_PASS) {
				define.REDIS_PASS = envRedisPass
			}
			if fn.IsNotEmptyAndNotDefaultString(args.REDIS_PASS, define.DEFAULT_REDIS_PASS) {
				define.REDIS_PASS = args.REDIS_PASS
			}

			envRedisDB := fn.StringToPositiveInteger(os.Getenv(ENV_KEY_REDIS_DB))
			if envRedisDB >= 0 {
				define.REDIS_DB = envRedisDB
			}
			if args.REDIS_DB >= 0 && args.REDIS_DB != define.DEFAULT_REDIS_DB {
				define.REDIS_DB = args.REDIS_DB
			}
		}
	}

	envMemory := os.Getenv(ENV_MEMORY)
	if envMemory != "" {
		define.IN_MEMORY_CACHE = fn.IsBoolString(envMemory)
	}
	if args.IN_MEMORY_CACHE != define.IN_MEMORY_CACHE {
		define.IN_MEMORY_CACHE = args.IN_MEMORY_CACHE
	}
	if define.IN_MEMORY_CACHE {
		envMemoryExpiration := fn.StringToPositiveInteger(os.Getenv(ENV_MEMORY_EXPIRATION))
		if envMemoryExpiration >= 0 {
			define.IN_MEMORY_EXPIRATION = envMemoryExpiration
		}
		if args.IN_MEMORY_EXPIRATION >= 0 && args.IN_MEMORY_EXPIRATION != define.DEFAULT_IN_MEMORY_CACHE_EXPIRATION {
			define.IN_MEMORY_EXPIRATION = args.IN_MEMORY_EXPIRATION
		}
	}

	// todo check `addr:port` is vaild
	envHeadlessServer := os.Getenv(ENV_KEY_HEADLESS_SERVER)
	if fn.IsNotEmptyAndNotDefaultString(envHeadlessServer, define.DEFAULT_HEADLESS_SERVER) {
		define.HEADLESS_SERVER = envHeadlessServer
	}
	if fn.IsNotEmptyAndNotDefaultString(args.HEADLESS_SERVER, define.DEFAULT_HEADLESS_SERVER) {
		define.HEADLESS_SERVER = args.HEADLESS_SERVER
	}

	// todo check `addr:port` is vaild
	envProxyServer := os.Getenv(ENV_KEY_PROXY)
	if fn.IsNotEmptyAndNotDefaultString(envProxyServer, define.DEFAULT_PROXY_ADDRESS) {
		define.PROXY_SERVER = envProxyServer
	}
	if fn.IsNotEmptyAndNotDefaultString(args.PROXY_SERVER, define.DEFAULT_PROXY_ADDRESS) {
		define.PROXY_SERVER = args.PROXY_SERVER
	}

	envJsExecTimeout := fn.StringToPositiveInteger(os.Getenv(ENV_KEY_JS_EXEC_TIMEOUT))
	if envJsExecTimeout >= 0 {
		define.JS_EXECUTE_TIMEOUT = envJsExecTimeout
	}
	if args.JS_EXECUTE_TIMEOUT > 0 && args.JS_EXECUTE_TIMEOUT != define.DEFAULT_JS_EXECUTE_TIMEOUT {
		define.JS_EXECUTE_TIMEOUT = args.JS_EXECUTE_TIMEOUT
	}

	envHeadlessSlowMotion := fn.StringToPositiveInteger(os.Getenv(ENV_KEY_HEADLESS_SLOW_MOTION))
	if envHeadlessSlowMotion >= 0 {
		define.HEADLESS_SLOW_MOTION = envHeadlessSlowMotion
	}
	if args.HEADLESS_SLOW_MOTION >= 0 && args.HEADLESS_SLOW_MOTION != define.DEFAULT_HEADLESS_SLOW_MOTION {
		define.HEADLESS_SLOW_MOTION = args.HEADLESS_SLOW_MOTION
	}

	envHeadlessExecTimeout := fn.StringToPositiveInteger(os.Getenv(ENV_KEY_HEADLESS_EXEC_TIMEOUT))
	if envHeadlessExecTimeout > 0 {
		define.HEADLESS_EXCUTE_TIMEOUT = envHeadlessExecTimeout
	}
	if args.HEADLESS_EXCUTE_TIMEOUT > 0 && args.HEADLESS_EXCUTE_TIMEOUT != define.DEFAULT_HEADLESS_EXCUTE_TIMEOUT {
		define.HEADLESS_EXCUTE_TIMEOUT = args.HEADLESS_EXCUTE_TIMEOUT
	}
}
