package sshgw

import (
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/LouisonH/airlock-relay/internal/capability"
)

var (
	ErrRouteNotFound = errors.New("SSH route not found")
	ErrInvalidRoute  = errors.New("invalid SSH route")
	sshAliasPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
)

type Policy struct {
	AllowedCommands            map[string]struct{}
	LocalPublicKeyFingerprints map[string]struct{}
	AllowStdin                 bool
}

func NewPolicy(commands, fingerprints []string, allowStdin bool) Policy {
	policy := Policy{
		AllowedCommands:            make(map[string]struct{}, len(commands)),
		LocalPublicKeyFingerprints: make(map[string]struct{}, len(fingerprints)),
		AllowStdin:                 allowStdin,
	}
	for _, command := range commands {
		policy.AllowedCommands[command] = struct{}{}
	}
	for _, fingerprint := range fingerprints {
		policy.LocalPublicKeyFingerprints[fingerprint] = struct{}{}
	}
	return policy
}

func (p Policy) AllowsCommand(command string) bool {
	_, ok := p.AllowedCommands[command]
	return ok
}

type Route struct {
	Name             string
	Alias            string
	TargetSecretRef  string
	CapabilityDigest capability.Digest
	Policy           Policy
	Egress           string
	Enabled          bool
}

func (r Route) Validate() error {
	if !sshAliasPattern.MatchString(r.Alias) {
		return fmt.Errorf("%w: invalid alias", ErrInvalidRoute)
	}
	if r.TargetSecretRef != "ssh/"+r.Alias {
		return fmt.Errorf("%w: invalid secret reference", ErrInvalidRoute)
	}
	if r.CapabilityDigest == (capability.Digest{}) {
		return fmt.Errorf("%w: capability is required", ErrInvalidRoute)
	}
	if len(r.Policy.AllowedCommands) == 0 {
		return fmt.Errorf("%w: at least one exact command is required", ErrInvalidRoute)
	}
	for command := range r.Policy.AllowedCommands {
		if command == "" || len(command) > 4096 || strings.ContainsAny(command, "\x00\r\n") {
			return fmt.Errorf("%w: invalid allowed command", ErrInvalidRoute)
		}
	}
	for fingerprint := range r.Policy.LocalPublicKeyFingerprints {
		if !validSHA256Fingerprint(fingerprint) {
			return fmt.Errorf("%w: invalid local public key fingerprint", ErrInvalidRoute)
		}
	}
	switch r.Egress {
	case "", "Direct", "Proxy", "Auto":
		return nil
	default:
		return fmt.Errorf("%w: invalid egress", ErrInvalidRoute)
	}
}

func validSHA256Fingerprint(fingerprint string) bool {
	encoded, ok := strings.CutPrefix(fingerprint, "SHA256:")
	if !ok {
		return false
	}
	digest, err := base64.RawStdEncoding.DecodeString(encoded)
	return err == nil && len(digest) == 32 && base64.RawStdEncoding.EncodeToString(digest) == encoded
}

type Registry struct {
	mu     sync.RWMutex
	routes map[string]Route
}

func NewRegistry(initial ...Route) (*Registry, error) {
	registry := &Registry{routes: make(map[string]Route, len(initial))}
	for _, route := range initial {
		if err := registry.Upsert(route); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func (r *Registry) Upsert(route Route) error {
	if err := route.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[route.Alias] = cloneRoute(route)
	return nil
}

func (r *Registry) Lookup(alias string) (Route, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	route, ok := r.routes[alias]
	if !ok || !route.Enabled {
		return Route{}, ErrRouteNotFound
	}
	return cloneRoute(route), nil
}

func (r *Registry) Get(alias string) (Route, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	route, ok := r.routes[alias]
	if !ok {
		return Route{}, ErrRouteNotFound
	}
	return cloneRoute(route), nil
}

func (r *Registry) List() []Route {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Route, 0, len(r.routes))
	for _, route := range r.routes {
		result = append(result, cloneRoute(route))
	}
	return result
}

func (r *Registry) SetEnabled(alias string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	route, ok := r.routes[alias]
	if !ok {
		return ErrRouteNotFound
	}
	route.Enabled = enabled
	r.routes[alias] = route
	return nil
}

func (r *Registry) Delete(alias string) (Route, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	route, ok := r.routes[alias]
	if !ok {
		return Route{}, ErrRouteNotFound
	}
	delete(r.routes, alias)
	return cloneRoute(route), nil
}

func (r *Registry) DisableAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for alias, route := range r.routes {
		route.Enabled = false
		r.routes[alias] = route
	}
}

func cloneRoute(route Route) Route {
	route.Policy.AllowedCommands = cloneStringSet(route.Policy.AllowedCommands)
	route.Policy.LocalPublicKeyFingerprints = cloneStringSet(route.Policy.LocalPublicKeyFingerprints)
	return route
}

func cloneStringSet(source map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(source))
	for value := range source {
		result[value] = struct{}{}
	}
	return result
}
