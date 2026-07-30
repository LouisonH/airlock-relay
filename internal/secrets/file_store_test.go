package secrets

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestFileStorePersistsWithUserOnlyPermissions(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "airlock")
	path := filepath.Join(directory, "protected-targets.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	proxyURL, _ := url.Parse("socks5://127.0.0.1:7890")
	if err := store.PutProxyConfig(t.Context(), "egress/default", ProxyConfig{URL: proxyURL}); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := reopened.ResolveProxyConfig(t.Context(), "egress/default")
	if err != nil || resolved.URL.String() != proxyURL.String() {
		t.Fatalf("resolved proxy = %+v, %v", resolved, err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil || fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("file permissions = %v, %v", fileInfo.Mode().Perm(), err)
	}
	directoryInfo, err := os.Stat(directory)
	if err != nil || directoryInfo.Mode().Perm() != 0o700 {
		t.Fatalf("directory permissions = %v, %v", directoryInfo.Mode().Perm(), err)
	}
	if err := reopened.DeleteTarget(t.Context(), "egress/default"); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.ResolveProxyConfig(t.Context(), "egress/default"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted proxy error = %v", err)
	}
}

func TestFileStoreRejectsOverexposedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "protected-targets.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"entries":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileStore(path); err == nil {
		t.Fatal("NewFileStore accepted a world-readable secret file")
	}
}
