package jssdk

import (
	_ "embed"
)

//go:embed js/jquery.min.js
var CSR_SHIM string

//go:embed js/sdk.js
var SDK string

//go:embed js/ssr.js
var SSR_SHIM string
