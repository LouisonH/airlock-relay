package secrets

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync"
)

var ErrNotFound = errors.New("secret not found")
var ErrInvalidReference = errors.New("invalid secret reference")
var ErrUnsupported = errors.New("platform secret store is not supported")

type HTTPTarget struct {
	BaseURL *url.URL
	Headers http.Header
}

type ProxyConfig struct {
	URL *url.URL
}

func (c ProxyConfig) Clone() ProxyConfig {
	cloned := ProxyConfig{}
	if c.URL != nil {
		urlCopy := *c.URL
		cloned.URL = &urlCopy
	}
	return cloned
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
	ResolveProxyConfig(ctx context.Context, reference string) (ProxyConfig, error)
}

type MutableStore interface {
	Store
	PutHTTPTarget(ctx context.Context, reference string, target HTTPTarget) error
	PutProxyConfig(ctx context.Context, reference string, config ProxyConfig) error
	DeleteTarget(ctx context.Context, reference string) error
}

// MemoryStore is intended for tests and the protocol spike only. Production
// targets will be supplied by an OS-backed SecretStore implementation.
type MemoryStore struct {
	mu      sync.RWMutex
	targets map[string]HTTPTarget
	proxies map[string]ProxyConfig
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{targets: make(map[string]HTTPTarget), proxies: make(map[string]ProxyConfig)}
}

func (s *MemoryStore) PutProxyConfig(_ context.Context, reference string, config ProxyConfig) error {
	if err := validateReference(reference); err != nil {
		return err
	}
	if err := validateProxyConfig(config); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.proxies[reference] = config.Clone()
	return nil
}

func (s *MemoryStore) ResolveProxyConfig(_ context.Context, reference string) (ProxyConfig, error) {
	if err := validateReference(reference); err != nil {
		return ProxyConfig{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	config, ok := s.proxies[reference]
	if !ok {
		return ProxyConfig{}, ErrNotFound
	}
	return config.Clone(), nil
}

func (s *MemoryStore) PutHTTPTarget(_ context.Context, reference string, target HTTPTarget) error {
	if err := validateReference(reference); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.targets[reference] = target.Clone()
	return nil
}

func (s *MemoryStore) ResolveHTTPTarget(_ context.Context, reference string) (HTTPTarget, error) {
	if err := validateReference(reference); err != nil {
		return HTTPTarget{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	target, ok := s.targets[reference]
	if !ok {
		return HTTPTarget{}, ErrNotFound
	}
	return target.Clone(), nil
}

func (s *MemoryStore) DeleteTarget(_ context.Context, reference string) error {
	if err := validateReference(reference); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, targetExists := s.targets[reference]
	_, proxyExists := s.proxies[reference]
	if !targetExists && !proxyExists {
		return ErrNotFound
	}
	delete(s.targets, reference)
	delete(s.proxies, reference)
	return nil
}

func validateProxyConfig(config ProxyConfig) error {
	if config.URL == nil || config.URL.Host == "" || config.URL.RawQuery != "" || config.URL.Fragment != "" || (config.URL.Path != "" && config.URL.Path != "/") {
		return errors.New("invalid proxy config")
	}
	switch config.URL.Scheme {
	case "http", "https", "socks5", "socks5h":
		return nil
	default:
		return errors.New("invalid proxy config")
	}
}
