package jssdk

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDenoEnvironmentDoesNotInheritSecretsOrPermissionOverrides(t *testing.T) {
	t.Setenv("RSS_ADMIN_TOKEN", "secret-admin")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret-cloud")
	t.Setenv("DENO_FLAGS", "--allow-all")
	t.Setenv("NODE_OPTIONS", "--require=malicious")
	t.Setenv("HTTP_PROXY", "http://secret:proxy@localhost")
	t.Setenv("TMPDIR", "/tmp/worker-test")
	env := strings.Join(denoEnvironment(), "\n")
	for _, secret := range []string{"secret-admin", "secret-cloud", "allow-all", "malicious", "secret:proxy"} {
		if strings.Contains(env, secret) {
			t.Fatalf("inherited %s", secret)
		}
	}
	if !strings.Contains(env, "TMPDIR=/tmp/worker-test") {
		t.Fatal("missing required temporary directory")
	}
}

func TestDenoProcessReceivesSanitizedEnvironment(t *testing.T) {
	t.Setenv("RSS_ADMIN_TOKEN", "must-not-cross-process-boundary")
	dir := t.TempDir()
	output := filepath.Join(dir, "environment")
	executable := filepath.Join(dir, "worker")
	script := "#!/bin/sh\n/usr/bin/env > " + strconv.Quote(output) + "\nprintf '{\"ready\":true}\\n'\n"
	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	runtime := &Runtime{config: RuntimeConfig{Binary: executable}}
	process, err := startDenoProcess(WithRuntime(t.Context(), runtime))
	if err != nil {
		t.Fatal(err)
	}
	defer process.stop()
	<-process.done
	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "must-not-cross-process-boundary") {
		t.Fatal("application secret inherited by process")
	}
}
