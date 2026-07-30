package secrets

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/pem"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"golang.org/x/crypto/ssh"
)

type memoryKeychainBackend struct {
	data map[string][]byte
}

func newMemoryKeychainBackend() *memoryKeychainBackend {
	return &memoryKeychainBackend{data: make(map[string][]byte)}
}

func (b *memoryKeychainBackend) Put(reference string, data []byte) error {
	b.data[reference] = append([]byte(nil), data...)
	return nil
}

func (b *memoryKeychainBackend) Get(reference string) ([]byte, error) {
	data, ok := b.data[reference]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), data...), nil
}

func (b *memoryKeychainBackend) Delete(reference string) error {
	if _, ok := b.data[reference]; !ok {
		return ErrNotFound
	}
	delete(b.data, reference)
	return nil
}

func TestKeychainStoreRoundTrip(t *testing.T) {
	backend := newMemoryKeychainBackend()
	store := newKeychainStore(backend)
	baseURL, err := url.Parse("https://private.example/api/")
	if err != nil {
		t.Fatal(err)
	}
	target := HTTPTarget{
		BaseURL: baseURL,
		Headers: http.Header{"Authorization": {"Bearer keychain-secret-sentinel"}},
	}
	if err := store.PutHTTPTarget(context.Background(), "target/manual", target); err != nil {
		t.Fatalf("PutHTTPTarget() error = %v", err)
	}

	resolved, err := store.ResolveHTTPTarget(context.Background(), "target/manual")
	if err != nil {
		t.Fatalf("ResolveHTTPTarget() error = %v", err)
	}
	if resolved.BaseURL.String() != target.BaseURL.String() {
		t.Fatalf("BaseURL = %q", resolved.BaseURL)
	}
	if got := resolved.Headers.Get("Authorization"); got != "Bearer keychain-secret-sentinel" {
		t.Fatalf("Authorization = %q", got)
	}

	resolved.Headers.Set("Authorization", "changed")
	again, err := store.ResolveHTTPTarget(context.Background(), "target/manual")
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Headers.Get("Authorization"); got != "Bearer keychain-secret-sentinel" {
		t.Fatalf("stored target was mutated: %q", got)
	}

	if err := store.DeleteTarget(context.Background(), "target/manual"); err != nil {
		t.Fatalf("DeleteTarget() error = %v", err)
	}
	if _, err := store.ResolveHTTPTarget(context.Background(), "target/manual"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ResolveHTTPTarget() after delete error = %v", err)
	}
}

func TestKeychainStoreProtectsProxyConfig(t *testing.T) {
	backend := newMemoryKeychainBackend()
	store := newKeychainStore(backend)
	proxyURL, err := url.Parse("socks5://proxy-user:proxy-secret-sentinel@127.0.0.1:7890")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutProxyConfig(context.Background(), "egress/default", ProxyConfig{URL: proxyURL}); err != nil {
		t.Fatalf("PutProxyConfig() error = %v", err)
	}
	resolved, err := store.ResolveProxyConfig(context.Background(), "egress/default")
	if err != nil {
		t.Fatalf("ResolveProxyConfig() error = %v", err)
	}
	if resolved.URL.String() != proxyURL.String() {
		t.Fatalf("proxy URL = %q", resolved.URL)
	}
	resolved.URL.Host = "changed.invalid"
	again, err := store.ResolveProxyConfig(context.Background(), "egress/default")
	if err != nil || again.URL.Host != "127.0.0.1:7890" {
		t.Fatalf("stored proxy was mutated: %v, %v", again.URL, err)
	}
	if err := store.DeleteTarget(context.Background(), "egress/default"); err != nil {
		t.Fatalf("DeleteTarget() error = %v", err)
	}
	if _, err := store.ResolveProxyConfig(context.Background(), "egress/default"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ResolveProxyConfig() after delete error = %v", err)
	}
}

func TestKeychainStoreRejectsInvalidReferences(t *testing.T) {
	store := newKeychainStore(newMemoryKeychainBackend())
	for _, reference := range []string{"", "../target", "Target/UPPER", "target with spaces"} {
		if _, err := store.ResolveHTTPTarget(context.Background(), reference); !errors.Is(err, ErrInvalidReference) {
			t.Errorf("ResolveHTTPTarget(%q) error = %v", reference, err)
		}
	}
}

