package application

import (
	"context"
	"os"
	"time"

	"github.com/soulteary/RSS-Can/internal/access"
	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"github.com/soulteary/RSS-Can/internal/network"
)

type Diagnostic struct {
	Name    string
	OK      bool
	Message string
}

// Diagnose is a read-only startup check: it never starts the HTTP listener or
// edits the authorization database. Messages deliberately omit secrets/URLs.
func Diagnose(ctx context.Context, config Config) []Diagnostic {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	results := []Diagnostic{}
	add := func(name string, ok bool, yes, no string) {
		message := yes
		if !ok {
			message = no
		}
		results = append(results, Diagnostic{name, ok, message})
	}
	serverOK := config.Server.Timeout > 0 && config.Server.FeedPath != "" && config.Server.RuleDirectory != ""
	add("server", serverOK, "Server configuration is valid.", "Set a positive timeout, feed path and rule directory.")
	policy, policyErr := network.ParseFetchPolicy(config.Network.Policy.Mode)
	if config.Server.Access != nil {
		policy = network.PublicPolicy()
	}
	proxy, proxyErr := network.NormalizeProxy(config.Network.Proxy)
	add("fetch-policy", policyErr == nil && proxyErr == nil && !(policy.PublicOnly() && proxy != ""), "Fetch policy and proxy configuration are compatible.", "Use a valid fetch policy; public/team mode cannot use a proxy.")
	info, err := os.Stat(config.Server.RuleDirectory)
	rulesOK := err == nil && info.IsDir()
	if rulesOK {
		_, err = os.ReadDir(config.Server.RuleDirectory)
		rulesOK = err == nil
	}
	add("rules", rulesOK, "Rule directory is readable.", "Make the configured rule directory readable by the service user.")
	if config.Server.Access != nil {
		valid := config.Server.Access.Validate() == nil && len(config.Server.AdminToken) >= 32
		add("team", valid, "Team identity configuration is valid; verify proxy-only network access in deployment.", "Team mode needs HTTPS origin, explicit proxy CIDRs, Warden and a 32-character admin token.")
		ok := valid && access.Inspect(config.Server.Access.StorePath, config.Server.Access.WardenURL) == nil
		add("authorization-store", ok, "Authorization storage can be used without changing its contents.", "Use a private 0700 directory and a valid 0600 store for this directory issuer.")
	}
	runtimeConfig := config.JavaScript
	runtimeConfig.Workers = 1
	runtime, err := jssdk.NewRuntime(ctx, runtimeConfig)
	if err == nil {
		err = runtime.Check(ctx)
		_ = runtime.Close()
	}
	add("javascript", err == nil, "JavaScript runtime is ready.", "Install the supported Deno runtime or set RSS_CAN_DENO to its executable.")
	cache := cacher.New(config.Cache)
	checkCtx, checkCancel := context.WithTimeout(ctx, 2*time.Second)
	health := cache.Check(checkCtx)
	checkCancel()
	_ = cache.Close()
	add("cache", health.Ready, "A configured cache backend is ready.", "Enable memory cache or check the Redis connection.")
	return results
}
