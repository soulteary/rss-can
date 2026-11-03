package application

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/access"
	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/jssdk"
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
	if err := access.Inspect(filepath.Join(t.TempDir(), "missing.json"), "issuer"); err == nil {
		t.Fatal("nonprivate directory accepted")
	}
}
