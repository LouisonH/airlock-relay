package control

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/LouisonH/airlock-relay/internal/capability"
	"github.com/LouisonH/airlock-relay/internal/egress"
	"github.com/LouisonH/airlock-relay/internal/routes"
	"github.com/LouisonH/airlock-relay/internal/secrets"
	"github.com/LouisonH/airlock-relay/internal/sshgw"
	"golang.org/x/crypto/ssh"
)

type failingMetadataStore struct{}

func (failingMetadataStore) Load() ([]routes.HTTPRoute, error) {
	return nil, errors.New("metadata unavailable")
}

func (failingMetadataStore) Save([]routes.HTTPRoute) error {
	return errors.New("metadata unavailable")
}

type failingDeleteStore struct {
	*secrets.MemoryStore
}

func (failingDeleteStore) DeleteTarget(context.Context, string) error {
	return errors.New("keychain unavailable")
}

func TestProtectedControlChannelCreatesSanitizedRoute(t *testing.T) {
	directory, err := os.MkdirTemp("/tmp", "airlock-control-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	paths := Paths{
		Directory: directory,
		Socket:    filepath.Join(directory, "control.sock"),
	}
	registry, err := routes.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	store := secrets.NewMemoryStore()
	metadata := routes.NewFileStore(filepath.Join(directory, "routes.json"))
	if err := prepareDirectory(paths.Directory); err != nil {
		t.Fatal(err)
	}
	token := "airlock_control_test_token_32_bytes"
	server := &Server{registry: registry, secrets: store, persistence: metadata, egress: egress.NewManager(nil), token: token}
	request := Request{
		Version: protocolVersion,
		Token:   token,
		Action:  "create_http_route",
		Create: &CreateHTTPRoute{
			Name: "Downloads", Alias: "downloads", BaseURL: "https://secret.example/private/",
			Authorization: "Bearer sentinel-secret", Egress: egress.Auto,
		},
	}
	response, raw := sendRequest(t, server, request)
	if !response.OK || response.Created == nil || response.Created.Capability == "" {
		t.Fatalf("create response = %+v", response)
	}
	if strings.Contains(raw, "secret.example") || strings.Contains(raw, "sentinel-secret") {
		t.Fatalf("control response leaked a protected target: %s", raw)
	}
	if response.Created.Route.LocalEndpoint != "127.0.0.1:4768/r/downloads" {
		t.Fatalf("unexpected local endpoint: %s", response.Created.Route.LocalEndpoint)
	}
	if response.Created.Route.Egress != egress.Auto {
		t.Fatalf("egress = %q", response.Created.Route.Egress)
	}
	target, err := store.ResolveHTTPTarget(t.Context(), "routes/downloads")
	if err != nil {
		t.Fatal(err)
	}
	if target.BaseURL.String() != "https://secret.example/private/" || target.Headers.Get("Authorization") != "Bearer sentinel-secret" {
		t.Fatal("protected target was not stored intact")
	}

	response, _ = sendRequest(t, server, Request{Version: protocolVersion, Token: token, Action: "set_route_enabled", Alias: "downloads", Enabled: true})
	if !response.OK || response.Routes[0].Status != "enabled" {
		t.Fatalf("enable response = %+v", response)
	}
	response, _ = sendRequest(t, server, Request{Version: protocolVersion, Token: token, Action: "stop_all"})
	if !response.OK || len(response.Routes) != 1 || response.Routes[0].Status != "disabled" {
		t.Fatalf("stop response = %+v", response)
	}
	response, _ = sendRequest(t, server, Request{Version: protocolVersion, Token: token, Action: "delete_route", Alias: "downloads"})
	if !response.OK || len(response.Routes) != 0 {
		t.Fatalf("delete response = %+v", response)
	}
	if _, err := store.ResolveHTTPTarget(t.Context(), "routes/downloads"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("deleted target error = %v", err)
	}
	loaded, err := metadata.Load()
	if err != nil || len(loaded) != 0 {
		t.Fatalf("persisted routes after delete = %+v, %v", loaded, err)
	}
}

