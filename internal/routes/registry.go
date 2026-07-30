package routes

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/LouisonH/airlock-relay/internal/capability"
)

var (
	ErrNotFound     = errors.New("route not found")
	ErrInvalidRoute = errors.New("invalid route")
	aliasPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
)

const (
	KindHTTP = "HTTP"
	KindLLM  = "LLM"

	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"

	DefaultLLMMaxRequestBytes   int64 = 2 << 20
	DefaultLLMRequestsPerMinute       = 60
	DefaultLLMMaxConcurrent           = 4
)

type HTTPPolicy struct {
	AllowedMethods    map[string]struct{}
	AllowedQueryKeys  map[string]struct{}
	AllowedPaths      map[string]struct{}
	AllowedModels     map[string]struct{}
	MaxRequestBytes   int64
	MaxOutputTokens   int
	RequestsPerMinute int
	MaxConcurrent     int
	TrackUsage        bool
}

type LLMUsage struct {
	Requests     uint64
	InputTokens  uint64
	OutputTokens uint64
}

func NewHTTPPolicy(methods, queryKeys []string) HTTPPolicy {
	policy := HTTPPolicy{
		AllowedMethods:   make(map[string]struct{}, len(methods)),
		AllowedQueryKeys: make(map[string]struct{}, len(queryKeys)),
	}
	for _, method := range methods {
		policy.AllowedMethods[strings.ToUpper(method)] = struct{}{}
	}
	for _, key := range queryKeys {
		policy.AllowedQueryKeys[key] = struct{}{}
	}
	return policy
}

func NewLLMPolicy(provider string, models []string, maxOutputTokens int) HTTPPolicy {
	paths := []string{}
	switch provider {
	case ProviderOpenAI:
		paths = []string{"/v1/responses", "/v1/chat/completions"}
	case ProviderAnthropic:
		paths = []string{"/v1/messages"}
	}
	policy := NewHTTPPolicy([]string{"POST"}, nil)
	policy.AllowedPaths = make(map[string]struct{}, len(paths))
	for _, allowedPath := range paths {
		policy.AllowedPaths[allowedPath] = struct{}{}
	}
	policy.AllowedModels = make(map[string]struct{}, len(models))
	for _, model := range models {
		policy.AllowedModels[model] = struct{}{}
	}
	policy.MaxRequestBytes = DefaultLLMMaxRequestBytes
	policy.MaxOutputTokens = maxOutputTokens
	policy.RequestsPerMinute = DefaultLLMRequestsPerMinute
	policy.MaxConcurrent = DefaultLLMMaxConcurrent
	return policy
}

func (p HTTPPolicy) AllowsMethod(method string) bool {
	_, ok := p.AllowedMethods[strings.ToUpper(method)]
	return ok
}

func (p HTTPPolicy) AllowsQueryKey(key string) bool {
	_, ok := p.AllowedQueryKeys[key]
	return ok
}

func (p HTTPPolicy) AllowsPath(value string) bool {
	_, ok := p.AllowedPaths[value]
	return ok
}

func (p HTTPPolicy) AllowsModel(model string) bool {
	_, ok := p.AllowedModels[model]
	return ok
}

type HTTPRoute struct {
	Name             string
	Alias            string
	Kind             string
	Provider         string
	TargetSecretRef  string
	CapabilityDigest capability.Digest
	Policy           HTTPPolicy
	Egress           string
	Enabled          bool
	Usage            LLMUsage
}

func (r HTTPRoute) EffectiveKind() string {
	if r.Kind == "" {
		return KindHTTP
	}
	return r.Kind
}

