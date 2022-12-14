package main

import (
	"fmt"

	"github.com/soulteary/RSS-Can/internal/parser"
	"github.com/soulteary/RSS-Can/internal/rule"
	"github.com/soulteary/RSS-Can/internal/server"
)

func main() {
	rules := rule.LoadRules()
	fmt.Println(rules)

	config, _ := rule.GenerateConfigByRule(rules[0])

	data := parser.GetWebsiteDataWithConfig(config)
	server.ServAPI(data)
}
