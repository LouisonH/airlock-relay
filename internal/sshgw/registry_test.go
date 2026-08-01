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

func TestRegistryMapsUniqueLocalUsernamesAndPreservesLegacyAliases(t *testing.T) {
	legacy := Route{
		Name: "Legacy", Alias: "legacy", TargetSecretRef: "ssh/legacy",
		CapabilityDigest: capability.Hash("shared-local-password"),
		Policy:           NewPolicy([]string{"uptime"}, nil, false),
		Enabled:          true,
	}
	mapped := Route{
		Name: "Builder", Alias: "build", LocalUsername: "builder",
		TargetSecretRef: "ssh/build", CapabilityDigest: capability.Hash("shared-local-password"),
		Policy: NewPolicy([]string{"uptime"}, nil, false), Enabled: true,
	}
	registry, err := NewRegistry(legacy, mapped)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.EffectiveLocalUsername() != "legacy" {
		t.Fatalf("legacy effective username = %q", legacy.EffectiveLocalUsername())
	}
	for username, wantAlias := range map[string]string{"legacy": "legacy", "builder": "build"} {
		resolved, err := registry.LookupByUsername(username)
		if err != nil || resolved.Alias != wantAlias {
			t.Fatalf("LookupByUsername(%q) = %+v, %v", username, resolved, err)
		}
	}

	duplicate := mapped
	duplicate.Name = "Deploy"
	duplicate.Alias = "deploy"
	duplicate.TargetSecretRef = "ssh/deploy"
	if err := registry.Upsert(duplicate); !errors.Is(err, ErrLocalUsernameInUse) {
		t.Fatalf("duplicate local username error = %v", err)
	}
	if err := registry.SetLocalUsernameAndCommandPolicy("legacy", "builder", legacy.Policy); !errors.Is(err, ErrLocalUsernameInUse) {
		t.Fatalf("duplicate username update error = %v", err)
	}
	if route, err := registry.Get("legacy"); err != nil || route.EffectiveLocalUsername() != "legacy" {
		t.Fatalf("failed update changed legacy mapping: %+v, %v", route, err)
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
		func() Route { route := base; route.LocalUsername = "Invalid User"; return route }(),
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

func TestAllowAllCommandPolicyStillRejectsMalformedExec(t *testing.T) {
	policy := NewPolicyWithOptions(nil, nil, false, true, true)
	if !policy.AllowsCommand("uname -a") {
		t.Fatal("allow-all policy rejected a valid exec command")
	}
	for _, command := range []string{"", "whoami\ncat /etc/passwd", "printf ok\x00whoami"} {
		if policy.AllowsCommand(command) {
			t.Fatalf("allow-all policy accepted malformed command %q", command)
		}
	}
	route := Route{
		Alias: "nas", TargetSecretRef: "ssh/nas",
		CapabilityDigest: capability.Hash("airlock-local-capability"),
		Policy:           policy, Enabled: true,
	}
	if err := route.Validate(); err != nil {
		t.Fatalf("allow-all route validation failed: %v", err)
	}
}

func TestKeywordReplacementsAreOrderedAndValidated(t *testing.T) {
	rules := []KeywordReplacement{
		{From: "input.user", To: "service", Enabled: true},
		{From: "service.passwd", To: "protected-value", Enabled: true},
		{From: "unused", To: "ignored", Enabled: false},
	}
	if err := ValidateKeywordReplacements(rules); err != nil {
		t.Fatalf("ValidateKeywordReplacements() error = %v", err)
	}
	if got := ApplyKeywordReplacements("login input.user.passwd unused", rules); got != "login protected-value unused" {
		t.Fatalf("ApplyKeywordReplacements() = %q", got)
	}
	for _, rules := range [][]KeywordReplacement{
		{{From: "", To: "value", Enabled: true}},
		{{From: "line\nbreak", To: "value", Enabled: true}},
		{{From: "key", To: "value\x00", Enabled: true}},
	} {
		if err := ValidateKeywordReplacements(rules); !errors.Is(err, ErrInvalidRoute) {
			t.Fatalf("ValidateKeywordReplacements(%+v) error = %v", rules, err)
		}
	}
}