func TestCreateLLMRouteSeparatesLocalAndUpstreamAPIKeys(t *testing.T) {
	directory := t.TempDir()
	registry, _ := routes.NewRegistry()
	store := secrets.NewMemoryStore()
	server := &Server{
		registry: registry, secrets: store,
		persistence: routes.NewFileStore(filepath.Join(directory, "routes.json")),
		egress:      egress.NewManager(nil), token: "airlock_control_test_token_32_bytes",
		httpAddress: "127.0.0.1:4768",
	}
	const localAPIKey = "local-api-key-sentinel-1234"
	const upstreamAPIKey = "upstream-api-key-sentinel"
	response := server.createLLMRoute(&CreateHTTPRoute{
		Name: "Coding", Alias: "coding", BaseURL: "https://llm.private.invalid/gateway/",
		Authorization: upstreamAPIKey, Provider: routes.ProviderOpenAI,
		Models: []string{"gpt-test", "gpt-coding"}, MaxOutputTokens: 8192,
		LocalAPIKey: localAPIKey, Egress: egress.Auto,
	})
	if !response.OK || response.Created == nil || response.Created.Capability != "" {
		t.Fatalf("create LLM response = %+v", response)
	}
	if response.Created.Route.Kind != routes.KindLLM || response.Created.Route.LocalEndpoint != "http://127.0.0.1:4768/r/coding" || !strings.Contains(response.Created.Route.PermissionSummary, "2 models") {
		t.Fatalf("LLM summary = %+v", response.Created.Route)
	}
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"llm.private.invalid", localAPIKey, upstreamAPIKey} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("LLM response leaked %q: %s", secret, raw)
		}
	}
	target, err := store.ResolveHTTPTarget(t.Context(), "routes/coding")
	if err != nil || target.BaseURL.String() != "https://llm.private.invalid/gateway" || target.Headers.Get("Authorization") != "Bearer "+upstreamAPIKey {
		t.Fatalf("protected LLM target = %+v, %v", target, err)
	}
	stored, err := registry.Get("coding")
	if err != nil || capability.Verify(localAPIKey, stored.CapabilityDigest) != nil || !stored.Policy.AllowsModel("gpt-coding") || stored.Policy.MaxOutputTokens != 8192 {
		t.Fatalf("LLM route = %+v, %v", stored, err)
	}
	loaded, err := server.persistence.Load()
	if err != nil || len(loaded) != 1 || loaded[0].Provider != routes.ProviderOpenAI {
		t.Fatalf("persisted LLM route = %+v, %v", loaded, err)
	}
}

func TestCreateLLMRouteGeneratesLocalAPIKeyAndInjectsAnthropicHeaders(t *testing.T) {
	registry, _ := routes.NewRegistry()
	store := secrets.NewMemoryStore()
	server := &Server{
		registry: registry, secrets: store,
		persistence: routes.NewFileStore(filepath.Join(t.TempDir(), "routes.json")),
		egress:      egress.NewManager(nil), token: "airlock_control_test_token_32_bytes",
	}
	response := server.createLLMRoute(&CreateHTTPRoute{
		Name: "Writing", Alias: "writing", BaseURL: "https://api.anthropic.com",
		Authorization: "anthropic-upstream-key", Provider: routes.ProviderAnthropic,
		Models: []string{"claude-test"}, MaxOutputTokens: 4096,
	})
	if !response.OK || response.Created == nil || !strings.HasPrefix(response.Created.Capability, "airlock_") {
		t.Fatalf("generated LLM key response = %+v", response)
	}
	target, err := store.ResolveHTTPTarget(t.Context(), "routes/writing")
	if err != nil || target.Headers.Get("X-Api-Key") != "anthropic-upstream-key" || target.Headers.Get("Anthropic-Version") != "2023-06-01" || target.Headers.Get("Authorization") != "" {
		t.Fatalf("Anthropic target = %+v, %v", target, err)
	}
	route, err := registry.Get("writing")
	if err != nil || capability.Verify(response.Created.Capability, route.CapabilityDigest) != nil {
		t.Fatalf("generated local API key was not stored as a digest: %v", err)
	}
}

