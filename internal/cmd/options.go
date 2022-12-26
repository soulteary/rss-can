package cmd

import (
	"os"
	"regexp"
	"strings"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
)

func SantizeFeedPath(feedpath string) string {
	s := "/" + strings.TrimSpace(strings.TrimRight(strings.TrimLeft(feedpath, "/"), "/"))
	var re = regexp.MustCompile(`^\/[\w\d\-\s\_]+$`)
	match := re.FindAllStringSubmatch(s, -1)
	if len(match) == 0 {
		return define.DEFAULT_HTTP_FEED_PATH
	}
	return strings.ToLower(s)
}

func UpdateBoolOption(key string, args bool, defaults bool) bool {
	env := os.Getenv(key)
	if env != "" {
		return fn.IsBoolString(env)
	}
	if args != defaults {
		return args
	}
	return defaults
}

func updateNumberOption(key string, args int, defaults int, allowZero bool) int {
	env := fn.StringToPositiveInteger(os.Getenv(key))

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

func updateStringOption(key string, args string, defaults string) string {
	env := os.Getenv(key)
	if fn.IsNotEmptyAndNotDefaultString(env, defaults) {
		return env
	}
	if fn.IsNotEmptyAndNotDefaultString(args, defaults) {
		return args
	}
	return defaults
}

func updateLogOption(key string, args string, defaults string) string {
	env := os.Getenv(key)
	if fn.IsVaildLogLevel(env) {
		return strings.ToLower(env)
	}

	args = strings.ToLower(args)
	if fn.IsVaildLogLevel(args) && args != defaults {
		return strings.ToLower(args)
	}
	return defaults
}

func updateFeedPathOption(key string, args string, defaults string) string {
	env := SantizeFeedPath(os.Getenv(key))
	if fn.IsNotEmptyAndNotDefaultString(env, defaults) {
		return env
	}
	argHttpFeedPath := SantizeFeedPath(args)
	if fn.IsNotEmptyAndNotDefaultString(argHttpFeedPath, defaults) {
		return argHttpFeedPath
	}
	return defaults
}

func updatePortOption(key string, args int, defaults int) int {
	env := fn.StringToPositiveInteger(os.Getenv(key))
	if fn.IsVaildPortRange(env) {
		return env
	}
	if fn.IsVaildPortRange(args) && args != defaults {
		return args
	}
	return defaults
}
