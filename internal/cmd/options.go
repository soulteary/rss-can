package cmd

import (
	"os"
	"regexp"
	"strings"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
)

func SantizeFeedPath(feedpath string) string {
	s := "/" + strings.TrimRight(strings.TrimLeft(feedpath, "/"), "/")
	var re = regexp.MustCompile(`^\/[\w\d\-\_]+$`)
	match := re.FindAllStringSubmatch(s, -1)
	if len(match) == 0 {
		return define.DEFAULT_HTTP_FEED_PATH
	}
	return strings.ToLower(s)
}

func updateBoolOption(envKey string, args bool, defaults bool) bool {
	env := os.Getenv(envKey)
	if env != "" {
		return fn.IsBoolString(env)
	}
	if args != defaults {
		return args
	}
	return false
}

func updateNumberOption(envKey string, args int, defaults int, allowZero bool) int {
	env := fn.StringToPositiveInteger(os.Getenv(envKey))

	if allowZero {
		if env >= 0 {
			return env
		}
		if args >= 0 && args != defaults {
			return args
		}
	} else {
		if env > 0 {
			return env
		}
		if args > 0 && args != defaults {
			return args
		}
	}
	return defaults
}

func updateStringOption(envKey string, args string, defaults string) string {
	env := os.Getenv(envKey)
	if fn.IsNotEmptyAndNotDefaultString(env, defaults) {
		return env
	}
	if fn.IsNotEmptyAndNotDefaultString(args, defaults) {
		return args
	}
	return defaults
}

func updateLogOption(envKey string, args string, defaults string) string {
	env := os.Getenv(envKey)
	if fn.IsVaildLogLevel(env) {
		return strings.ToLower(env)
	}

	args = strings.ToLower(args)
	if fn.IsVaildLogLevel(args) && args != defaults {
		return strings.ToLower(args)
	}
	return defaults
}

func updateFeedPathOption(envKey string, args string, defaults string) string {
	env := SantizeFeedPath(os.Getenv(envKey))
	if fn.IsNotEmptyAndNotDefaultString(env, defaults) {
		return env
	}
	argHttpFeedPath := SantizeFeedPath(args)
	if fn.IsNotEmptyAndNotDefaultString(argHttpFeedPath, defaults) {
		return argHttpFeedPath
	}
	return defaults
}

func updatePortOption(envKey string, args int, defaults int) int {
	env := fn.StringToPositiveInteger(os.Getenv(envKey))
	if fn.IsVaildPortRange(env) {
		return env
	}
	if fn.IsVaildPortRange(args) && args != defaults {
		return args
	}
	return defaults
}
