package secrets

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
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

type SSHTarget struct {
	Address            string
	Username           string
	Password           []byte
	PrivateKey         []byte
	PrivateKeyPassword []byte
	ExpectedHostKey    []byte
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

func (t SSHTarget) Clone() SSHTarget {
	return SSHTarget{
		Address:            t.Address,
		Username:           t.Username,
		Password:           append([]byte(nil), t.Password...),
		PrivateKey:         append([]byte(nil), t.PrivateKey...),
		PrivateKeyPassword: append([]byte(nil), t.PrivateKeyPassword...),
		ExpectedHostKey:    append([]byte(nil), t.ExpectedHostKey...),
	}
}

type Store interface {
	ResolveHTTPTarget(ctx context.Context, reference string) (HTTPTarget, error)
	ResolveProxyConfig(ctx context.Context, reference string) (ProxyConfig, error)
	ResolveSSHTarget(ctx context.Context, reference string) (SSHTarget, error)
}

type MutableStore interface {
	Store
	PutHTTPTarget(ctx context.Context, reference string, target HTTPTarget) error
	PutProxyConfig(ctx context.Context, reference string, config ProxyConfig) error
	PutSSHTarget(ctx context.Context, reference string, target SSHTarget) error
	DeleteTarget(ctx context.Context, reference string) error
}

// MemoryStore is intended for tests and the protocol spike only. Production
// targets will be supplied by an OS-backed SecretStore implementation.
type MemoryStore struct {
	mu      sync.RWMutex
	targets map[string]HTTPTarget
	proxies map[string]ProxyConfig
	ssh     map[string]SSHTarget
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		targets: make(map[string]HTTPTarget),
		proxies: make(map[string]ProxyConfig),
		ssh:     make(map[string]SSHTarget),
	}
}

func (s *MemoryStore) PutSSHTarget(_ context.Context, reference string, target SSHTarget) error {
	if err := validateReference(reference); err != nil {
		return err
	}
	if err := validateSSHTarget(target); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ssh[reference] = target.Clone()
	return nil
}

func (s *MemoryStore) ResolveSSHTarget(_ context.Context, reference string) (SSHTarget, error) {
	if err := validateReference(reference); err != nil {
		return SSHTarget{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	target, ok := s.ssh[reference]
	if !ok {
		return SSHTarget{}, ErrNotFound
	}
	return target.Clone(), nil
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
	_, sshExists := s.ssh[reference]
	if !targetExists && !proxyExists && !sshExists {
		return ErrNotFound
	}
	delete(s.targets, reference)
	delete(s.proxies, reference)
	if target, ok := s.ssh[reference]; ok {
		clearSSHTarget(&target)
		delete(s.ssh, reference)
	}
	return nil
}

func validateSSHTarget(target SSHTarget) error {
	host, portText, err := net.SplitHostPort(target.Address)
	if err != nil || host == "" {
		return errors.New("invalid SSH target")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 || target.Username == "" || len(target.Username) > 255 || strings.ContainsRune(target.Username, '\x00') {
		return errors.New("invalid SSH target")
	}
	hasPassword := len(target.Password) > 0
	hasPrivateKey := len(target.PrivateKey) > 0
	if hasPassword == hasPrivateKey || (!hasPrivateKey && len(target.PrivateKeyPassword) > 0) {
		return errors.New("invalid SSH authentication")
	}
	if len(target.ExpectedHostKey) == 0 {
		return errors.New("invalid SSH host key")
	}
	if _, err := ssh.ParsePublicKey(target.ExpectedHostKey); err != nil {
		return errors.New("invalid SSH host key")
	}
	return nil
}

func clearSSHTarget(target *SSHTarget) {
	clear(target.Password)
	clear(target.PrivateKey)
	clear(target.PrivateKeyPassword)
	clear(target.ExpectedHostKey)
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
