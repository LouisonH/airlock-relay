package sshgw

import (
	"errors"
	"testing"

	"github.com/LouisonH/airlock-relay/internal/capability"
)

func TestRegistryValidatesAndDefensivelyClonesRoutes(t *testing.T) {
	route := Route{
		Name:             "Build",
		Alias:            "build",
		TargetSecretRef:  "ssh/build",
		CapabilityDigest: capability.Hash("airlock-local-capability"),
		Policy:           NewPolicy([]string{"build --release"}, []string{"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}, false),
		Egress:           "Auto",
		Enabled:          true,
	}
	registry, err := NewRegistry(route)
	if err != nil {
		t.Fatal(err)
	}
	route.Policy.AllowedCommands["unexpected"] = struct{}{}

	resolved, err := registry.Lookup("build")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Policy.AllowsCommand("unexpected") {
		t.Fatal("registry retained caller-owned command map")
	}
	resolved.Policy.AllowedCommands["mutated"] = struct{}{}
	again, err := registry.Lookup("build")
	if err != nil {
		t.Fatal(err)
	}
	if again.Policy.AllowsCommand("mutated") {
		t.Fatal("lookup returned registry-owned command map")
	}

	if err := registry.SetEnabled("build", false); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Lookup("build"); !errors.Is(err, ErrRouteNotFound) {
		t.Fatalf("disabled route lookup error = %v", err)
	}
}

func TestRouteRejectsUnsafePolicy(t *testing.T) {
	base := Route{
		Alias:            "build",
		TargetSecretRef:  "ssh/build",
		CapabilityDigest: capability.Hash("airlock-local-capability"),
		Policy:           NewPolicy([]string{"build --release"}, nil, false),
		Enabled:          true,
	}
	invalid := []Route{
		func() Route { route := base; route.Alias = "../build"; return route }(),
		func() Route { route := base; route.TargetSecretRef = "routes/build"; return route }(),
		func() Route { route := base; route.CapabilityDigest = capability.Digest{}; return route }(),
		func() Route { route := base; route.Policy = NewPolicy([]string{""}, nil, false); return route }(),
		func() Route {
			route := base
			route.Policy = NewPolicy([]string{"build\nwhoami"}, nil, false)
			return route
		}(),
		func() Route {
			route := base
			route.Policy = NewPolicy([]string{"build"}, []string{"SHA256:not-a-key"}, false)
			return route
		}(),
		func() Route { route := base; route.Egress = "FallbackAlways"; return route }(),
	}
	for _, route := range invalid {
		if err := route.Validate(); !errors.Is(err, ErrInvalidRoute) {
			t.Fatalf("Validate() error = %v", err)
		}
	}
}