func TestCreateLLMRouteRejectsWeakLocalAPIKeyAndInvalidPolicy(t *testing.T) {
	server := &Server{}
	for _, input := range []*CreateHTTPRoute{
		{Name: "Weak", Authorization: "upstream", LocalAPIKey: "too-short"},
		{Name: "Models", Authorization: "upstream", Provider: routes.ProviderOpenAI, Models: []string{""}, MaxOutputTokens: 100},
	} {
		if response := server.createLLMRoute(input); response.OK {
			t.Fatalf("invalid LLM route was accepted: %+v", input)
		}
	}
}

func TestValidLocalAPIKeyRejectsWhitespace(t *testing.T) {
	for _, value := range []string{
		"too-short",
		"sixteen-chars-ok\n",
		"sixteen\u00a0chars-key",
		"sixteen chars key",
	} {
		if validLocalAPIKey(value) {
			t.Fatalf("validLocalAPIKey(%q) = true", value)
		}
	}
	if !validLocalAPIKey("custom-local-api-key-1234") {
		t.Fatal("valid custom local API key was rejected")
	}
}

func TestUpdateLLMPolicyAndRotateLocalAPIKey(t *testing.T) {
	registry, _ := routes.NewRegistry()
	store := secrets.NewMemoryStore()
	metadata := routes.NewFileStore(filepath.Join(t.TempDir(), "routes.json"))
	server := &Server{
		registry: registry, secrets: store, persistence: metadata,
		egress: egress.NewManager(nil), token: "airlock_control_test_token_32_bytes",
		httpAddress: "127.0.0.1:4768",
	}
	const oldKey = "old-local-api-key-sentinel"
	created := server.createLLMRoute(&CreateHTTPRoute{
		Name: "Coding", Alias: "coding", BaseURL: "https://llm.private.invalid",
		Authorization: "upstream-key", Provider: routes.ProviderOpenAI,
		Models: []string{"model-a"}, MaxOutputTokens: 4096, LocalAPIKey: oldKey,
	})
	if !created.OK {
		t.Fatalf("create LLM route = %+v", created)
	}

	updated := server.setLLMPolicy("coding", &CreateHTTPRoute{
		Models: []string{"model-b", "model-c"}, MaxOutputTokens: 8192,
		RequestsPerMinute: 120, MaxConcurrent: 8, TrackUsage: true,
	})
	if !updated.OK || len(updated.Routes) != 1 {
		t.Fatalf("set LLM policy = %+v", updated)
	}
	route, err := registry.Get("coding")
	if err != nil || route.Policy.AllowsModel("model-a") || !route.Policy.AllowsModel("model-b") || route.Policy.MaxOutputTokens != 8192 || route.Policy.RequestsPerMinute != 120 || route.Policy.MaxConcurrent != 8 || !route.Policy.TrackUsage {
		t.Fatalf("updated LLM route = %+v, %v", route, err)
	}
	if got := updated.Routes[0]; got.Provider != routes.ProviderOpenAI || len(got.AllowedModels) != 2 || got.RequestsPerMinute != 120 || got.MaxConcurrent != 8 || !got.TrackUsage {
		t.Fatalf("updated LLM summary = %+v", got)
	}
	if err := registry.RecordLLMUsage("coding", 3, 1200, 400); err != nil {
		t.Fatal(err)
	}
	reset := server.resetLLMUsage("coding")
	if !reset.OK || reset.Routes[0].TotalRequests != 0 || reset.Routes[0].InputTokens != 0 || reset.Routes[0].OutputTokens != 0 {
		t.Fatalf("reset LLM usage = %+v", reset)
	}

	rotated := server.rotateLLMAPIKey("coding", &CreateHTTPRoute{})
	if !rotated.OK || rotated.Created == nil || !strings.HasPrefix(rotated.Created.Capability, "airlock_") {
		t.Fatalf("rotate generated LLM key = %+v", rotated)
	}
	route, err = registry.Get("coding")
	if err != nil || capability.Verify(oldKey, route.CapabilityDigest) == nil || capability.Verify(rotated.Created.Capability, route.CapabilityDigest) != nil {
		t.Fatalf("generated rotated capability was not enforced: %v", err)
	}

	const customKey = "new-custom-local-api-key"
	custom := server.rotateLLMAPIKey("coding", &CreateHTTPRoute{LocalAPIKey: customKey})
	if !custom.OK || custom.Created == nil || custom.Created.Capability != "" {
		t.Fatalf("rotate custom LLM key = %+v", custom)
	}
	route, err = registry.Get("coding")
	if err != nil || capability.Verify(customKey, route.CapabilityDigest) != nil || capability.Verify(rotated.Created.Capability, route.CapabilityDigest) == nil {
		t.Fatalf("custom rotated capability was not enforced: %v", err)
	}
	loaded, err := metadata.Load()
	if err != nil || len(loaded) != 1 || loaded[0].CapabilityDigest != route.CapabilityDigest || loaded[0].Policy.RequestsPerMinute != 120 {
		t.Fatalf("persisted rotated LLM route = %+v, %v", loaded, err)
	}
}

