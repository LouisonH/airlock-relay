package routes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LouisonH/airlock-relay/internal/capability"
)

func TestFileStoreRoundTripContainsNoProtectedTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes.json")
	store := NewFileStore(path)
	digest := capability.Hash("airlock_sentinel_capability")
	want := HTTPRoute{
		Name: "Downloads", Alias: "downloads", TargetSecretRef: "routes/downloads",
		CapabilityDigest: digest, Policy: NewHTTPPolicy([]string{"HEAD", "GET"}, []string{"channel"}),
		Egress: "Direct", Enabled: true,
	}
	if err := store.Save([]HTTPRoute{want}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("metadata mode = %o, want 600", info.Mode().Perm())
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "secret.example") || strings.Contains(string(payload), "sentinel-authorization") {
		t.Fatal("route metadata contains protected target data")
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded) != 1 || loaded[0].Alias != want.Alias || loaded[0].CapabilityDigest != digest || !loaded[0].Enabled || !loaded[0].Policy.AllowsQueryKey("channel") {
		t.Fatalf("Load() = %+v", loaded)
	}
}

func TestFileStoreRejectsUnsafeMetadata(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "routes.json")
	payload := `{"version":1,"routes":[{"name":"x","alias":"safe","target_secret_ref":"routes/other","capability_digest":"0000000000000000000000000000000000000000000000000000000000000000","allowed_methods":["GET"],"egress":"Direct","enabled":true}]}`
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileStore(path).Load(); err == nil {
		t.Fatal("Load() accepted a mismatched secret reference")
	}
}
