package parser

import (
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/ssr"
)

func GetWebsiteDataWithConfig(config define.JavaScriptConfig) (result define.BodyParsed) {
	if config.Mode == "ssr" {
		return ssr.GetWebsiteDataWithConfig(config)
	}
	// TODO handle csr, remote ...
	return result
}
