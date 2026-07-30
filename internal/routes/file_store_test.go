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

func TestFileStorePersistsLLMPolicyAndMigratesV1(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "routes.json")
	digest := capability.Hash("local-llm-api-key")
	route := HTTPRoute{
		Name: "Coding", Alias: "coding", Kind: KindLLM, Provider: ProviderOpenAI,
		TargetSecretRef: "routes/coding", CapabilityDigest: digest,
		Policy: NewLLMPolicy(ProviderOpenAI, []string{"gpt-5.1", "gpt-5.2-codex"}, 8192),
		Egress: "Auto", Enabled: true,
	}
	route.Policy.TrackUsage = true
	route.Usage = LLMUsage{Requests: 9, InputTokens: 1200, OutputTokens: 340}
	store := NewFileStore(path)
	if err := store.Save([]HTTPRoute{route}); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil || len(loaded) != 1 {
		t.Fatalf("Load() = %+v, %v", loaded, err)
	}
	got := loaded[0]
	if got.EffectiveKind() != KindLLM || got.Provider != ProviderOpenAI || !got.Policy.AllowsPath("/v1/responses") || !got.Policy.AllowsModel("gpt-5.2-codex") || got.Policy.MaxOutputTokens != 8192 || got.Policy.MaxRequestBytes != DefaultLLMMaxRequestBytes || got.Policy.RequestsPerMinute != DefaultLLMRequestsPerMinute || got.Policy.MaxConcurrent != DefaultLLMMaxConcurrent || !got.Policy.TrackUsage || got.Usage != (LLMUsage{}) {
		t.Fatalf("loaded LLM route = %+v", got)
	}

	v1Path := filepath.Join(directory, "routes-v1.json")
	v1 := `{"version":1,"routes":[{"name":"legacy","alias":"legacy","target_secret_ref":"routes/legacy","capability_digest":"0000000000000000000000000000000000000000000000000000000000000000","allowed_methods":["GET"],"egress":"Direct","enabled":true}]}`
	if err := os.WriteFile(v1Path, []byte(v1), 0o600); err != nil {
		t.Fatal(err)
	}
	legacy, err := NewFileStore(v1Path).Load()
	if err != nil || len(legacy) != 1 || legacy[0].EffectiveKind() != KindHTTP {
		t.Fatalf("legacy routes = %+v, %v", legacy, err)
	}
}
