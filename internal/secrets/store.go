package secrets

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync"
)

var ErrNotFound = errors.New("secret not found")

type HTTPTarget struct {
	BaseURL *url.URL
	Headers http.Header
}

func (t HTTPTarget) Clone() HTTPTarget {
	cloned := HTTPTarget{Headers: t.Headers.Clone()}
	if t.BaseURL != nil {
		urlCopy := *t.BaseURL
		cloned.BaseURL = &urlCopy
	}
	return cloned
}

type Store interface {
	ResolveHTTPTarget(ctx context.Context, reference string) (HTTPTarget, error)
}

// MemoryStore is intended for tests and the protocol spike only. Production
// targets will be supplied by an OS-backed SecretStore implementation.
type MemoryStore struct {
	mu      sync.RWMutex
	targets map[string]HTTPTarget
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{targets: make(map[string]HTTPTarget)}
}

func (s *MemoryStore) PutHTTPTarget(reference string, target HTTPTarget) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.targets[reference] = target.Clone()
}

func (s *MemoryStore) ResolveHTTPTarget(_ context.Context, reference string) (HTTPTarget, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	target, ok := s.targets[reference]
	if !ok {
		return HTTPTarget{}, ErrNotFound
	}
	return target.Clone(), nil
}
