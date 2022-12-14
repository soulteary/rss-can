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
	jsApp, _ := os.ReadFile("./config/config.js")
	inject := string(jsApp)

	jsConfig, err := javascript.RunCode(inject, "JSON.stringify(getConfig());")
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