func TestLLMPolicyAndKeyRotationRollbackOnPersistenceFailure(t *testing.T) {
	original := routes.HTTPRoute{
		Name: "Coding", Alias: "coding", Kind: routes.KindLLM, Provider: routes.ProviderOpenAI,
		TargetSecretRef: "routes/coding", CapabilityDigest: capability.Hash("original-local-api-key"),
		Policy: routes.NewLLMPolicy(routes.ProviderOpenAI, []string{"model-a"}, 4096), Enabled: true,
	}
	registry, err := routes.NewRegistry(original)
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{registry: registry, secrets: secrets.NewMemoryStore(), persistence: failingMetadataStore{}, egress: egress.NewManager(nil)}
	if response := server.setLLMPolicy("coding", &CreateHTTPRoute{Models: []string{"model-b"}, MaxOutputTokens: 8192, RequestsPerMinute: 10, MaxConcurrent: 2}); response.OK {
		t.Fatalf("policy update unexpectedly succeeded: %+v", response)
	}
	if response := server.rotateLLMAPIKey("coding", &CreateHTTPRoute{LocalAPIKey: "rotated-local-api-key"}); response.OK {
		t.Fatalf("key rotation unexpectedly succeeded: %+v", response)
	}
	got, err := registry.Get("coding")
	if err != nil || got.CapabilityDigest != original.CapabilityDigest || !got.Policy.AllowsModel("model-a") || got.Policy.MaxOutputTokens != 4096 {
		t.Fatalf("rollback route = %+v, %v", got, err)
	}
}

func TestProtectedProxyConfigurationNeverReturnsURL(t *testing.T) {
	registry, _ := routes.NewRegistry()
	store := secrets.NewMemoryStore()
	manager := egress.NewManager(nil)
	server := &Server{
		registry: registry, secrets: store, persistence: routes.NewFileStore(filepath.Join(t.TempDir(), "routes.json")),
		egress: manager, token: "airlock_control_test_token_32_bytes",
	}
	request := Request{
		Version: protocolVersion, Token: server.token, Action: "configure_proxy",
		ProxyURL: "socks5://proxy-user:proxy-secret-sentinel@127.0.0.1:7890",
	}
	response, raw := sendRequest(t, server, request)
	if !response.OK || !response.ProxyConfigured || !manager.Configured() {
		t.Fatalf("configure response = %+v", response)
	}
	if strings.Contains(raw, "127.0.0.1:7890") || strings.Contains(raw, "proxy-secret-sentinel") {
		t.Fatalf("control response leaked protected proxy: %s", raw)
	}
	config, err := store.ResolveProxyConfig(t.Context(), egress.DefaultSecretReference)
	if err != nil || config.URL.String() != request.ProxyURL {
		t.Fatalf("stored proxy = %v, %v", config.URL, err)
	}

	response, _ = sendRequest(t, server, Request{Version: protocolVersion, Token: server.token, Action: "clear_proxy"})
	if !response.OK || response.ProxyConfigured || manager.Configured() {
		t.Fatalf("clear response = %+v", response)
	}
	if _, err := store.ResolveProxyConfig(t.Context(), egress.DefaultSecretReference); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("cleared proxy error = %v", err)
	}
}

