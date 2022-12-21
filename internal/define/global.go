package define

var (
	GLOBAL_DEBUG_MODE       = true
	GLOBAL_DEBUG_LEVEL      = "info" // debug logger level: `debug`, `info`, `warn`, `error`
	GLOBAL_USER_AGENT       = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36"
	GLOBAL_REQ_TIMEOUT      = 5 // seconds, fetch remote data timeout
	GLOBAL_SERVER_TIMEOUT   = 8 // seconds, web server request timeout
	DEFAULT_RULES_DIRECTORY = "./rules"
	DEFAULT_HTTP_PORT       = 8080
)

var (
	REDIS_ENABLED       = true
	PROD_REDIS_ADDRESS  = "127.0.0.1:6379"
	PROD_REDIS_PASSWORD = ""
	PROD_REDIS_DB       = 0
	DEV_REDIS_ADDRESS   = "127.0.0.1:6379"
	DEV_REDIS_PASSWORD  = ""
	DEV_REDIS_DB        = 0
)

var (
	MEMORY_CACHE_ENABLED       = true
	IN_MEMORY_CACHE_EXPIRATION = 10 * 60 //seconds, 10min
	IN_MEMORY_CACHE_STORE_NAME = "memory_cache"
)

var (
	DEFAULT_HEADLESS_CHROME    = "127.0.0.1:9222"
	DEFAULT_PROXY_ADDRESS      = ""
	DEFAULT_JS_EXECUTE_TIMEOUT = 200 // milliseconds
	JS_EXECUTE_THORTTLING      = 2   // seconds
	HEADLESS_SLOW_MOTION       = 2   // seconds
	HEADLESS_EXCUTE_TIMEOUT    = 5   // Second
)
