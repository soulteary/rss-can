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

//go:embed js/hep.js
var FILE_HEP_JS string

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

// MIX mode
const TPL_MIX_JS = `()=> document.documentElement.innerHTML`

const CSR_NO_OP_ALERT = `window.alert = () => {};window.prompt = () => {}`

// mix rule with template, generate get config function for RSS Can
func GenerateGetConfigWithRule(rule []byte) string {
	return fmt.Sprintf("var potted = new POTTED();\n%s\n%s", rule, "JSON.stringify(potted.GetConfig());")
}

// mix csr sdk and csr app for cdp client
var GenerateCSRInjectParser = func(app []byte) string {
	return fmt.Sprintf("()=> (function(window){\n%s;\nvar potted = new POTTED();\n%s;\npotted.GetData();return potted.value;})(window)", TPL_CSR_JS, app)
}

// mix hep.js and inspector js for inspector page
var GenerateInspector = func(app []byte) string {
	return fmt.Sprintf("<script>%s;\n%s;\n</script>", FILE_HEP_JS, app)
}
