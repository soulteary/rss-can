package define

// CSR features switch
var (
	CSR_DEBUG              = false
	CSR_INCOGNITO          = false
	CSR_IGNORE_CERT_ERRORS = true
)

// Parser mode types
const (
	DEFAULT_PARSE_MODE = PARSE_MODE_SSR // Use `ssr` as default and fallback for document parsing
	PARSE_MODE_SSR     = "ssr"
	PARSE_MODE_CSR     = "csr"
	PARSE_MODE_MIX     = "mix"
)