func TestSSHSecretsRoundTripWithoutAliasing(t *testing.T) {
	backend := newMemoryKeychainBackend()
	store := newKeychainStore(backend)
	hostKey := testSSHPublicKey(t)
	target := SSHTarget{
		Address:         "ssh.internal.invalid:2222",
		Username:        "protected-user",
		Password:        []byte("upstream-password-sentinel"),
		ExpectedHostKey: hostKey.Marshal(),
	}
	if err := store.PutSSHTarget(t.Context(), "ssh/build", target); err != nil {
		t.Fatal(err)
	}

	resolved, err := store.ResolveSSHTarget(t.Context(), "ssh/build")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Address != target.Address || resolved.Username != target.Username || string(resolved.Password) != string(target.Password) {
		t.Fatal("protected SSH target changed during round trip")
	}
	resolved.Password[0] = 'X'
	resolved.ExpectedHostKey[0] ^= 0xff
	again, err := store.ResolveSSHTarget(t.Context(), "ssh/build")
	if err != nil {
		t.Fatal(err)
	}
	if string(again.Password) != "upstream-password-sentinel" {
		t.Fatal("stored SSH password was mutated")
	}
	if string(again.ExpectedHostKey) != string(hostKey.Marshal()) {
		t.Fatal("stored SSH host key was mutated")
	}
}

func TestMemoryStoreRejectsAmbiguousSSHAuthentication(t *testing.T) {
	store := NewMemoryStore()
	base := SSHTarget{
		Address:         "127.0.0.1:22",
		Username:        "build",
		ExpectedHostKey: testSSHPublicKey(t).Marshal(),
	}
	for _, target := range []SSHTarget{
		base,
		func() SSHTarget {
			target := base.Clone()
			target.Password = []byte("password")
			target.PrivateKey = []byte("key")
			return target
		}(),
		func() SSHTarget {
			target := base.Clone()
			target.PrivateKeyPassword = []byte("orphaned")
			return target
		}(),
	} {
		if err := store.PutSSHTarget(t.Context(), "ssh/build", target); err == nil {
			t.Fatal("invalid SSH authentication was accepted")
		}
	}
}

func TestKeychainStorePreservesEncryptedSSHPrivateKey(t *testing.T) {
	store := newKeychainStore(newMemoryKeychainBackend())
	privateKey := ed25519.NewKeyFromSeed(bytesOf(7, ed25519.SeedSize))
	block, err := ssh.MarshalPrivateKeyWithPassphrase(privateKey, "airlock test", []byte("key-passphrase-sentinel"))
	if err != nil {
		t.Fatal(err)
	}
	target := SSHTarget{
		Address:            "ssh.internal.invalid:22",
		Username:           "private-key-user",
		PrivateKey:         pem.EncodeToMemory(block),
		PrivateKeyPassword: []byte("key-passphrase-sentinel"),
		ExpectedHostKey:    testSSHPublicKey(t).Marshal(),
	}
	if err := store.PutSSHTarget(t.Context(), "ssh/private", target); err != nil {
		t.Fatal(err)
	}
	resolved, err := store.ResolveSSHTarget(t.Context(), "ssh/private")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(resolved.PrivateKey, target.PrivateKey) || !bytes.Equal(resolved.PrivateKeyPassword, target.PrivateKeyPassword) {
		t.Fatal("encrypted SSH private key changed during round trip")
	}
}

func TestKeychainStorePreservesSSHHostIdentity(t *testing.T) {
	store := newKeychainStore(newMemoryKeychainBackend())
	privateKey := ed25519.NewKeyFromSeed(bytesOf(9, ed25519.SeedSize))
	block, err := ssh.MarshalPrivateKey(privateKey, "airlock host")
	if err != nil {
		t.Fatal(err)
	}
	identity := SSHHostIdentity{PrivateKey: pem.EncodeToMemory(block)}
	if err := store.PutSSHHostIdentity(t.Context(), "ssh/host-identity", identity); err != nil {
		t.Fatal(err)
	}
	resolved, err := store.ResolveSSHHostIdentity(t.Context(), "ssh/host-identity")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(resolved.PrivateKey, identity.PrivateKey) {
		t.Fatal("SSH host identity changed during Keychain round trip")
	}
	resolved.PrivateKey[0] ^= 0xff
	again, err := store.ResolveSSHHostIdentity(t.Context(), "ssh/host-identity")
	if err != nil || !bytes.Equal(again.PrivateKey, identity.PrivateKey) {
		t.Fatal("stored SSH host identity was mutated")
	}
}

func bytesOf(value byte, length int) []byte {
	result := make([]byte, length)
	for index := range result {
		result[index] = value
	}
	return result
}

func testSSHPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	privateKey := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	publicKey, err := ssh.NewPublicKey(privateKey.Public())
	if err != nil {
		t.Fatal(err)
	}
	return publicKey
}
