package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTokenGenerateCreatesProtectedFileWithoutPrintingSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.token")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"token", "generate", "--output", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("token generate = %d: %s", code, stderr.String())
	}
	contents, err := os.ReadFile(path)
	if err != nil || len(strings.TrimSpace(string(contents))) < 32 {
		t.Fatalf("token file = %q, %v", contents, err)
	}
	if strings.Contains(stdout.String(), strings.TrimSpace(string(contents))) {
		t.Fatal("token was printed to stdout")
	}
}

func TestClientRequiresProtectedAbsoluteTokenFile(t *testing.T) {
	_, err := newClient(rootOptions{dataDir: "/tmp/airlock", tokenFile: "relative.token", timeout: 20 * time.Second})
	if err == nil || !strings.Contains(err.Error(), "token-file") {
		t.Fatalf("newClient error = %v", err)
	}
}

func TestRouteCreateRefusesUnconfirmedUnrestrictedSSH(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ssh.json")
	if err := os.WriteFile(path, []byte(`{"allow_all_commands":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runRouteCreate(nil, time.Second, "ssh", []string{"--file", path}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "allow-all-confirmed") {
		t.Fatalf("route create = %d: %s", code, stderr.String())
	}
}