func TestCreateSSHRoutePersistsOnlyProtectedReference(t *testing.T) {
	directory := t.TempDir()
	registry, _ := routes.NewRegistry()
	sshRegistry, _ := sshgw.NewRegistry()
	store := secrets.NewMemoryStore()
	sshMetadata := sshgw.NewFileStore(filepath.Join(directory, "ssh-routes.json"))
	privateKey := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	hostKey, err := ssh.NewPublicKey(privateKey.Public())
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{
		registry: registry, secrets: store,
		persistence: routes.NewFileStore(filepath.Join(directory, "routes.json")),
		egress:      egress.NewManager(nil), token: "airlock_control_test_token_32_bytes",
		ssh: &SSHConfiguration{
			Registry: sshRegistry, Persistence: sshMetadata,
			ListenAddress: "127.0.0.1:4770", HostKey: hostKey,
		},
	}
	response := server.createSSHRoute(&CreateSSHRoute{
		Name: "Build", Alias: "build", Address: "ssh.private.invalid",
		Username: "upstream-user-sentinel", Password: "upstream-password-sentinel",
		ExpectedHostKey: base64.StdEncoding.EncodeToString(hostKey.Marshal()),
		AllowedCommand:  "printf airlock-ok", RecordCommands: true, Egress: egress.Auto,
	})
	if !response.OK || response.Created == nil || response.Created.Capability == "" {
		t.Fatalf("create SSH response = %+v", response)
	}
	if response.Created.Route.LocalEndpoint != "build@127.0.0.1:4770" || response.Created.Route.Kind != "SSH" || response.Created.Route.Status != "disabled" {
		t.Fatalf("created SSH summary = %+v", response.Created.Route)
	}
	if response.Created.Route.AllowedCommand != "printf airlock-ok" {
		t.Fatalf("created SSH allowed command = %q", response.Created.Route.AllowedCommand)
	}
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	for _, protected := range []string{"ssh.private.invalid", "upstream-user-sentinel", "upstream-password-sentinel"} {
		if strings.Contains(string(raw), protected) {
			t.Fatalf("SSH control response leaked %q: %s", protected, raw)
		}
	}
	target, err := store.ResolveSSHTarget(t.Context(), "ssh/build")
	if err != nil || target.Address != "ssh.private.invalid:22" || target.Username != "upstream-user-sentinel" || string(target.Password) != "upstream-password-sentinel" {
		t.Fatalf("protected SSH target = %+v, %v", target, err)
	}
	loaded, err := sshMetadata.Load()
	if err != nil || len(loaded) != 1 || loaded[0].TargetSecretRef != "ssh/build" || !loaded[0].Policy.AllowsCommand("printf airlock-ok") {
		t.Fatalf("persisted SSH routes = %+v, %v", loaded, err)
	}
	updated := server.setSSHPolicy("build", &SSHPolicyUpdate{AllowedCommand: "uname -a", RecordCommands: true})
	if !updated.OK || len(updated.Routes) != 1 || updated.Routes[0].AllowedCommand != "uname -a" || updated.Routes[0].AllowAllCommands {
		t.Fatalf("updated exact-command policy response = %+v", updated)
	}
	loaded, err = sshMetadata.Load()
	if err != nil || len(loaded) != 1 || !loaded[0].Policy.AllowsCommand("uname -a") || loaded[0].Policy.AllowsCommand("printf airlock-ok") {
		t.Fatalf("persisted exact-command policy = %+v, %v", loaded, err)
	}
	updated = server.setSSHPolicy("build", &SSHPolicyUpdate{AllowAllCommands: true, RecordCommands: true})
	if !updated.OK || len(updated.Routes) != 1 || !updated.Routes[0].AllowAllCommands || !updated.Routes[0].RecordCommands {
		t.Fatalf("updated SSH policy response = %+v", updated)
	}
	loaded, err = sshMetadata.Load()
	if err != nil || len(loaded) != 1 || !loaded[0].Policy.AllowsCommand("uname -a") || !loaded[0].Policy.RecordCommands {
		t.Fatalf("persisted all-command policy = %+v, %v", loaded, err)
	}
	if enabled := server.setRouteEnabled("build", true); !enabled.OK || enabled.Routes[0].Status != "enabled" {
		t.Fatalf("enable SSH response = %+v", enabled)
	}
	if deleted := server.deleteRoute("build"); !deleted.OK || len(deleted.Routes) != 0 {
		t.Fatalf("delete SSH response = %+v", deleted)
	}
	if _, err := store.ResolveSSHTarget(t.Context(), "ssh/build"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("deleted SSH target error = %v", err)
	}

	const localPassword = "local-password-sentinel"
	custom := server.createSSHRoute(&CreateSSHRoute{
		Name: "Custom", Alias: "custom", Address: "ssh.private.invalid",
		Username: "upstream-user", Password: "upstream-password", LocalPassword: localPassword,
		ExpectedHostKey: base64.StdEncoding.EncodeToString(hostKey.Marshal()),
		AllowedCommand:  "printf airlock-ok", Egress: egress.Direct,
	})
	if !custom.OK || custom.Created == nil || custom.Created.Capability != "" {
		t.Fatalf("custom local password response = %+v", custom)
	}
	customRoute, err := sshRegistry.Get("custom")
	if err != nil || capability.Verify(localPassword, customRoute.CapabilityDigest) != nil {
		t.Fatalf("custom local password was not stored as a valid digest: %v", err)
	}
	raw, err = json.Marshal(custom)
	if err != nil || strings.Contains(string(raw), localPassword) {
		t.Fatalf("custom local password leaked in control response: %s, %v", raw, err)
	}
}

