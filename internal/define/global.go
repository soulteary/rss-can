package define

var (
	DEBUG_MODE      = false
	DEBUG_LEVEL     = "info"
	USER_AGENT      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36"
	REQUEST_TIMEOUT = 45 // seconds, fetch remote data timeout
	SERVER_TIMEOUT  = 50 // seconds, web server request timeout
	RULES_DIRECTORY = "./rules"
	HTTP_HOST       = "0.0.0.0"
	HTTP_PORT       = 8080
	HTTP_FEED_PATH  = "/feed"

	REDIS        = true
	REDIS_SERVER = "127.0.0.1:6379"
	REDIS_PASS   = ""
	REDIS_DB     = 0

	IN_MEMORY_CACHE      = true
	IN_MEMORY_EXPIRATION = 10 * 60 //seconds, 10min

	HEADLESS_SERVER         = "127.0.0.1:9222"
	PROXY_SERVER            = ""
	JS_EXECUTE_TIMEOUT      = 200 // milliseconds
	HEADLESS_SLOW_MOTION    = 2   // seconds
	HEADLESS_EXCUTE_TIMEOUT = 10  // Second
)
