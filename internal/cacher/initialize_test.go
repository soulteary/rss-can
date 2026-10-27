package cacher

import (
	"os"
	"os/exec"
	"testing"

	"github.com/soulteary/RSS-Can/internal/cmd"
)

func TestInitializationUsesRuntimeConfiguration(t *testing.T) {
	if os.Getenv("RSS_TEST_CACHE_CHILD") == "1" {
		if instanceRedis != nil || instanceMemory != nil {
			t.Fatal("cache initialized before runtime configuration")
		}
		cmd.ApplyFlags()
		Initialize()
		options := instanceRedis.Options()
		if options.Addr != "127.0.0.1:16380" || options.Password != "regression-secret" || options.DB != 3 {
			t.Fatalf("runtime config ignored: addr=%s db=%d", options.Addr, options.DB)
		}
		if instanceMemory == nil {
			t.Fatal("memory backend not initialized")
		}
		instanceRedis.Close()
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(executable, "-test.run=^TestInitializationUsesRuntimeConfiguration$")
	child.Env = append(os.Environ(), "RSS_TEST_CACHE_CHILD=1", "RSS_REDIS=true", "RSS_SERVER=127.0.0.1:16380", "RSS_REDIS_PASSWD=regression-secret", "RSS_REDIS_DB=3")
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("configuration subprocess: %v\n%s", err, output)
	}
}