func (r HTTPRoute) Validate() error {
	if !aliasPattern.MatchString(r.Alias) {
		return fmt.Errorf("%w: invalid alias", ErrInvalidRoute)
	}
	if r.TargetSecretRef == "" {
		return fmt.Errorf("%w: target secret reference is required", ErrInvalidRoute)
	}
	if len(r.Policy.AllowedMethods) == 0 {
		return fmt.Errorf("%w: at least one HTTP method is required", ErrInvalidRoute)
	}
	if r.Egress != "" && r.Egress != "Direct" && r.Egress != "Proxy" && r.Egress != "Auto" {
		return fmt.Errorf("%w: invalid egress", ErrInvalidRoute)
	}
	switch r.EffectiveKind() {
	case KindHTTP:
		if r.Provider != "" {
			return fmt.Errorf("%w: HTTP route cannot declare an LLM provider", ErrInvalidRoute)
		}
	case KindLLM:
		if !validLLMProvider(r.Provider) || len(r.Policy.AllowedModels) == 0 || r.Policy.MaxRequestBytes < 1 || r.Policy.MaxRequestBytes > 16<<20 || r.Policy.MaxOutputTokens < 1 || r.Policy.MaxOutputTokens > 1_000_000 || r.Policy.RequestsPerMinute < 1 || r.Policy.RequestsPerMinute > 60_000 || r.Policy.MaxConcurrent < 1 || r.Policy.MaxConcurrent > 1_024 {
			return fmt.Errorf("%w: invalid LLM policy", ErrInvalidRoute)
		}
		if len(r.Policy.AllowedMethods) != 1 || !r.Policy.AllowsMethod("POST") || len(r.Policy.AllowedQueryKeys) != 0 || len(r.Policy.AllowedPaths) == 0 {
			return fmt.Errorf("%w: invalid LLM request policy", ErrInvalidRoute)
		}
		for allowedPath := range r.Policy.AllowedPaths {
			if !providerAllowsPath(r.Provider, allowedPath) {
				return fmt.Errorf("%w: invalid LLM endpoint", ErrInvalidRoute)
			}
		}
		for model := range r.Policy.AllowedModels {
			if strings.TrimSpace(model) != model || model == "" || len(model) > 200 || strings.ContainsAny(model, "\x00\r\n\t") {
				return fmt.Errorf("%w: invalid LLM model", ErrInvalidRoute)
			}
		}
	default:
		return fmt.Errorf("%w: invalid route kind", ErrInvalidRoute)
	}
	return nil
}

func validLLMProvider(provider string) bool {
	return provider == ProviderOpenAI || provider == ProviderAnthropic
}

func providerAllowsPath(provider, value string) bool {
	switch provider {
	case ProviderOpenAI:
		return value == "/v1/responses" || value == "/v1/chat/completions"
	case ProviderAnthropic:
		return value == "/v1/messages"
	default:
		return false
	}
}

type Registry struct {
	mu     sync.RWMutex
	routes map[string]HTTPRoute
}

func NewRegistry(initial ...HTTPRoute) (*Registry, error) {
	registry := &Registry{routes: make(map[string]HTTPRoute, len(initial))}
	for _, route := range initial {
		if err := registry.Upsert(route); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func (r *Registry) Upsert(route HTTPRoute) error {
	if err := route.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[route.Alias] = cloneRoute(route)
	return nil
}

func (r *Registry) Lookup(alias string) (HTTPRoute, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	route, ok := r.routes[alias]
	if !ok || !route.Enabled {
		return HTTPRoute{}, ErrNotFound
	}
	return cloneRoute(route), nil
}

func (r *Registry) Get(alias string) (HTTPRoute, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	route, ok := r.routes[alias]
	if !ok {
		return HTTPRoute{}, ErrNotFound
	}
	return cloneRoute(route), nil
}

func (r *Registry) List() []HTTPRoute {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]HTTPRoute, 0, len(r.routes))
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
		return ErrNotFound
	}
	route.Enabled = enabled
	r.routes[alias] = route
	return nil
}

func (r *Registry) RecordLLMUsage(alias string, requests, inputTokens, outputTokens uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	route, ok := r.routes[alias]
	if !ok || route.EffectiveKind() != KindLLM || !route.Policy.TrackUsage {
		return ErrNotFound
	}
	route.Usage.Requests += requests
	route.Usage.InputTokens += inputTokens
	route.Usage.OutputTokens += outputTokens
	r.routes[alias] = route
	return nil
}

func (r *Registry) ResetLLMUsage(alias string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	route, ok := r.routes[alias]
	if !ok || route.EffectiveKind() != KindLLM {
		return ErrNotFound
	}
	route.Usage = LLMUsage{}
	r.routes[alias] = route
	return nil
}

func (r *Registry) Delete(alias string) (HTTPRoute, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	route, ok := r.routes[alias]
	if !ok {
		return HTTPRoute{}, ErrNotFound
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

func cloneRoute(route HTTPRoute) HTTPRoute {
	route.Policy.AllowedMethods = cloneSet(route.Policy.AllowedMethods)
	route.Policy.AllowedQueryKeys = cloneSet(route.Policy.AllowedQueryKeys)
	route.Policy.AllowedPaths = cloneSet(route.Policy.AllowedPaths)
	route.Policy.AllowedModels = cloneSet(route.Policy.AllowedModels)
	return route
}

func cloneSet(source map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(source))
	for key := range source {
		result[key] = struct{}{}
	}
	return result
}
