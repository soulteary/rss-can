package main

import (
	"fmt"

	"github.com/soulteary/RSS-Can/internal/rule"
	"github.com/soulteary/RSS-Can/internal/server"
	"github.com/soulteary/RSS-Can/internal/ssr"
)

func main() {
	rules := rule.LoadRules()
	fmt.Println(rules)

	config, _ := rule.GenerateConfigByRule(rules[0])

	data := ssr.GetWebsiteDataWithConfig(config)
	server.ServAPI(data)
}
