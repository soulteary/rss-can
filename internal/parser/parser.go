package parser

import (
	"github.com/soulteary/RSS-Can/internal/csr"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/ssr"
)

func GetWebsiteDataWithConfig(config define.JavaScriptConfig) (result define.BodyParsed) {
	if config.Mode == "ssr" {
		return ssr.GetWebsiteDataWithConfig(config)
	}

	if config.Mode == "csr" {
		const container = "127.0.0.1:9222"
		const proxy = ""
		return csr.ParsePageByGoRod(config, container, proxy)
	}

	// TODO handle mix, remote ...
	return result
}
