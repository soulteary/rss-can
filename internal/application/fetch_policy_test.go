package application

import (
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/network"
)

func TestTeamModeCannotSelectTrustedFetchPolicy(t *testing.T) {
	t.Setenv("RSS_ACCESS_MODE", "team")
	t.Setenv("RSS_FETCH_POLICY", "trusted")
	config, err := ConfigFromGlobals()
	if err != nil {
		t.Fatal(err)
	}
	if !config.Network.Policy.PublicOnly() {
		t.Fatal("team mode permitted trusted fetches")
	}
}
func TestInvalidFetchPolicyRejectedAtStartup(t *testing.T) {
	t.Setenv("RSS_FETCH_POLICY", "pubic")
	if _, err := ConfigFromGlobals(); err == nil {
		t.Fatal("misspelled policy accepted")
	}
}
func TestPublicFetchProxyRejectedBeforeRuntimeStartup(t *testing.T) {
	config, err := ConfigFromGlobals()
	if err != nil {
		t.Fatal(err)
	}
	config.Network.Policy = network.PublicPolicy()
	config.Network.Proxy = "http://127.0.0.1:8080"
	if _, err := New(t.Context(), config); err == nil || !strings.Contains(err.Error(), "does not allow a proxy") {
		t.Fatalf("unexpected startup error %v", err)
	}
}
