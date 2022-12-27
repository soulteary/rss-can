package jssdk

import (
	_ "embed"
	"fmt"
)

//go:embed js/jquery.min.js
var FILE_SHIM_CSR string

//go:embed js/sdk.js
var FILE_SDK string

//go:embed js/ssr.js
var FILE_SHIM_SSR string

//go:embed js/moment.min.js
var FILE_MOMENT_JS string

//go:embed js/date.js
var FILE_DATE_JS string

// combine moment.js with date functions
var TPL_DATE_JS = func() string {
	return fmt.Sprintf("%s\n%s", FILE_MOMENT_JS, FILE_DATE_JS)
}()

// combine sdk and ssr shim for SSR
var TPL_SSR_JS = func() string {
	return fmt.Sprintf("%s\n%s", FILE_SHIM_SSR, FILE_SDK)
}()

// combine sdk and csr shim for CSR
var TPL_CSR_JS = func() string {
	return fmt.Sprintf("%s\n%s", FILE_SHIM_CSR, FILE_SDK)
}()
