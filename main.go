package main

import (
	"fmt"
	"os"

	"github.com/soulteary/RSS-Can/internal/javascript"
	"github.com/soulteary/RSS-Can/internal/parser"
	"github.com/soulteary/RSS-Can/internal/server"
	"github.com/soulteary/RSS-Can/internal/ssr"
)

func main() {
	jsSSR, _ := os.ReadFile("./internal/jssdk/ssr.js")
	jsSDK, _ := os.ReadFile("./internal/jssdk/sdk.js")
	base := fmt.Sprintf("%s\n%s\n", jsSSR, jsSDK)

	jsApp, _ := os.ReadFile("./config/config.js")
	inject := fmt.Sprintf("var potted = new POTTED();\n%s\n%s", jsApp, "JSON.stringify(potted.GetConfig());")

	jsConfig, err := javascript.RunCode(base, inject)
	if err != nil {
		fmt.Println(err)
		return
	}

	config, err := parser.ParseConfigFromJSON(jsConfig)
	if err != nil {
		fmt.Println(err)
		return
	}

	data := ssr.GetWebsiteDataWithConfig(config)
	server.ServAPI(data)
}
