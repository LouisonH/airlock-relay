package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
)

const targetFormatVersion = 1
const proxyFormatVersion = 1
const sshTargetFormatVersion = 1
const sshIdentityFormatVersion = 1

var referencePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9/_-]{0,127}$`)

type keychainBackend interface {
	Put(reference string, data []byte) error
	Get(reference string) ([]byte, error)
	Delete(reference string) error
}

const platformStoreService = "io.airlock.relay.targets"

type KeychainStore struct {
	backend keychainBackend
}

type persistedHTTPTarget struct {
	Version int                 `json:"version"`
	BaseURL string              `json:"base_url"`
	Headers map[string][]string `json:"headers,omitempty"`
}

type persistedProxyConfig struct {
	Version int    `json:"version"`
	URL     string `json:"url"`
}

type persistedSSHTarget struct {
	Version            int    `json:"version"`
	Address            string `json:"address"`
	Username           string `json:"username"`
	Password           []byte `json:"password,omitempty"`
	PrivateKey         []byte `json:"private_key,omitempty"`
	PrivateKeyPassword []byte `json:"private_key_password,omitempty"`
	ExpectedHostKey    []byte `json:"expected_host_key"`
}

type persistedSSHHostIdentity struct {
	Version    int    `json:"version"`
	PrivateKey []byte `json:"private_key"`
}

func newKeychainStore(backend keychainBackend) *KeychainStore {
	return &KeychainStore{backend: backend}
}

func (s *KeychainStore) PutHTTPTarget(ctx context.Context, reference string, target HTTPTarget) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateReference(reference); err != nil {
		return err
	}
	if target.BaseURL == nil || (target.BaseURL.Scheme != "http" && target.BaseURL.Scheme != "https") || target.BaseURL.Host == "" {
		return errors.New("invalid HTTP target")
	}
	payload, err := json.Marshal(persistedHTTPTarget{
		Version: targetFormatVersion,
		BaseURL: target.BaseURL.String(),
		Headers: map[string][]string(target.Headers.Clone()),
	})
	if err != nil {
		return errors.New("encode protected target")
	}
	defer clear(payload)
	if err := s.backend.Put(reference, payload); err != nil {
		return errors.New("store protected target")
	}
	return nil
}

func (s *KeychainStore) ResolveHTTPTarget(ctx context.Context, reference string) (HTTPTarget, error) {
	if err := ctx.Err(); err != nil {
		return HTTPTarget{}, err
	}
	if err := validateReference(reference); err != nil {
		return HTTPTarget{}, err
	}
	payload, err := s.backend.Get(reference)
	if err != nil {
		return HTTPTarget{}, err
	}
	defer clear(payload)

	var stored persistedHTTPTarget
	if err := json.Unmarshal(payload, &stored); err != nil || stored.Version != targetFormatVersion {
		return HTTPTarget{}, errors.New("decode protected target")
	}
	baseURL, err := url.Parse(stored.BaseURL)
	if err != nil || baseURL.Host == "" || (baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		return HTTPTarget{}, errors.New("decode protected target")
	}
	return HTTPTarget{BaseURL: baseURL, Headers: http.Header(stored.Headers).Clone()}, nil
}

func (s *KeychainStore) PutProxyConfig(ctx context.Context, reference string, config ProxyConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateReference(reference); err != nil {
		return err
	}
	if err := validateProxyConfig(config); err != nil {
		return err
	}
	payload, err := json.Marshal(persistedProxyConfig{Version: proxyFormatVersion, URL: config.URL.String()})
	if err != nil {
		return errors.New("encode protected proxy")
	}
	defer clear(payload)
	if err := s.backend.Put(reference, payload); err != nil {
		return errors.New("store protected proxy")
	}
	return nil
}

func (s *KeychainStore) ResolveProxyConfig(ctx context.Context, reference string) (ProxyConfig, error) {
	if err := ctx.Err(); err != nil {
		return ProxyConfig{}, err
	}
	if err := validateReference(reference); err != nil {
		return ProxyConfig{}, err
	}
	payload, err := s.backend.Get(reference)
	if err != nil {
		return ProxyConfig{}, err
	}
	defer clear(payload)

	var stored persistedProxyConfig
	if err := json.Unmarshal(payload, &stored); err != nil || stored.Version != proxyFormatVersion {
		return ProxyConfig{}, errors.New("decode protected proxy")
	}
	proxyURL, err := url.Parse(stored.URL)
	config := ProxyConfig{URL: proxyURL}
	if err != nil || validateProxyConfig(config) != nil {
		return ProxyConfig{}, errors.New("decode protected proxy")
	}
	return config, nil
}

func (s *KeychainStore) PutSSHTarget(ctx context.Context, reference string, target SSHTarget) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateReference(reference); err != nil {
		return err
	}
	if err := validateSSHTarget(target); err != nil {
		return err
	}
	payload, err := json.Marshal(persistedSSHTarget{
		Version:            sshTargetFormatVersion,
		Address:            target.Address,
		Username:           target.Username,
		Password:           target.Password,
		PrivateKey:         target.PrivateKey,
		PrivateKeyPassword: target.PrivateKeyPassword,
		ExpectedHostKey:    target.ExpectedHostKey,
	})
	if err != nil {
		return errors.New("encode protected SSH target")
	}
	defer clear(payload)
	if err := s.backend.Put(reference, payload); err != nil {
		return errors.New("store protected SSH target")
	}
	return nil
}

func (s *KeychainStore) ResolveSSHTarget(ctx context.Context, reference string) (SSHTarget, error) {
	if err := ctx.Err(); err != nil {
		return SSHTarget{}, err
	}
	if err := validateReference(reference); err != nil {
		return SSHTarget{}, err
	}
	payload, err := s.backend.Get(reference)
	if err != nil {
		return SSHTarget{}, err
	}
	defer clear(payload)

	var stored persistedSSHTarget
	if err := json.Unmarshal(payload, &stored); err != nil || stored.Version != sshTargetFormatVersion {
		return SSHTarget{}, errors.New("decode protected SSH target")
	}
	target := SSHTarget{
		Address:            stored.Address,
		Username:           stored.Username,
		Password:           stored.Password,
		PrivateKey:         stored.PrivateKey,
		PrivateKeyPassword: stored.PrivateKeyPassword,
		ExpectedHostKey:    stored.ExpectedHostKey,
	}
	if err := validateSSHTarget(target); err != nil {
		clearSSHTarget(&target)
		return SSHTarget{}, errors.New("decode protected SSH target")
	}
	return target, nil
}

func (s *KeychainStore) PutSSHHostIdentity(ctx context.Context, reference string, identity SSHHostIdentity) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateReference(reference); err != nil {
		return err
	}
	if err := validateSSHHostIdentity(identity); err != nil {
		return err
	}
	payload, err := json.Marshal(persistedSSHHostIdentity{Version: sshIdentityFormatVersion, PrivateKey: identity.PrivateKey})
	if err != nil {
		return errors.New("encode protected SSH host identity")
	}
	defer clear(payload)
	if err := s.backend.Put(reference, payload); err != nil {
		return errors.New("store protected SSH host identity")
	}
	return nil
}

func (s *KeychainStore) ResolveSSHHostIdentity(ctx context.Context, reference string) (SSHHostIdentity, error) {
	if err := ctx.Err(); err != nil {
		return SSHHostIdentity{}, err
	}
	if err := validateReference(reference); err != nil {
		return SSHHostIdentity{}, err
	}
	payload, err := s.backend.Get(reference)
	if err != nil {
		return SSHHostIdentity{}, err
	}
	defer clear(payload)
	var stored persistedSSHHostIdentity
	if err := json.Unmarshal(payload, &stored); err != nil || stored.Version != sshIdentityFormatVersion {
		return SSHHostIdentity{}, errors.New("decode protected SSH host identity")
	}
	identity := SSHHostIdentity{PrivateKey: stored.PrivateKey}
	if err := validateSSHHostIdentity(identity); err != nil {
		clear(identity.PrivateKey)
		return SSHHostIdentity{}, errors.New("decode protected SSH host identity")
	}
	return identity, nil
}

func (s *KeychainStore) DeleteTarget(ctx context.Context, reference string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateReference(reference); err != nil {
		return err
	}
	return s.backend.Delete(reference)
}

func validateReference(reference string) error {
	if !referencePattern.MatchString(reference) {
		return ErrInvalidReference
	}
	return nil
}
