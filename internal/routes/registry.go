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

type HTTPPolicy struct {
	AllowedMethods   map[string]struct{}
	AllowedQueryKeys map[string]struct{}
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

func (p HTTPPolicy) AllowsMethod(method string) bool {
	_, ok := p.AllowedMethods[strings.ToUpper(method)]
	return ok
}

func (p HTTPPolicy) AllowsQueryKey(key string) bool {
	_, ok := p.AllowedQueryKeys[key]
	return ok
}

type HTTPRoute struct {
	Alias            string
	TargetSecretRef  string
	CapabilityDigest capability.Digest
	Policy           HTTPPolicy
	Enabled          bool
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
	return nil
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

func cloneRoute(route HTTPRoute) HTTPRoute {
	route.Policy.AllowedMethods = cloneSet(route.Policy.AllowedMethods)
	route.Policy.AllowedQueryKeys = cloneSet(route.Policy.AllowedQueryKeys)
	return route
}

func cloneSet(source map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(source))
	for key := range source {
		result[key] = struct{}{}
	}
	return result
}
