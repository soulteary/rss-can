package application

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/access"
	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"github.com/soulteary/RSS-Can/internal/network"
)

func TestDoctorReportsFailuresWithoutSecretsOrServer(t *testing.T) {
	config := Config{Cache: cacher.Config{MemoryEnabled: true}, JavaScript: jssdk.RuntimeConfig{Binary: "/missing/secret-runtime"}}
	results := Diagnose(context.Background(), config)
	failed := 0
	for _, r := range results {
		if !r.OK {
			failed++
		}
		if strings.Contains(r.Message, "secret-runtime") {
			t.Fatal("raw configuration leaked")
		}
	}
	if failed < 2 {
		t.Fatal("missing failures")
	}
}

func TestDoctorRejectsPublicProxyCombination(t *testing.T) {
	cfg := Config{Cache: cacher.Config{MemoryEnabled: true}, JavaScript: jssdk.RuntimeConfig{Binary: "/missing/deno"}}
	cfg.Network.Policy = network.PublicPolicy()
	cfg.Network.Proxy = "http://proxy.example:8080"
	found := false
	for _, r := range Diagnose(t.Context(), cfg) {
		if r.Name == "fetch-policy" {
			found = true
			if r.OK {
				t.Fatal("invalid public proxy reported healthy")
			}
		}
	}
	if !found {
		t.Fatal("fetch policy check missing")
	}
}

func TestDoctorConfigurationHintNeverEchoesInput(t *testing.T) {
	for _, input := range []string{"RSS_JS_WORKERS bad-secret", "RSS_FETCH_POLICY private-secret", "RSS_SNAPSHOT_TTL token-secret", "password=secret"} {
		hint := ConfigurationDiagnostic(errors.New(input))
		if hint.OK || strings.Contains(hint.Message, "secret") {
			t.Fatal("configuration error leaked input")
		}
		if strings.HasPrefix(input, "RSS_JS_WORKERS") && !strings.Contains(hint.Message, "RSS_JS_WORKERS") {
			t.Fatal("missing actionable variable hint")
		}
	}
}

func TestDoctorDoesNotMutateAuthorizationStore(t *testing.T) {
	cfg := teamConfig(t)
	store, err := access.Open(cfg.Server.Access.StorePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.BindIssuer(cfg.Server.Access.WardenURL); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(cfg.Server.Access.StorePath)
	if err != nil {
		t.Fatal(err)
	}
	results := Diagnose(t.Context(), cfg)
	for _, r := range results {
		if !r.OK {
			t.Fatalf("%s: %s", r.Name, r.Message)
		}
	}
	after, _ := os.ReadFile(cfg.Server.Access.StorePath)
	if string(before) != string(after) {
		t.Fatal("doctor changed authorization state")
	}
	publicDir := t.TempDir()
	if err := os.Chmod(publicDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := access.Inspect(filepath.Join(publicDir, "missing.json"), "issuer"); err == nil {
		t.Fatal("nonprivate directory accepted")
	}
}
