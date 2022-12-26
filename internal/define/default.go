package define

const (
	DEFAULT_DEBUG_MODE      = false
	DEFAULT_DEBUG_LEVEL     = "info" // debug logger level: `debug`, `info`, `warn`, `error`
	DEFAULT_USER_AGENT      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36"
	DEFAULT_REQUEST_TIMEOUT = 5 // seconds, fetch remote data timeout
	DEFAULT_SERVER_TIMEOUT  = 8 // seconds, web server request timeout
	DEFAULT_RULES_DIRECTORY = "./rules"
	DEFAULT_HTTP_HOST       = "0.0.0.0"
	DEFAULT_HTTP_PORT       = 8080
	DEFAULT_HTTP_FEED_PATH  = "/feed"
)

const (
	DEFAULT_REDIS        = true
	DEFAULT_REDIS_SERVER = "127.0.0.1:6379"
	DEFAULT_REDIS_PASS   = ""
	DEFAULT_REDIS_DB     = 0
)

const (
	DEFAULT_IN_MEMORY_CACHE            = true
	DEFAULT_IN_MEMORY_CACHE_EXPIRATION = 10 * 60 //seconds, 10min
	DEFAULT_IN_MEMORY_CACHE_STORE_NAME = "memory_cache"
)

const (
	DEFAULT_HEADLESS_SERVER         = "127.0.0.1:9222"
	DEFAULT_PROXY_ADDRESS           = ""
	DEFAULT_JS_EXECUTE_TIMEOUT      = 200 // milliseconds
	DEFAULT_JS_EXECUTE_THORTTLING   = 2   // seconds
	DEFAULT_HEADLESS_SLOW_MOTION    = 2   // seconds
	DEFAULT_HEADLESS_EXCUTE_TIMEOUT = 5   // Second
)
