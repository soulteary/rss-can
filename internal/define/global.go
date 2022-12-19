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
