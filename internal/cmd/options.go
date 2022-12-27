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

func UpdateNumberOption(key string, args int, defaults int, allowZero bool) int {
	env := fn.StringToPositiveInteger(os.Getenv(key))
	num := defaults
	if allowZero {
		if env >= 0 {
			num = env
		}
		if args >= 0 && args != defaults {
			num = args
		}
	} else {
		if env > 0 {
			num = env
		}
		if args > 0 && args != defaults {
			num = args
		}
	}
	return num
}

func UpdateStringOption(key string, args string, defaults string) string {
	env := os.Getenv(key)
	str := defaults
	if fn.IsNotEmptyAndNotDefaultString(env, defaults) {
		str = env
	}
	if fn.IsNotEmptyAndNotDefaultString(args, defaults) {
		str = args
	}
	return str
}

func UpdateLogOption(key string, args string, defaults string) string {
	env := os.Getenv(key)
	level := defaults
	if fn.IsVaildLogLevel(env) {
		level = strings.ToLower(env)
	}

	args = strings.ToLower(args)
	if fn.IsVaildLogLevel(args) && args != defaults {
		level = strings.ToLower(args)
	}
	return level
}

func UpdateFeedPathOption(key string, args string, defaults string) string {
	env := SantizeFeedPath(os.Getenv(key))
	feed := defaults
	if fn.IsNotEmptyAndNotDefaultString(env, defaults) {
		feed = env
	}
	argHttpFeedPath := SantizeFeedPath(args)
	if fn.IsNotEmptyAndNotDefaultString(argHttpFeedPath, defaults) {
		feed = argHttpFeedPath
	}
	return feed
}

func UpdatePortOption(key string, args int, defaults int) int {
	env := fn.StringToPositiveInteger(os.Getenv(key))
	port := defaults
	if fn.IsVaildPortRange(env) {
		port = env
	}
	if fn.IsVaildPortRange(args) && args != defaults {
		port = args
	}
	return port
}

func UpdateHostOption(key string, args string, defaults string) string {
	env := os.Getenv(key)
	str := defaults
	if fn.IsNotEmptyAndNotDefaultString(env, defaults) {
		if fn.IsVaildIPAddr(env) {
			str = env
		}
	}
	if fn.IsNotEmptyAndNotDefaultString(args, defaults) {
		if fn.IsVaildIPAddr(args) {
			str = args
		}
	}
	return str
}
