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
	ErrRouteNotFound      = errors.New("SSH route not found")
	ErrInvalidRoute       = errors.New("invalid SSH route")
	ErrLocalUsernameInUse = errors.New("SSH local username is already mapped")
	sshAliasPattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	localUsernamePattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
)

const (
	DefaultAuthenticationTimeoutSeconds = 20
	MinAuthenticationTimeoutSeconds     = 3
	MaxAuthenticationTimeoutSeconds     = 120
	MaxKeywordReplacements              = 64
	MaxKeywordFromBytes                 = 256
	MaxKeywordToBytes                   = 1024
)

type KeywordReplacement struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Enabled bool   `json:"enabled"`
}

func ValidateKeywordReplacements(replacements []KeywordReplacement) error {
	if len(replacements) > MaxKeywordReplacements {
		return fmt.Errorf("%w: too many keyword replacements", ErrInvalidRoute)
	}
	for _, replacement := range replacements {
		if replacement.From == "" || len(replacement.From) > MaxKeywordFromBytes || len(replacement.To) > MaxKeywordToBytes || strings.ContainsAny(replacement.From+replacement.To, "\x00\r\n") {
			return fmt.Errorf("%w: invalid keyword replacement", ErrInvalidRoute)
		}
	}
	return nil
}

func ApplyKeywordReplacements(command string, replacements []KeywordReplacement) string {
	for _, replacement := range replacements {
		if replacement.Enabled && replacement.From != "" {
			command = strings.ReplaceAll(command, replacement.From, replacement.To)
		}
	}
	return command
}

// NormalizeAuthenticationTimeoutSeconds supplies the secure default for
// metadata written by older Airlock versions and bounds new user input.
func NormalizeAuthenticationTimeoutSeconds(seconds int) (int, error) {
	if seconds == 0 {
		return DefaultAuthenticationTimeoutSeconds, nil
	}
	if seconds < MinAuthenticationTimeoutSeconds || seconds > MaxAuthenticationTimeoutSeconds {
		return 0, fmt.Errorf("%w: authentication timeout must be %d to %d seconds", ErrInvalidRoute, MinAuthenticationTimeoutSeconds, MaxAuthenticationTimeoutSeconds)
	}
	return seconds, nil
}

type Policy struct {
	AllowedCommands            map[string]struct{}
	LocalPublicKeyFingerprints map[string]struct{}
	AllowStdin                 bool
	AllowAllCommands           bool
	RecordCommands             bool
	AllowSFTP                  bool
	AllowInteractiveShell      bool
}

func NewPolicy(commands, fingerprints []string, allowStdin bool) Policy {
	return NewPolicyWithOptions(commands, fingerprints, allowStdin, false, false)
}

func NewPolicyWithOptions(commands, fingerprints []string, allowStdin, allowAllCommands, recordCommands bool) Policy {
	policy := Policy{
		AllowedCommands:            make(map[string]struct{}, len(commands)),
		LocalPublicKeyFingerprints: make(map[string]struct{}, len(fingerprints)),
		AllowStdin:                 allowStdin,
		AllowAllCommands:           allowAllCommands,
		RecordCommands:             recordCommands,
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
	if !validCommand(command) {
		return false
	}
	if p.AllowAllCommands {
		return true
	}
	_, ok := p.AllowedCommands[command]
	return ok
}

func validCommand(command string) bool {
	return command != "" && len(command) <= 4096 && !strings.ContainsAny(command, "\x00\r\n")
}

type Route struct {
	Name                         string
	Alias                        string
	LocalUsername                string
	TargetSecretRef              string
	CapabilityDigest             capability.Digest
	Policy                       Policy
	Egress                       string
	AuthenticationTimeoutSeconds int
	KeywordReplacements          []KeywordReplacement
	Enabled                      bool
}

func (r Route) EffectiveAuthenticationTimeoutSeconds() int {
	if seconds, err := NormalizeAuthenticationTimeoutSeconds(r.AuthenticationTimeoutSeconds); err == nil {
		return seconds
	}
	return DefaultAuthenticationTimeoutSeconds
}

func (r Route) EffectiveLocalUsername() string {
	if r.LocalUsername == "" {
		return r.Alias
	}
	return r.LocalUsername
}

func (r Route) Validate() error {
	if !sshAliasPattern.MatchString(r.Alias) {
		return fmt.Errorf("%w: invalid alias", ErrInvalidRoute)
	}
	if !localUsernamePattern.MatchString(r.EffectiveLocalUsername()) {
		return fmt.Errorf("%w: invalid local username", ErrInvalidRoute)
	}
	if r.TargetSecretRef != "ssh/"+r.Alias {
		return fmt.Errorf("%w: invalid secret reference", ErrInvalidRoute)
	}
	if r.CapabilityDigest == (capability.Digest{}) {
		return fmt.Errorf("%w: capability is required", ErrInvalidRoute)
	}
	if _, err := NormalizeAuthenticationTimeoutSeconds(r.AuthenticationTimeoutSeconds); err != nil {
		return err
	}
	if err := ValidateKeywordReplacements(r.KeywordReplacements); err != nil {
		return err
	}
	if !r.Policy.AllowAllCommands && len(r.Policy.AllowedCommands) == 0 {
		return fmt.Errorf("%w: at least one exact command is required", ErrInvalidRoute)
	}
	for command := range r.Policy.AllowedCommands {
		if !validCommand(command) {
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
	for alias, existing := range r.routes {
		if alias != route.Alias && existing.EffectiveLocalUsername() == route.EffectiveLocalUsername() {
			return ErrLocalUsernameInUse
		}
	}
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

func (r *Registry) LookupByUsername(username string) (Route, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, route := range r.routes {
		if route.Enabled && route.EffectiveLocalUsername() == username {
			return cloneRoute(route), nil
		}
	}
	return Route{}, ErrRouteNotFound
}

func (r *Registry) GetByUsername(username string) (Route, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, route := range r.routes {
		if route.EffectiveLocalUsername() == username {
			return cloneRoute(route), nil
		}
	}
	return Route{}, ErrRouteNotFound
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

func (r *Registry) SetCommandPolicy(alias string, policy Policy) error {
	return r.SetLocalUsernameAndCommandPolicy(alias, "", policy)
}

func (r *Registry) SetLocalUsernameAndCommandPolicy(alias, localUsername string, policy Policy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	route, ok := r.routes[alias]
	if !ok {
		return ErrRouteNotFound
	}
	if localUsername == "" {
		localUsername = route.EffectiveLocalUsername()
	}
	for otherAlias, existing := range r.routes {
		if otherAlias != alias && existing.EffectiveLocalUsername() == localUsername {
			return ErrLocalUsernameInUse
		}
	}
	policy.LocalPublicKeyFingerprints = cloneStringSet(route.Policy.LocalPublicKeyFingerprints)
	policy.AllowStdin = false
	route.LocalUsername = localUsername
	route.Policy = policy
	if err := route.Validate(); err != nil {
		return err
	}
	r.routes[alias] = cloneRoute(route)
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
	route.KeywordReplacements = append([]KeywordReplacement(nil), route.KeywordReplacements...)
	return route
}

func cloneStringSet(source map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(source))
	for value := range source {
		result[value] = struct{}{}
	}
	return result
}
