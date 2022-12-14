package parser

import (
	"strings"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/ssr"
)

func GetWebsiteDataWithConfig(config define.JavaScriptConfig, parseMode string) (result define.BodyParsed) {

	if strings.ToLower(parseMode) != "ssr" {
		// TODO add warning when result length is zero
		return result
	}

	return ssr.GetWebsiteDataWithConfig(config)
}
