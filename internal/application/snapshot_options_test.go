package application

import (
	"testing"
	"time"
)

func TestSnapshotConfigurationDefaultsAndValidation(t *testing.T) {
	t.Setenv("RSS_SNAPSHOT_TTL", "")
	t.Setenv("RSS_SNAPSHOT_MAX_STALE", "")
	c, err := ConfigFromGlobals()
	if err != nil {
		t.Fatal(err)
	}
	if c.Snapshots.TTL != 30*time.Second || c.Snapshots.MaxStale != time.Hour {
		t.Fatalf("defaults %+v", c.Snapshots)
	}
	t.Setenv("RSS_SNAPSHOT_TTL", "0")
	c, err = ConfigFromGlobals()
	if err != nil || c.Snapshots.TTL != 0 {
		t.Fatal("cannot disable snapshots")
	}
	for _, bad := range []string{"-1", "3601", "garbage"} {
		t.Setenv("RSS_SNAPSHOT_TTL", bad)
		if _, err := ConfigFromGlobals(); err == nil {
			t.Fatalf("bad ttl %q", bad)
		}
	}
	t.Setenv("RSS_SNAPSHOT_TTL", "30")
	t.Setenv("RSS_SNAPSHOT_MAX_STALE", "86401")
	if _, err := ConfigFromGlobals(); err == nil {
		t.Fatal("bad stale accepted")
	}
}