func TestCreateSSHRouteRejectsWeakCustomLocalPassword(t *testing.T) {
	server := &Server{ssh: &SSHConfiguration{}}
	response := server.createSSHRoute(&CreateSSHRoute{Name: "Weak", LocalPassword: "short"})
	if response.OK || response.Error != "invalid SSH route details" {
		t.Fatalf("weak local password response = %+v", response)
	}
}

func TestControlChannelRejectsInvalidToken(t *testing.T) {
	registry, _ := routes.NewRegistry()
	server := &Server{registry: registry, secrets: secrets.NewMemoryStore(), token: "expected-token"}
	response := server.dispatch(Request{Version: protocolVersion, Token: "wrong-token", Action: "status"})
	if response.OK || response.Error != "control authentication failed" {
		t.Fatalf("response = %+v", response)
	}
}

func TestSecretStoreMigrationCopiesVerifiesThenCleansPreviousStore(t *testing.T) {
	directory := t.TempDir()
	registry, _ := routes.NewRegistry()
	sshRegistry, _ := sshgw.NewRegistry()
	source := secrets.NewMemoryStore()
	destination := secrets.NewMemoryStore()
	server := &Server{
		registry: registry, secrets: source,
		persistence: routes.NewFileStore(filepath.Join(directory, "routes.json")),
		egress:      egress.NewManager(nil), token: "airlock_control_test_token_32_bytes",
		ssh: &SSHConfiguration{Registry: sshRegistry, Persistence: sshgw.NewFileStore(filepath.Join(directory, "ssh-routes.json")), ListenAddress: "127.0.0.1:4770"},
	}
	httpCreated := server.createHTTPRoute(&CreateHTTPRoute{Name: "HTTP", Alias: "http", BaseURL: "https://secret.invalid/base/"})
	if !httpCreated.OK {
		t.Fatalf("create HTTP route = %+v", httpCreated)
	}
	_, digest, err := capability.Generate()
	if err != nil {
		t.Fatal(err)
	}
	sshRoute := sshgw.Route{
		Name: "SSH", Alias: "ssh", TargetSecretRef: "ssh/ssh", CapabilityDigest: digest,
		Policy: sshgw.NewPolicy([]string{"true"}, nil, false), Egress: egress.Direct,
	}
	if err := sshRegistry.Upsert(sshRoute); err != nil {
		t.Fatal(err)
	}
	privateKey := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	hostKey, err := ssh.NewPublicKey(privateKey.Public())
	if err != nil {
		t.Fatal(err)
	}
	if err := source.PutSSHTarget(t.Context(), "ssh/ssh", secrets.SSHTarget{
		Address: "127.0.0.1:22", Username: "upstream", Password: []byte("upstream-password"), ExpectedHostKey: hostKey.Marshal(),
	}); err != nil {
		t.Fatal(err)
	}
	proxyURL, _ := url.Parse("socks5://127.0.0.1:7890")
	if err := source.PutProxyConfig(t.Context(), egress.DefaultSecretReference, secrets.ProxyConfig{URL: proxyURL}); err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(privateKey, "Airlock test host")
	if err != nil {
		t.Fatal(err)
	}
	if err := source.PutSSHHostIdentity(t.Context(), sshgw.HostIdentitySecretReference, secrets.SSHHostIdentity{PrivateKey: pem.EncodeToMemory(block)}); err != nil {
		t.Fatal(err)
	}
	if err := server.ConfigureSecretStoreMigration(secrets.StoreModeKeychain, func(mode string) (secrets.MutableStore, error) {
		if mode != secrets.StoreModeLocalFile {
			return nil, errors.New("unexpected mode")
		}
		return destination, nil
	}); err != nil {
		t.Fatal(err)
	}
	if response := server.migrateSecretStore(secrets.StoreModeLocalFile); !response.OK {
		t.Fatalf("migration response = %+v", response)
	}
	if _, err := destination.ResolveHTTPTarget(t.Context(), "routes/http"); err != nil {
		t.Fatalf("migrated HTTP target = %v", err)
	}
	if _, err := destination.ResolveSSHTarget(t.Context(), "ssh/ssh"); err != nil {
		t.Fatalf("migrated SSH target = %v", err)
	}
	if _, err := destination.ResolveProxyConfig(t.Context(), egress.DefaultSecretReference); err != nil {
		t.Fatalf("migrated proxy = %v", err)
	}
	if _, err := destination.ResolveSSHHostIdentity(t.Context(), sshgw.HostIdentitySecretReference); err != nil {
		t.Fatalf("migrated host identity = %v", err)
	}
	if _, err := source.ResolveHTTPTarget(t.Context(), "routes/http"); err != nil {
		t.Fatalf("source was removed before restart: %v", err)
	}

	server.secrets = destination
	server.secretMode = secrets.StoreModeLocalFile
	server.storeFactory = func(mode string) (secrets.MutableStore, error) { return source, nil }
	if response := server.cleanupSecretStore(secrets.StoreModeKeychain); !response.OK || response.Warning != "" {
		t.Fatalf("cleanup response = %+v", response)
	}
	if _, err := source.ResolveHTTPTarget(t.Context(), "routes/http"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("previous HTTP target still exists: %v", err)
	}
	if _, err := source.ResolveSSHTarget(t.Context(), "ssh/ssh"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("previous SSH target still exists: %v", err)
	}
	if _, err := source.ResolveProxyConfig(t.Context(), egress.DefaultSecretReference); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("previous proxy still exists: %v", err)
	}
	if _, err := source.ResolveSSHHostIdentity(t.Context(), sshgw.HostIdentitySecretReference); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("previous host identity still exists: %v", err)
	}
}

