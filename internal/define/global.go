package define

import "time"

const JS_EXECUTE_TIMEOUT = 200 * time.Millisecond
const JS_EXECUTE_THORTTLING = 2 * time.Second

const GLOBAL_USER_AGENT = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36"
const GLOBAL_REQ_TIMEOUT = 5 * time.Second

// Use UTF-8 encoding as default and fallback for document detection
const DEFAULT_DOCUMENT_CHARSET = CHARSET_UTF8

// Use `ssr` as default and fallback for document parsing
const DEFAULT_PARSE_MODE = PARSE_MODE_SSR

const GLOBAL_DEBUG_MODE = true
const GLOBAL_DEBUG_LEVEL = "info"

const DEFAULT_RULES_DIRECTORY = "./rules"

const DEFAULT_HTT_PORT = 8080

const (
	PROD_REDIS_ADDRESS  = "127.0.0.1:6379"
	PROD_REDIS_PASSWORD = ""
	PROD_REDIS_DB = 0
	DEV_REDIS_ADDRESS   = "127.0.0.1:6379"
	DEV_REDIS_PASSWORD  = ""
	DEV_REDIS_DB  = 0
)

const (
	IN_MEMORY_CACHE_EXPIRATION = 60 * time.Second
	IN_MEMORY_CACHE_STORE_NAME = "memory_cache"
)
