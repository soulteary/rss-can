package access

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectIsReadOnlyAndRejectsUnsafeStores(t *testing.T) {
	s := testStore(t)
	if err := s.BindIssuer("https://warden.example"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(s.path)
	if err := Inspect(s.path, "https://warden.example"); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(s.path)
	if string(before) != string(after) {
		t.Fatal("inspection mutated store")
	}
	if Inspect(s.path, "https://other.example") == nil {
		t.Fatal("wrong issuer accepted")
	}
	if Inspect("relative.json", "") == nil {
		t.Fatal("relative path accepted")
	}
	if err := os.Chmod(s.path, 0644); err != nil {
		t.Fatal(err)
	}
	if Inspect(s.path, "https://warden.example") == nil {
		t.Fatal("publicly readable store accepted")
	}
	_ = os.Chmod(s.path, 0600)
	link := filepath.Join(filepath.Dir(s.path), "link")
	if err := os.Symlink(s.path, link); err != nil {
		t.Fatal(err)
	}
	if Inspect(link, "") == nil {
		t.Fatal("symlink accepted")
	}
	if err := os.WriteFile(s.path, []byte(`{"bad":"schema"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if Inspect(s.path, "") == nil {
		t.Fatal("invalid schema accepted")
	}
	missing := filepath.Join(filepath.Dir(s.path), "new.json")
	if Inspect(missing, "") != nil {
		t.Fatal("valid new location rejected")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("inspection created store")
	}
}
