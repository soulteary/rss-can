package cmd

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/soulteary/RSS-Can/internal/define"
)

type AppFlags struct {
	DEBUG_MODE  bool
	DEBUG_LEVEL string

	Host           string
	HTTP_PORT      int
	HTTP_FEED_PATH string

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

func SantizeFeedPath(feedpath string) string {
	s := "/" + strings.TrimRight(strings.TrimLeft(feedpath, "/"), "/")
	var re = regexp.MustCompile(`^\/[\w\d\-\_]+$`)
	match := re.FindAllStringSubmatch(s, -1)
	if len(match) == 0 {
		return define.DEFAULT_HTTP_FEED_PATH
	}
	return strings.ToLower(s)
}

const (
	ENV_KEY_DEBUG                 = "RSS_DEBUG"
	ENV_KEY_DEBUG_LEVEL           = "RSS_DEBUG_LEVEL"
	ENV_KEY_REQUEST_TIMEOUT       = "RSS_REQUEST_TIMEOUT"
	ENV_KEY_SERVER_TIMEOUT        = "RSS_SERVER_TIMEOUT"
	ENV_KEY_RULE                  = "RSS_RULE"
	ENV_KEY_PORT                  = "RSS_PORT"
	ENV_KEY_REDIS                 = "RSS_REDIS"
	ENV_KEY_REDIS_SERVER          = "RSS_SERVER"
	ENV_KEY_REDIS_PASSWD          = "RSS_REDIS_PASSWD"
	ENV_KEY_REDIS_DB              = "RSS_REDIS_DB"
	ENV_MEMORY                    = "RSS_MEMORY"
	ENV_MEMORY_EXPIRATION         = "RSS_MEMORY_EXPIRATION"
	ENV_KEY_HEADLESS_SERVER       = "RSS_HEADLESS_SERVER"
	ENV_KEY_PROXY                 = "RSS_PROXY"
	ENV_KEY_JS_EXEC_TIMEOUT       = "RSS_JS_EXEC_TIMEOUT"
	ENV_KEY_HEADLESS_SLOW_MOTION  = "RSS_HEADLESS_SLOW_MOTION"
	ENV_KEY_HEADLESS_EXEC_TIMEOUT = "RSS_HEADLESS_EXEC_TIMEOUT"
	ENV_KEY_HTTP_FEED_PATH        = "RSS_HTTP_FEED_PATH"
)

func ApplyFlags() {
	args := ParseFlags()

	envDebugMode := os.Getenv(ENV_KEY_DEBUG)
	if envDebugMode != "" {
		define.DEBUG_MODE = IsBool(envDebugMode)
	}
	if args.DEBUG_MODE != define.DEFAULT_DEBUG_MODE {
		define.DEBUG_MODE = args.DEBUG_MODE
	}

	envDebugLevel := os.Getenv(ENV_KEY_DEBUG_LEVEL)
	if IsVaildLogLevel(envDebugLevel) {
		define.DEBUG_LEVEL = envDebugLevel
	}
	args.DEBUG_LEVEL = strings.ToLower(args.DEBUG_LEVEL)
	if IsVaildLogLevel(args.DEBUG_LEVEL) && args.DEBUG_LEVEL != define.DEFAULT_DEBUG_LEVEL {
		define.DEBUG_LEVEL = args.DEBUG_LEVEL
	}

	envRequestTimeout := ConvertStringToPositiveInteger(os.Getenv(ENV_KEY_REQUEST_TIMEOUT))
	if envRequestTimeout > 0 {
		define.REQUEST_TIMEOUT = envRequestTimeout
	}
	if args.REQUEST_TIMEOUT > 0 && args.REQUEST_TIMEOUT != define.REQUEST_TIMEOUT {
		define.REQUEST_TIMEOUT = args.REQUEST_TIMEOUT
	}

	envServerTimeout := ConvertStringToPositiveInteger(os.Getenv(ENV_KEY_SERVER_TIMEOUT))
	if envServerTimeout > 0 {
		define.SERVER_TIMEOUT = envServerTimeout
	}
	if args.SERVER_TIMEOUT > 0 {
		define.SERVER_TIMEOUT = args.SERVER_TIMEOUT
	}

	envRuleDir := os.Getenv(ENV_KEY_RULE)
	if envRuleDir != "" {
		define.RULES_DIRECTORY = envRuleDir
	}
	if args.RULES_DIRECTORY != define.RULES_DIRECTORY {
		define.RULES_DIRECTORY = args.RULES_DIRECTORY
	}

	envPort := ConvertStringToPositiveInteger(os.Getenv(ENV_KEY_PORT))
	if IsVaildPortRange(envPort) {
		define.HTTP_PORT = envPort
	}
	if IsVaildPortRange(args.HTTP_PORT) && args.HTTP_PORT != define.HTTP_PORT {
		define.HTTP_PORT = args.HTTP_PORT
	}

	envHttpFeedPath := SantizeFeedPath(os.Getenv(ENV_KEY_HTTP_FEED_PATH))
	if envHttpFeedPath != define.DEFAULT_HTTP_FEED_PATH {
		define.HTTP_FEED_PATH = envHttpFeedPath
	}
	argHttpFeedPath := SantizeFeedPath(args.HTTP_FEED_PATH)
	if argHttpFeedPath != define.DEFAULT_HTTP_FEED_PATH {
		define.HTTP_FEED_PATH = argHttpFeedPath
	}

	envRedis := os.Getenv(ENV_KEY_REDIS)
	if envRedis != "" {
		define.REDIS = IsBool(envRedis)
	}
	if args.REDIS != define.REDIS {
		define.REDIS = args.REDIS
	}

	if define.REDIS {
		// todo check `addr:port` is vaild
		envRedisServer := os.Getenv(ENV_KEY_REDIS_SERVER)
		if envRedisServer != "" {
			define.REDIS_SERVER = envRedisServer
		}
		if args.REDIS_SERVER != define.REDIS_SERVER {
			define.REDIS_SERVER = args.REDIS_SERVER
		}

		envRedisPass := os.Getenv(ENV_KEY_REDIS_PASSWD)
		if envRedisPass != "" {
			define.REDIS_PASS = envRedisPass
		}
		if args.REDIS_PASS != define.REDIS_PASS {
			define.REDIS_PASS = args.REDIS_PASS
		}

		envRedisDB := ConvertStringToPositiveInteger(os.Getenv(ENV_KEY_REDIS_DB))
		if envRedisDB >= 0 {
			define.REDIS_DB = envRedisDB
		}
		if args.REDIS_DB != define.REDIS_DB {
			define.REDIS_DB = args.REDIS_DB
		}
	}

	envMemory := os.Getenv(ENV_MEMORY)
	if envMemory != "" {
		define.IN_MEMORY_CACHE = IsBool(envMemory)
	}
	if args.IN_MEMORY_CACHE != define.IN_MEMORY_CACHE {
		define.IN_MEMORY_CACHE = args.IN_MEMORY_CACHE
	}
	if define.IN_MEMORY_CACHE {
		envMemoryExpiration := ConvertStringToPositiveInteger(os.Getenv(ENV_MEMORY_EXPIRATION))
		if envMemoryExpiration >= 0 {
			define.IN_MEMORY_EXPIRATION = envMemoryExpiration
		}
		if args.IN_MEMORY_EXPIRATION != define.IN_MEMORY_EXPIRATION {
			define.IN_MEMORY_EXPIRATION = args.IN_MEMORY_EXPIRATION
		}
	}

	// todo check `addr:port` is vaild
	envHeadlessServer := os.Getenv(ENV_KEY_HEADLESS_SERVER)
	if envHeadlessServer != "" {
		define.HEADLESS_SERVER = envHeadlessServer
	}
	if args.HEADLESS_SERVER != define.HEADLESS_SERVER {
		define.HEADLESS_SERVER = args.HEADLESS_SERVER
	}

	// todo check `addr:port` is vaild
	envProxyServer := os.Getenv(ENV_KEY_PROXY)
	if envProxyServer != "" {
		define.PROXY_SERVER = envProxyServer
	}
	if args.PROXY_SERVER != define.PROXY_SERVER {
		define.PROXY_SERVER = args.PROXY_SERVER
	}

	envJsExecTimeout := ConvertStringToPositiveInteger(os.Getenv(ENV_KEY_JS_EXEC_TIMEOUT))
	if envJsExecTimeout >= 0 {
		define.JS_EXECUTE_TIMEOUT = envJsExecTimeout
	}
	if args.JS_EXECUTE_TIMEOUT > 0 {
		define.JS_EXECUTE_TIMEOUT = args.JS_EXECUTE_TIMEOUT
	}

	envHeadlessSlowMotion := ConvertStringToPositiveInteger(os.Getenv(ENV_KEY_HEADLESS_SLOW_MOTION))
	if envHeadlessSlowMotion >= 0 {
		define.HEADLESS_SLOW_MOTION = envHeadlessSlowMotion
	}
	if args.HEADLESS_SLOW_MOTION > 0 {
		define.HEADLESS_SLOW_MOTION = args.HEADLESS_SLOW_MOTION
	}

	envHeadlessExecTimeout := ConvertStringToPositiveInteger(os.Getenv(ENV_KEY_HEADLESS_EXEC_TIMEOUT))
	if envHeadlessExecTimeout > 0 {
		define.HEADLESS_EXCUTE_TIMEOUT = envHeadlessExecTimeout
	}
	if args.HEADLESS_EXCUTE_TIMEOUT > 0 {
		define.HEADLESS_EXCUTE_TIMEOUT = args.HEADLESS_EXCUTE_TIMEOUT
	}

}
