package control

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LouisonH/airlock-relay/internal/routes"
	"github.com/LouisonH/airlock-relay/internal/secrets"
)

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
	if err := prepareDirectory(paths.Directory); err != nil {
		t.Fatal(err)
	}
	token := "airlock_control_test_token_32_bytes"
	server := &Server{registry: registry, secrets: store, token: token}
	request := Request{
		Version: protocolVersion,
		Token:   token,
		Action:  "create_http_route",
		Create: &CreateHTTPRoute{
			Name: "Downloads", Alias: "downloads", BaseURL: "https://secret.example/private/",
			Authorization: "Bearer sentinel-secret",
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
	target, err := store.ResolveHTTPTarget(t.Context(), "routes/downloads")
	if err != nil {
		t.Fatal(err)
	}
	if target.BaseURL.String() != "https://secret.example/private/" || target.Headers.Get("Authorization") != "Bearer sentinel-secret" {
		t.Fatal("protected target was not stored intact")
	}

	response, _ = sendRequest(t, server, Request{Version: protocolVersion, Token: token, Action: "stop_all"})
	if !response.OK || len(response.Routes) != 1 || response.Routes[0].Status != "disabled" {
		t.Fatalf("stop response = %+v", response)
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
