package control

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LouisonH/airlock-relay/internal/capability"
	"github.com/LouisonH/airlock-relay/internal/egress"
	"github.com/LouisonH/airlock-relay/internal/routes"
	"github.com/LouisonH/airlock-relay/internal/secrets"
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

func TestControlChannelRejectsInvalidToken(t *testing.T) {
	registry, _ := routes.NewRegistry()
	server := &Server{registry: registry, secrets: secrets.NewMemoryStore(), token: "expected-token"}
	response := server.dispatch(Request{Version: protocolVersion, Token: "wrong-token", Action: "status"})
	if response.OK || response.Error != "control authentication failed" {
		t.Fatalf("response = %+v", response)
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
