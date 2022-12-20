package define

import "time"

const GLOBAL_USER_AGENT = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36"
const GLOBAL_REQ_TIMEOUT = 5 * time.Second

const (
	// Use UTF-8 encoding as default and fallback for document detection
	DEFAULT_DOCUMENT_CHARSET = CHARSET_UTF8
	// Use `ssr` as default and fallback for document parsing
	DEFAULT_PARSE_MODE      = PARSE_MODE_SSR
	DEFAULT_RULES_DIRECTORY = "./rules"
	DEFAULT_HTTP_PORT       = 8080
)

const (
	GLOBAL_DEBUG_MODE = true
	// debug logger level: `debug`, `info`, `warn`, `error`
	GLOBAL_DEBUG_LEVEL = "info"
)

const (
	JS_EXECUTE_TIMEOUT    = 200 * time.Millisecond
	JS_EXECUTE_THORTTLING = 2 * time.Second
)

const (
	REDIS_ENABLED       = true
	PROD_REDIS_ADDRESS  = "127.0.0.1:6379"
	PROD_REDIS_PASSWORD = ""
	PROD_REDIS_DB       = 0
	DEV_REDIS_ADDRESS   = "127.0.0.1:6379"
	DEV_REDIS_PASSWORD  = ""
	DEV_REDIS_DB        = 0
)

const (
	MEMORY_CACHE_ENABLED       = true
	IN_MEMORY_CACHE_EXPIRATION = 60 * time.Second
	IN_MEMORY_CACHE_STORE_NAME = "memory_cache"
)
