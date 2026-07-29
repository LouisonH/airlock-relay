package secrets

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"
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

func TestKeychainStoreRejectsInvalidReferences(t *testing.T) {
	store := newKeychainStore(newMemoryKeychainBackend())
	for _, reference := range []string{"", "../target", "Target/UPPER", "target with spaces"} {
		if _, err := store.ResolveHTTPTarget(context.Background(), reference); !errors.Is(err, ErrInvalidReference) {
			t.Errorf("ResolveHTTPTarget(%q) error = %v", reference, err)
		}
	}
}