func TestCommandPreviewIsBoundedAndValidUTF8(t *testing.T) {
	command := strings.Repeat("测", 300)
	preview := commandPreview(command)
	if !strings.HasSuffix(preview, "... (truncated)") || len(preview) > 540 || !utf8.ValidString(preview) {
		t.Fatalf("command preview = %q (%d bytes)", preview, len(preview))
	}
}

func TestPersistenceFailuresRollBackRouteMutations(t *testing.T) {
	route := testRoute("downloads", true)
	registry, err := routes.NewRegistry(route)
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{
		registry: registry, secrets: secrets.NewMemoryStore(),
		persistence: failingMetadataStore{}, token: "airlock_control_test_token_32_bytes",
	}

	response := server.setRouteEnabled(route.Alias, false)
	if response.OK || response.Error != "could not persist route status" {
		t.Fatalf("set enabled response = %+v", response)
	}
	assertRouteEnabled(t, registry, route.Alias, true)

	response = server.stopAll()
	if response.OK || response.Error != "could not persist stopped routes" {
		t.Fatalf("stop all response = %+v", response)
	}
	assertRouteEnabled(t, registry, route.Alias, true)

	response = server.deleteRoute(route.Alias)
	if response.OK || response.Error != "could not persist route deletion" {
		t.Fatalf("delete response = %+v", response)
	}
	assertRouteEnabled(t, registry, route.Alias, true)
}

