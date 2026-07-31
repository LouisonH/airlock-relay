package control

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/LouisonH/airlock-relay/internal/egress"
	"github.com/LouisonH/airlock-relay/internal/routes"
	"github.com/LouisonH/airlock-relay/internal/secrets"
)

func TestClientAuthenticatesUnixSocketRequests(t *testing.T) {
	placeholder, err := os.CreateTemp("", "airlock-control-*.sock")
	if err != nil {
		t.Fatal(err)
	}
	socket := placeholder.Name()
	if err := placeholder.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(socket); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
		_ = os.Remove(socket)
	})
	registry, err := routes.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{registry: registry, secrets: secrets.NewMemoryStore(), persistence: routes.NewFileStore(filepath.Join(t.TempDir(), "routes.json")), egress: egress.NewManager(nil), token: "airlock_control_client_token_32_bytes", health: make(map[string]routeHealthState)}
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			server.handle(connection)
		}
	}()
	client, err := NewClient(socket, "airlock_control_client_token_32_bytes")
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(context.Background(), Request{Action: "status"})
	if err != nil || !response.OK {
		t.Fatalf("client response = %+v, %v", response, err)
	}
}
