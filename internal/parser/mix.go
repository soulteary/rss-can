package parser

import (
	"github.com/soulteary/RSS-Can/internal/define"
)

func GetDataAndConfigByMix(config define.JavaScriptConfig, container string, proxyAddr string) (result define.BodyParsed) {
	return ParsePageByGoRod(config, container, proxyAddr, true)
}