func TestCreatePersistenceFailureRemovesRouteAndProtectedTarget(t *testing.T) {
	registry, _ := routes.NewRegistry()
	store := secrets.NewMemoryStore()
	server := &Server{
		registry: registry, secrets: store,
		persistence: failingMetadataStore{}, token: "airlock_control_test_token_32_bytes",
	}
	response := server.createHTTPRoute(&CreateHTTPRoute{
		Name: "Downloads", Alias: "downloads", BaseURL: "https://secret.example/private/",
		Authorization: "Bearer sentinel-secret",
	})
	if response.OK || response.Error != "could not persist route metadata" {
		t.Fatalf("create response = %+v", response)
	}
	if _, err := registry.Get("downloads"); !errors.Is(err, routes.ErrNotFound) {
		t.Fatalf("registry route error = %v", err)
	}
	if _, err := store.ResolveHTTPTarget(t.Context(), "routes/downloads"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("protected target error = %v", err)
	}
}

func TestDeleteCleanupFailureReturnsWarningWithoutRestoringCapability(t *testing.T) {
	route := testRoute("downloads", true)
	registry, _ := routes.NewRegistry(route)
	metadata := routes.NewFileStore(filepath.Join(t.TempDir(), "routes.json"))
	store := failingDeleteStore{MemoryStore: secrets.NewMemoryStore()}
	server := &Server{
		registry: registry, secrets: store, persistence: metadata,
		token: "airlock_control_test_token_32_bytes",
	}

	response := server.deleteRoute(route.Alias)
	if !response.OK || response.Warning == "" {
		t.Fatalf("delete response = %+v", response)
	}
	if _, err := registry.Lookup(route.Alias); !errors.Is(err, routes.ErrNotFound) {
		t.Fatalf("deleted capability route error = %v", err)
	}
	loaded, err := metadata.Load()
	if err != nil || len(loaded) != 0 {
		t.Fatalf("persisted routes = %+v, %v", loaded, err)
	}
}

func testRoute(alias string, enabled bool) routes.HTTPRoute {
	return routes.HTTPRoute{
		Name: "Downloads", Alias: alias, TargetSecretRef: "routes/" + alias,
		CapabilityDigest: capability.Hash("sentinel-capability"),
		Policy:           routes.NewHTTPPolicy([]string{http.MethodGet}, nil),
		Egress:           "Direct", Enabled: enabled,
	}
}

func assertRouteEnabled(t *testing.T, registry *routes.Registry, alias string, want bool) {
	t.Helper()
	route, err := registry.Get(alias)
	if err != nil || route.Enabled != want {
		t.Fatalf("route enabled = %v, error = %v, want %v", route.Enabled, err, want)
	}
}

func sendRequest(t *testing.T, server *Server, request Request) (Response, string) {
	t.Helper()
	client, serverConnection := net.Pipe()
	defer client.Close()
	go server.handle(serverConnection)
	if err := json.NewEncoder(client).Encode(request); err != nil {
		t.Fatal(err)
	}
	raw, err := bufio.NewReader(client).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	var response Response
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		t.Fatal(err)
	}
	return response, raw
}
