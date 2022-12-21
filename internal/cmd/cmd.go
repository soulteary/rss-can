package cmd

import (
	"flag"
	"os"
	"strconv"
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

func IsBool(input string) bool {
	s := strings.ToLower(input)
	if s == "true" || s == "1" || s == "on" {
		return true
	}
	return false
}

func IsVaildLogLevel(level string) bool {
	s := strings.ToLower(level)
	return s == "info" || s == "error" || s == "warn" || s == "debug"
}

func ConvertStringToPositiveInteger(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return i
}

func IsVaildPortRange(port int) bool {
	return port > 0 && port < 65535
}

func ApplyFlags() {
	args := ParseFlags()

	envDebugMode := os.Getenv("RSS_DEBUG")
	if envDebugMode != "" {
		define.DEBUG_MODE = IsBool(envDebugMode)
	}
	if args.DEBUG_MODE != define.DEFAULT_DEBUG_MODE {
		define.DEBUG_MODE = args.DEBUG_MODE
	}

	envDebugLevel := os.Getenv("RSS_DEBUG_LEVEL")
	if IsVaildLogLevel(envDebugLevel) {
		define.DEBUG_LEVEL = envDebugLevel
	}
	args.DEBUG_LEVEL = strings.ToLower(args.DEBUG_LEVEL)
	if IsVaildLogLevel(args.DEBUG_LEVEL) && args.DEBUG_LEVEL != define.DEFAULT_DEBUG_LEVEL {
		define.DEBUG_LEVEL = args.DEBUG_LEVEL
	}

	envRequestTimeout := ConvertStringToPositiveInteger(os.Getenv("RSS_REQUEST_TIMEOUT"))
	if envRequestTimeout > 0 {
		define.REQUEST_TIMEOUT = envRequestTimeout
	}
	if args.REQUEST_TIMEOUT > 0 && args.REQUEST_TIMEOUT != define.REQUEST_TIMEOUT {
		define.REQUEST_TIMEOUT = args.REQUEST_TIMEOUT
	}

	envServerTimeout := ConvertStringToPositiveInteger(os.Getenv("RSS_SERVER_TIMEOUT"))
	if envServerTimeout > 0 {
		define.SERVER_TIMEOUT = envServerTimeout
	}
	if args.SERVER_TIMEOUT > 0 {
		define.SERVER_TIMEOUT = args.SERVER_TIMEOUT
	}

	envRuleDir := os.Getenv("RSS_RULE")
	if envRuleDir != "" {
		define.RULES_DIRECTORY = envRuleDir
	}
	if args.RULES_DIRECTORY != define.RULES_DIRECTORY {
		define.RULES_DIRECTORY = args.RULES_DIRECTORY
	}

	envPort := ConvertStringToPositiveInteger(os.Getenv("RSS_PORT"))
	if IsVaildPortRange(envPort) {
		define.HTTP_PORT = envPort
	}
	if IsVaildPortRange(args.HTTP_PORT) && args.HTTP_PORT != define.HTTP_PORT {
		define.HTTP_PORT = args.HTTP_PORT
	}

	envRedis := os.Getenv("RSS_REDIS")
	if envRedis != "" {
		define.REDIS = IsBool(envRedis)
	}
	if args.REDIS != define.REDIS {
		define.REDIS = args.REDIS
	}

	if define.REDIS {
		// todo check `addr:port` is vaild
		envRedisServer := os.Getenv("RSS_SERVER")
		if envRedisServer != "" {
			define.REDIS_SERVER = envRedisServer
		}
		if args.REDIS_SERVER != define.REDIS_SERVER {
			define.REDIS_SERVER = args.REDIS_SERVER
		}

		envRedisPass := os.Getenv("RSS_REDIS_PASSWD")
		if envRedisPass != "" {
			define.REDIS_PASS = envRedisPass
		}
		if args.REDIS_PASS != define.REDIS_PASS {
			define.REDIS_PASS = args.REDIS_PASS
		}

		envRedisDB := ConvertStringToPositiveInteger(os.Getenv("RSS_REDIS_DB"))
		if envRedisDB >= 0 {
			define.REDIS_DB = envRedisDB
		}
		if args.REDIS_DB != define.REDIS_DB {
			define.REDIS_DB = args.REDIS_DB
		}
	}

	envMemory := os.Getenv("RSS_MEMORY")
	if envMemory != "" {
		define.IN_MEMORY_CACHE = IsBool(envMemory)
	}
	if args.IN_MEMORY_CACHE != define.IN_MEMORY_CACHE {
		define.IN_MEMORY_CACHE = args.IN_MEMORY_CACHE
	}
	if define.IN_MEMORY_CACHE {
		envMemoryExpiration := ConvertStringToPositiveInteger(os.Getenv("RSS_MEMORY_EXPIRATION"))
		if envMemoryExpiration >= 0 {
			define.IN_MEMORY_EXPIRATION = envMemoryExpiration
		}
		if args.IN_MEMORY_EXPIRATION != define.IN_MEMORY_EXPIRATION {
			define.IN_MEMORY_EXPIRATION = args.IN_MEMORY_EXPIRATION
		}
	}

	// todo check `addr:port` is vaild
	envHeadlessServer := os.Getenv("RSS_HEADLESS_SERVER")
	if envHeadlessServer != "" {
		define.HEADLESS_SERVER = envHeadlessServer
	}
	if args.HEADLESS_SERVER != define.HEADLESS_SERVER {
		define.HEADLESS_SERVER = args.HEADLESS_SERVER
	}

	// todo check `addr:port` is vaild
	envProxyServer := os.Getenv("RSS_PROXY")
	if envProxyServer != "" {
		define.PROXY_SERVER = envProxyServer
	}
	if args.PROXY_SERVER != define.PROXY_SERVER {
		define.PROXY_SERVER = args.PROXY_SERVER
	}

	envJsExecTimeout := ConvertStringToPositiveInteger(os.Getenv("RSS_JS_EXEC_TIMEOUT"))
	if envJsExecTimeout >= 0 {
		define.JS_EXECUTE_TIMEOUT = envJsExecTimeout
	}
	if args.JS_EXECUTE_TIMEOUT > 0 {
		define.JS_EXECUTE_TIMEOUT = args.JS_EXECUTE_TIMEOUT
	}

	envHeadlessSlowMotion := ConvertStringToPositiveInteger(os.Getenv("RSS_HEADLESS_SLOW_MONTION"))
	if envHeadlessSlowMotion >= 0 {
		define.HEADLESS_SLOW_MOTION = envHeadlessSlowMotion
	}
	if args.HEADLESS_SLOW_MOTION > 0 {
		define.HEADLESS_SLOW_MOTION = args.HEADLESS_SLOW_MOTION
	}

	envHeadlessExecTimeout := ConvertStringToPositiveInteger(os.Getenv("RSS_HEADLESS_EXEC_TIMEOUT"))
	if envHeadlessExecTimeout > 0 {
		define.HEADLESS_EXCUTE_TIMEOUT = envHeadlessExecTimeout
	}
	if args.HEADLESS_EXCUTE_TIMEOUT > 0 {
		define.HEADLESS_EXCUTE_TIMEOUT = args.HEADLESS_EXCUTE_TIMEOUT
	}

}
