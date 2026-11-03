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
	t.Setenv("DENO_DIR", "/must-not-use-shared-cache")
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
	if strings.Contains(string(content), "must-not-cross-process-boundary") || strings.Contains(string(content), "/must-not-use-shared-cache") {
		t.Fatal("application secret or cache override inherited by process")
	}
	var cacheDir string
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "DENO_DIR=") {
			cacheDir = strings.TrimPrefix(line, "DENO_DIR=")
		}
	}
	if cacheDir == "" {
		t.Fatal("missing private runtime cache")
	}
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Fatalf("runtime cache survived exit: %v", err)
	}
}

func TestFailedDenoStartRemovesPrivateCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	runtime := &Runtime{config: RuntimeConfig{Binary: filepath.Join(dir, "missing-deno")}}
	if _, err := startDenoProcess(WithRuntime(t.Context(), runtime)); err == nil {
		t.Fatal("missing executable started")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("failed startup leaked its private cache directory")
	}
}
