package cmd_test

import (
	"os"
	"testing"

	"github.com/soulteary/RSS-Can/internal/cmd"
	"github.com/soulteary/RSS-Can/internal/define"
)

func TestApplyFlags(t *testing.T) {
	cmd.ApplyFlags()

	if define.DEBUG_MODE != define.DEFAULT_DEBUG_MODE {
		t.Fatal("test flag failed")
	}
	if define.DEBUG_LEVEL != define.DEFAULT_DEBUG_LEVEL {
		t.Fatal("test flag failed")
	}
	if define.REQUEST_TIMEOUT != define.DEFAULT_REQUEST_TIMEOUT {
		t.Fatal("test flag failed")
	}
	if define.SERVER_TIMEOUT != define.DEFAULT_SERVER_TIMEOUT {
		t.Fatal("test flag failed")
	}
	if define.RULES_DIRECTORY != define.DEFAULT_RULES_DIRECTORY {
		t.Fatal("test flag failed")
	}
	if define.HTTP_HOST != define.DEFAULT_HTTP_HOST {
		t.Fatal("test flag failed")
	}
	if define.HTTP_PORT != define.DEFAULT_HTTP_PORT {
		t.Fatal("test flag failed")
	}
	if define.HTTP_FEED_PATH != define.DEFAULT_HTTP_FEED_PATH {
		t.Fatal("test flag failed")
	}
	if define.REDIS != define.DEFAULT_REDIS {
		t.Fatal("test flag failed")
	}
	envRedisServer := os.Getenv(cmd.ENV_KEY_REDIS_SERVER)
	if envRedisServer == "" {
		if define.REDIS_SERVER != define.DEFAULT_REDIS_SERVER {
			t.Fatal("test flag failed")
		}
	} else {
		if define.REDIS_SERVER != envRedisServer {
			t.Fatal("test flag failed")
		}
	}
	if define.REDIS_PASS != define.DEFAULT_REDIS_PASS {
		t.Fatal("test flag failed")
	}
	if define.REDIS_DB != define.DEFAULT_REDIS_DB {
		t.Fatal("test flag failed")
	}
	if define.IN_MEMORY_CACHE != define.DEFAULT_IN_MEMORY_CACHE {
		t.Fatal("test flag failed")
	}
	if define.IN_MEMORY_EXPIRATION != define.DEFAULT_IN_MEMORY_CACHE_EXPIRATION {
		t.Fatal("test flag failed")
	}
	if define.HEADLESS_SERVER != define.DEFAULT_HEADLESS_SERVER {
		t.Fatal("test flag failed")
	}
	if define.PROXY_SERVER != define.DEFAULT_PROXY_ADDRESS {
		t.Fatal("test flag failed")
	}
	if define.JS_EXECUTE_TIMEOUT != define.DEFAULT_JS_EXECUTE_TIMEOUT {
		t.Fatal("test flag failed")
	}
	if define.HEADLESS_SLOW_MOTION != define.DEFAULT_HEADLESS_SLOW_MOTION {
		t.Fatal("test flag failed")
	}
	if define.HEADLESS_EXCUTE_TIMEOUT != define.DEFAULT_HEADLESS_EXCUTE_TIMEOUT {
		t.Fatal("test flag failed")
	}
}
