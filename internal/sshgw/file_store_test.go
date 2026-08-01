package sshgw

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/LouisonH/airlock-relay/internal/capability"
	"github.com/LouisonH/airlock-relay/internal/secrets"
	"golang.org/x/crypto/ssh"
)

func TestSSHFileStoreRoundTripAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ssh-routes.json")
	store := NewFileStore(path)
	route := Route{
		Name: "Build", Alias: "build", LocalUsername: "builder", TargetSecretRef: "ssh/build",
		CapabilityDigest: capability.Hash("airlock-local-capability"),
		Policy:           NewPolicy([]string{"printf airlock-ok"}, []string{"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}, false),
		Egress:           "Auto", AuthenticationTimeoutSeconds: 37, Enabled: true,
	}
	route.Policy.AllowSFTP = true
	if err := store.Save([]Route{route}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("SSH metadata permissions = %o", info.Mode().Perm())
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].Alias != route.Alias || loaded[0].LocalUsername != "builder" || loaded[0].AuthenticationTimeoutSeconds != 37 || !loaded[0].Policy.AllowsCommand("printf airlock-ok") || !loaded[0].Policy.AllowSFTP || !loaded[0].Enabled {
		t.Fatalf("loaded SSH routes = %+v", loaded)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, protected := range []string{"upstream.invalid", "upstream-user", "upstream-password"} {
		if bytes.Contains(raw, []byte(protected)) {
			t.Fatalf("SSH metadata contains protected value %q", protected)
		}
	}
}

func TestSSHFileStoreLoadsLegacyAliasAsLocalUsername(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ssh-routes.json")
	digest := capability.Hash("legacy-local-capability")
	document := metadataDocument{
		Version: sshMetadataVersion,
		Routes: []persistedRoute{{
			Name: "Legacy", Alias: "legacy", TargetSecretRef: "ssh/legacy",
			CapabilityDigest: hex.EncodeToString(digest[:]),
			AllowedCommands:  []string{"uptime"}, Egress: "Direct", Enabled: true,
		}},
	}
	payload, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewFileStore(path)
	loaded, err := store.Load()
	if err != nil || len(loaded) != 1 {
		t.Fatalf("legacy routes = %+v, %v", loaded, err)
	}
	if loaded[0].LocalUsername != "" || loaded[0].EffectiveLocalUsername() != "legacy" {
		t.Fatalf("legacy username mapping = %+v", loaded[0])
	}
	if loaded[0].AuthenticationTimeoutSeconds != DefaultAuthenticationTimeoutSeconds {
		t.Fatalf("legacy authentication timeout = %d", loaded[0].AuthenticationTimeoutSeconds)
	}
	if loaded[0].Policy.AllowSFTP {
		t.Fatal("legacy route unexpectedly enabled SFTP")
	}
	if err := store.Save(loaded); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(raw, []byte(`"local_username": "legacy"`)) {
		t.Fatalf("upgraded metadata does not contain explicit local username: %s, %v", raw, err)
	}
}

func TestHostIdentityPersistsAcrossRestarts(t *testing.T) {
	store := secrets.NewMemoryStore()
	first, err := LoadOrCreateHostSigner(t.Context(), store)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateHostSigner(t.Context(), store)
	if err != nil {
		t.Fatal(err)
	}
	if ssh.FingerprintSHA256(first.PublicKey()) != ssh.FingerprintSHA256(second.PublicKey()) {
		t.Fatal("SSH host identity changed between loads")
	}
}
