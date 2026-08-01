package control

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/LouisonH/airlock-relay/internal/activity"
	"github.com/LouisonH/airlock-relay/internal/capability"
	"github.com/LouisonH/airlock-relay/internal/egress"
	"github.com/LouisonH/airlock-relay/internal/routes"
	"github.com/LouisonH/airlock-relay/internal/secrets"
	"github.com/LouisonH/airlock-relay/internal/sshgw"
	"golang.org/x/crypto/ssh"
)

const (
	protocolVersion           = 1
	maxMessageBytes           = 64 << 10
	controlExchangeTimeout    = 20 * time.Second
	controlHealthCheckTimeout = 125 * time.Second
	controlRequestReadWindow  = 10 * time.Second
)

type Paths struct {
	Directory string
	Socket    string
}

// ValidatePaths rejects control endpoint descriptions that do not belong to
// the current platform's owner-only transport.
func ValidatePaths(paths Paths) error {
	if !validControlPaths(paths) {
		return errors.New("invalid control paths")
	}
	return nil
}

type Server struct {
	registry     *routes.Registry
	secrets      secrets.MutableStore
	persistence  routes.MetadataStore
	egress       *egress.Manager
	ssh          *SSHConfiguration
	httpAddress  string
	token        string
	secretMode   string
	storeFactory SecretStoreFactory
	mutationMu   sync.Mutex
	activity     activity.Recorder
	healthMu     sync.RWMutex
	health       map[string]routeHealthState
}

type routeHealthState struct {
	Status    string
	CheckedAt time.Time
}

type SecretStoreFactory func(mode string) (secrets.MutableStore, error)

type Request struct {
	Version             int                        `json:"version"`
	Token               string                     `json:"token"`
	Action              string                     `json:"action"`
	Create              *CreateHTTPRoute           `json:"create,omitempty"`
	CreateSSH           *CreateSSHRoute            `json:"create_ssh,omitempty"`
	ProbeSSH            *ProbeSSHHostKey           `json:"probe_ssh,omitempty"`
	SSHPolicy           *SSHPolicyUpdate           `json:"ssh_policy,omitempty"`
	Alias               string                     `json:"alias,omitempty"`
	Enabled             bool                       `json:"enabled,omitempty"`
	ProxyURL            string                     `json:"proxy_url,omitempty"`
	Capability          string                     `json:"capability,omitempty"`
	Command             string                     `json:"command,omitempty"`
	SecretMode          string                     `json:"secret_store_mode,omitempty"`
	KeywordReplacements []sshgw.KeywordReplacement `json:"keyword_replacements,omitempty"`
}

type CreateHTTPRoute struct {
	Name              string   `json:"name"`
	Alias             string   `json:"alias"`
	BaseURL           string   `json:"base_url"`
	Authorization     string   `json:"authorization,omitempty"`
	Egress            string   `json:"egress,omitempty"`
	Provider          string   `json:"provider,omitempty"`
	Models            []string `json:"models,omitempty"`
	MaxOutputTokens   int      `json:"max_output_tokens,omitempty"`
	RequestsPerMinute int      `json:"requests_per_minute,omitempty"`
	MaxConcurrent     int      `json:"max_concurrent,omitempty"`
	TrackUsage        bool     `json:"track_usage,omitempty"`
	LocalAPIKey       string   `json:"local_api_key,omitempty"`
}

type CreateSSHRoute struct {
	Name                         string                     `json:"name"`
	Alias                        string                     `json:"alias"`
	LocalUsername                string                     `json:"local_username,omitempty"`
	Address                      string                     `json:"address"`
	Username                     string                     `json:"username"`
	Password                     string                     `json:"password"`
	LocalPassword                string                     `json:"local_password,omitempty"`
	ExpectedHostKey              string                     `json:"expected_host_key"`
	AllowedCommand               string                     `json:"allowed_command"`
	AllowAllCommands             bool                       `json:"allow_all_commands"`
	RecordCommands               bool                       `json:"record_commands"`
	AllowSFTP                    bool                       `json:"allow_sftp"`
	AuthenticationTimeoutSeconds int                        `json:"authentication_timeout_seconds,omitempty"`
	Egress                       string                     `json:"egress,omitempty"`
	KeywordReplacements          []sshgw.KeywordReplacement `json:"keyword_replacements,omitempty"`
}

type SSHPolicyUpdate struct {
	Name                         string `json:"name,omitempty"`
	LocalUsername                string `json:"local_username,omitempty"`
	AllowedCommand               string `json:"allowed_command"`
	AllowAllCommands             bool   `json:"allow_all_commands"`
	RecordCommands               bool   `json:"record_commands"`
	AllowSFTP                    bool   `json:"allow_sftp"`
	AuthenticationTimeoutSeconds int    `json:"authentication_timeout_seconds,omitempty"`
	Egress                       string `json:"egress,omitempty"`
}

type ProbeSSHHostKey struct {
	Address string `json:"address"`
	Egress  string `json:"egress,omitempty"`
}

type SSHConfiguration struct {
	Registry      *sshgw.Registry
	Persistence   sshgw.MetadataStore
	ListenAddress string
	DialAddress   string
	HTTPAddress   string
	NetworkScope  string
	HostKey       ssh.PublicKey
	CommandAudit  sshgw.CommandAudit
	Activity      activity.Recorder
}

type Response struct {
	OK                  bool                       `json:"ok"`
	Error               string                     `json:"error,omitempty"`
	Warning             string                     `json:"warning,omitempty"`
	Running             bool                       `json:"running"`
	Routes              []RouteSummary             `json:"routes,omitempty"`
	Created             *CreatedRoute              `json:"created,omitempty"`
	ProxyConfigured     bool                       `json:"proxy_configured"`
	SSHReady            bool                       `json:"ssh_ready"`
	SSHHostKeyProbe     *SSHHostKeyProbe           `json:"ssh_host_key_probe,omitempty"`
	HealthCheck         *HealthCheckSummary        `json:"health_check,omitempty"`
	Activity            []ActivitySummary          `json:"activity,omitempty"`
	KeywordReplacements []sshgw.KeywordReplacement `json:"keyword_replacements,omitempty"`
}

type SSHHostKeyProbe struct {
	HostKey     string `json:"host_key"`
	Fingerprint string `json:"fingerprint"`
}

type HealthCheckSummary struct {
	Alias     string `json:"alias"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Latency   string `json:"latency"`
	CheckedAt string `json:"checkedAt"`
}

type CreatedRoute struct {
	Route      RouteSummary `json:"route"`
	Capability string       `json:"capability"`
}

type RouteSummary struct {
	ID                           string   `json:"id"`
	Name                         string   `json:"name"`
	Alias                        string   `json:"alias"`
	LocalUsername                string   `json:"localUsername,omitempty"`
	Kind                         string   `json:"kind"`
	Status                       string   `json:"status"`
	LocalEndpoint                string   `json:"localEndpoint"`
	PermissionSummary            string   `json:"permissionSummary"`
	Egress                       string   `json:"egress"`
	Health                       string   `json:"health"`
	LastUsed                     string   `json:"lastUsed"`
	CurrentConnections           int      `json:"currentConnections"`
	AllowAllCommands             bool     `json:"allowAllCommands"`
	RecordCommands               bool     `json:"recordCommands"`
	AllowSFTP                    bool     `json:"allowSftp"`
	AllowedCommand               string   `json:"allowedCommand,omitempty"`
	Provider                     string   `json:"provider,omitempty"`
	AllowedModels                []string `json:"allowedModels,omitempty"`
	MaxOutputTokens              int      `json:"maxOutputTokens,omitempty"`
	RequestsPerMinute            int      `json:"requestsPerMinute,omitempty"`
	MaxConcurrent                int      `json:"maxConcurrent,omitempty"`
	TrackUsage                   bool     `json:"trackUsage,omitempty"`
	TotalRequests                uint64   `json:"totalRequests,omitempty"`
	InputTokens                  uint64   `json:"inputTokens,omitempty"`
	OutputTokens                 uint64   `json:"outputTokens,omitempty"`
	AuthenticationTimeoutSeconds int      `json:"authenticationTimeoutSeconds,omitempty"`
	KeywordReplacementCount      int      `json:"keywordReplacementCount,omitempty"`
}

type ActivitySummary struct {
	ID        string `json:"id"`
	Time      string `json:"time"`
	RouteName string `json:"routeName"`
	Caller    string `json:"caller"`
	Action    string `json:"action"`
	Detail    string `json:"detail,omitempty"`
	Result    string `json:"result"`
	Latency   string `json:"latency"`
	Egress    string `json:"egress"`
	Category  string `json:"category"`
	EventType string `json:"eventType"`
	when      time.Time
}

func Listen(paths Paths, token string, registry *routes.Registry, store secrets.MutableStore, persistence routes.MetadataStore, egressManager *egress.Manager) (net.Listener, *Server, error) {
	return listen(paths, token, registry, store, persistence, egressManager, nil)
}

func ListenWithSSH(paths Paths, token string, registry *routes.Registry, store secrets.MutableStore, persistence routes.MetadataStore, egressManager *egress.Manager, sshConfiguration SSHConfiguration) (net.Listener, *Server, error) {
	if sshConfiguration.Registry == nil || sshConfiguration.Persistence == nil || sshConfiguration.ListenAddress == "" || sshConfiguration.HostKey == nil {
		return nil, nil, errors.New("invalid SSH control configuration")
	}
	if sshConfiguration.DialAddress == "" {
		sshConfiguration.DialAddress = sshConfiguration.ListenAddress
	}
	return listen(paths, token, registry, store, persistence, egressManager, &sshConfiguration)
}

func listen(paths Paths, token string, registry *routes.Registry, store secrets.MutableStore, persistence routes.MetadataStore, egressManager *egress.Manager, sshConfiguration *SSHConfiguration) (net.Listener, *Server, error) {
	if len(token) < 32 || len(token) > 128 {
		return nil, nil, errors.New("invalid in-memory control token")
	}
	if persistence == nil {
		return nil, nil, errors.New("route metadata store is required")
	}
	if egressManager == nil {
		return nil, nil, errors.New("egress manager is required")
	}
	if err := ValidatePaths(paths); err != nil {
		return nil, nil, err
	}
	listener, err := listenControlEndpoint(paths)
	if err != nil {
		return nil, nil, fmt.Errorf("listen on control endpoint: %w", err)
	}
	httpAddress := "127.0.0.1:4768"
	if sshConfiguration != nil && sshConfiguration.HTTPAddress != "" {
		httpAddress = sshConfiguration.HTTPAddress
	}
	var recorder activity.Recorder
	if sshConfiguration != nil {
		recorder = sshConfiguration.Activity
	}
	return listener, &Server{registry: registry, secrets: store, persistence: persistence, egress: egressManager, ssh: sshConfiguration, httpAddress: httpAddress, token: token, activity: recorder, health: make(map[string]routeHealthState)}, nil
}

func (s *Server) Serve(listener net.Listener) error {
	for {
		connection, err := listener.Accept()
		if err != nil {
			return err
		}
		go s.handle(connection)
	}
}

func (s *Server) handle(connection net.Conn) {
	defer connection.Close()
	// Keep request reads short while leaving enough time for a bounded remote
	// authentication or health check to return a sanitized response.
	_ = connection.SetDeadline(time.Now().Add(controlRequestReadWindow))

	decoder := json.NewDecoder(io.LimitReader(connection, maxMessageBytes))
	decoder.DisallowUnknownFields()
	var request Request
	if err := decoder.Decode(&request); err != nil {
		writeResponse(connection, Response{Error: "invalid control request"})
		return
	}
	deadline := controlExchangeTimeout
	if request.Action == "test_route_health" {
		deadline = controlHealthCheckTimeout
	}
	_ = connection.SetDeadline(time.Now().Add(deadline))
	response := s.dispatch(request)
	writeResponse(connection, response)
}

func (s *Server) dispatch(request Request) Response {
	if request.Version != protocolVersion || !validToken(request.Token, s.token) {
		return Response{Error: "control authentication failed"}
	}

	switch request.Action {
	case "status", "list_routes":
		return s.successResponse()
	case "create_http_route":
		return s.createHTTPRoute(request.Create)
	case "create_llm_route":
		return s.createLLMRoute(request.Create)
	case "set_llm_policy":
		return s.setLLMPolicy(request.Alias, request.Create)
	case "rotate_llm_api_key":
		return s.rotateLLMAPIKey(request.Alias, request.Create)
	case "reset_llm_usage":
		return s.resetLLMUsage(request.Alias)
	case "probe_ssh_host_key":
		return s.probeSSHHostKey(request.ProbeSSH)
	case "create_ssh_route":
		return s.createSSHRoute(request.CreateSSH)
	case "set_ssh_policy":
		return s.setSSHPolicy(request.Alias, request.SSHPolicy)
	case "get_ssh_replacements":
		return s.getSSHKeywordReplacements(request.Alias)
	case "set_ssh_replacements":
		return s.setSSHKeywordReplacements(request.Alias, request.KeywordReplacements)
	case "update_ssh_target":
		return s.updateSSHTarget(request.Alias, request.CreateSSH)
	case "rotate_ssh_credential":
		return s.rotateSSHCredential(request.Alias, request.CreateSSH)
	case "test_ssh_route":
		return s.testSSHRoute(request.Alias, request.Capability, request.Command)
	case "test_ssh_route_authentication":
		return s.testSSHRouteAuthentication(request.Alias)
	case "test_route_health":
		return s.testRouteHealth(request.Alias)
	case "test_proxy_health":
		return s.testProxyHealth()
	case "set_route_enabled":
		return s.setRouteEnabled(request.Alias, request.Enabled)
	case "stop_all":
		return s.stopAll()
	case "delete_route":
		return s.deleteRoute(request.Alias)
	case "configure_proxy":
		return s.configureProxy(request.ProxyURL)
	case "clear_proxy":
		return s.clearProxy()
	case "migrate_secret_store":
		return s.migrateSecretStore(request.SecretMode)
	case "cleanup_secret_store":
		return s.cleanupSecretStore(request.SecretMode)
	default:
		return Response{Error: "unsupported control action"}
	}
}

func (s *Server) ConfigureSecretStoreMigration(currentMode string, factory SecretStoreFactory) error {
	if !validSecretStoreMode(currentMode) || factory == nil {
		return errors.New("invalid secret store migration configuration")
	}
	s.secretMode = currentMode
	s.storeFactory = factory
	return nil
}

func (s *Server) migrateSecretStore(targetMode string) Response {
	if s.storeFactory == nil || targetMode == s.secretMode || !validSecretStoreMode(targetMode) {
		return Response{Error: "invalid secret store migration"}
	}
	destination, err := s.storeFactory(targetMode)
	if err != nil {
		return Response{Error: "could not open target secret store"}
	}
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	ctx := context.Background()
	for _, route := range s.registry.List() {
		target, err := s.secrets.ResolveHTTPTarget(ctx, route.TargetSecretRef)
		if err != nil || destination.PutHTTPTarget(ctx, route.TargetSecretRef, target) != nil {
			return Response{Error: "could not migrate protected HTTP target"}
		}
		if _, err := destination.ResolveHTTPTarget(ctx, route.TargetSecretRef); err != nil {
			return Response{Error: "could not verify protected HTTP target"}
		}
	}
	if s.ssh != nil {
		for _, route := range s.ssh.Registry.List() {
			target, err := s.secrets.ResolveSSHTarget(ctx, route.TargetSecretRef)
			if err != nil {
				return Response{Error: "could not migrate protected SSH target"}
			}
			putErr := destination.PutSSHTarget(ctx, route.TargetSecretRef, target)
			clearResolvedSSHTarget(&target)
			if putErr != nil {
				return Response{Error: "could not migrate protected SSH target"}
			}
			verified, err := destination.ResolveSSHTarget(ctx, route.TargetSecretRef)
			if err != nil {
				return Response{Error: "could not verify protected SSH target"}
			}
			clearResolvedSSHTarget(&verified)
		}
	}
	proxy, err := s.secrets.ResolveProxyConfig(ctx, egress.DefaultSecretReference)
	if err == nil {
		if destination.PutProxyConfig(ctx, egress.DefaultSecretReference, proxy) != nil {
			return Response{Error: "could not migrate protected proxy"}
		}
		if _, err := destination.ResolveProxyConfig(ctx, egress.DefaultSecretReference); err != nil {
			return Response{Error: "could not verify protected proxy"}
		}
	} else if !errors.Is(err, secrets.ErrNotFound) {
		return Response{Error: "could not read protected proxy"}
	}
	identity, err := s.secrets.ResolveSSHHostIdentity(ctx, sshgw.HostIdentitySecretReference)
	if err == nil {
		putErr := destination.PutSSHHostIdentity(ctx, sshgw.HostIdentitySecretReference, identity)
		clear(identity.PrivateKey)
		if putErr != nil {
			return Response{Error: "could not migrate SSH host identity"}
		}
		verified, err := destination.ResolveSSHHostIdentity(ctx, sshgw.HostIdentitySecretReference)
		if err != nil {
			return Response{Error: "could not verify SSH host identity"}
		}
		clear(verified.PrivateKey)
	} else if !errors.Is(err, secrets.ErrNotFound) {
		return Response{Error: "could not read SSH host identity"}
	}
	return s.successResponse()
}

func (s *Server) cleanupSecretStore(mode string) Response {
	if s.storeFactory == nil || mode == s.secretMode || !validSecretStoreMode(mode) {
		return Response{Error: "invalid secret store cleanup"}
	}
	store, err := s.storeFactory(mode)
	if err != nil {
		return Response{Error: "could not open previous secret store"}
	}
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	references := []string{egress.DefaultSecretReference, sshgw.HostIdentitySecretReference}
	for _, route := range s.registry.List() {
		references = append(references, route.TargetSecretRef)
	}
	if s.ssh != nil {
		for _, route := range s.ssh.Registry.List() {
			references = append(references, route.TargetSecretRef)
		}
	}
	failed := false
	for _, reference := range references {
		if err := store.DeleteTarget(context.Background(), reference); err != nil && !errors.Is(err, secrets.ErrNotFound) {
			failed = true
		}
	}
	response := s.successResponse()
	if failed {
		response.Warning = "new secret store is active, but some previous copies could not be removed"
	}
	return response
}

func validSecretStoreMode(mode string) bool {
	return mode == secrets.StoreModeKeychain || mode == secrets.StoreModeLocalFile
}

func clearResolvedSSHTarget(target *secrets.SSHTarget) {
	clear(target.Password)
	clear(target.PrivateKey)
	clear(target.PrivateKeyPassword)
	clear(target.ExpectedHostKey)
}

func (s *Server) createHTTPRoute(input *CreateHTTPRoute) Response {
	if input == nil || strings.TrimSpace(input.Name) == "" || len(input.Name) > 80 || len(input.Authorization) > 8<<10 || strings.ContainsAny(input.Authorization, "\r\n") {
		return Response{Error: "invalid route details"}
	}
	baseURL, err := url.Parse(input.BaseURL)
	if err != nil || baseURL.Host == "" || (baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		return Response{Error: "target must be an HTTP or HTTPS URL"}
	}
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	if _, err := s.registry.Get(input.Alias); err == nil {
		return Response{Error: "route alias already exists"}
	} else if !errors.Is(err, routes.ErrNotFound) {
		return Response{Error: "could not inspect route registry"}
	}
	if s.ssh != nil {
		if _, err := s.ssh.Registry.Get(input.Alias); err == nil {
			return Response{Error: "route alias already exists"}
		} else if !errors.Is(err, sshgw.ErrRouteNotFound) {
			return Response{Error: "could not inspect route registry"}
		}
	}
	token, digest, err := capability.Generate()
	if err != nil {
		return Response{Error: "could not create route capability"}
	}
	reference := "routes/" + input.Alias
	headers := make(http.Header)
	if input.Authorization != "" {
		headers.Set("Authorization", input.Authorization)
	}
	if err := s.secrets.PutHTTPTarget(context.Background(), reference, secrets.HTTPTarget{BaseURL: baseURL, Headers: headers}); err != nil {
		return Response{Error: "could not store protected target"}
	}
	policy := input.Egress
	if policy == "" {
		policy = egress.Direct
	}
	route := routes.HTTPRoute{
		Name: input.Name, Alias: input.Alias, TargetSecretRef: reference,
		CapabilityDigest: digest, Policy: routes.NewHTTPPolicy([]string{http.MethodGet, http.MethodHead}, nil),
		Egress: policy, Enabled: false,
	}
	if err := s.registry.Upsert(route); err != nil {
		_ = s.secrets.DeleteTarget(context.Background(), reference)
		return Response{Error: "invalid route alias or policy"}
	}
	if err := s.persistence.Save(s.registry.List()); err != nil {
		_, _ = s.registry.Delete(route.Alias)
		_ = s.secrets.DeleteTarget(context.Background(), reference)
		return Response{Error: "could not persist route metadata"}
	}
	summary := summarize(route, s.httpAddress)
	response := s.successResponse()
	response.Created = &CreatedRoute{Route: summary, Capability: token}
	return response
}

func (s *Server) createLLMRoute(input *CreateHTTPRoute) Response {
	if input == nil || strings.TrimSpace(input.Name) == "" || len(input.Name) > 80 || !validEgressPolicy(input.Egress) || len(input.Authorization) == 0 || len(input.Authorization) > 16<<10 || strings.ContainsAny(input.Authorization, "\x00\r\n") || (input.LocalAPIKey != "" && !validLocalAPIKey(input.LocalAPIKey)) {
		return Response{Error: "invalid LLM route details"}
	}
	if input.Provider != routes.ProviderOpenAI && input.Provider != routes.ProviderAnthropic {
		return Response{Error: "unsupported LLM provider preset"}
	}
	models, ok := normalizeLLMModels(input.Models)
	if !ok || !validLLMLimits(input.MaxOutputTokens, input.RequestsPerMinute, input.MaxConcurrent, true) {
		return Response{Error: "invalid LLM policy"}
	}
	baseURL, err := url.Parse(strings.TrimSpace(input.BaseURL))
	if err != nil || baseURL.Host == "" || (baseURL.Scheme != "http" && baseURL.Scheme != "https") || baseURL.User != nil || baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return Response{Error: "target must be an HTTP or HTTPS base URL without credentials or query parameters"}
	}
	baseURL.Path = strings.TrimSuffix(strings.TrimRight(baseURL.Path, "/"), "/v1")
	baseURL.RawPath = ""

	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	if _, err := s.registry.Get(input.Alias); err == nil {
		return Response{Error: "route alias already exists"}
	} else if !errors.Is(err, routes.ErrNotFound) {
		return Response{Error: "could not inspect route registry"}
	}
	if s.ssh != nil {
		if _, err := s.ssh.Registry.Get(input.Alias); err == nil {
			return Response{Error: "route alias already exists"}
		} else if !errors.Is(err, sshgw.ErrRouteNotFound) {
			return Response{Error: "could not inspect route registry"}
		}
	}

	var token string
	var digest capability.Digest
	if input.LocalAPIKey == "" {
		token, digest, err = capability.Generate()
		if err != nil {
			return Response{Error: "could not create local LLM API key"}
		}
	} else {
		digest = capability.Hash(input.LocalAPIKey)
	}
	headers := make(http.Header)
	if input.Provider == routes.ProviderAnthropic {
		headers.Set("X-Api-Key", input.Authorization)
		headers.Set("Anthropic-Version", "2023-06-01")
	} else {
		headers.Set("Authorization", "Bearer "+input.Authorization)
	}
	reference := "routes/" + input.Alias
	if err := s.secrets.PutHTTPTarget(context.Background(), reference, secrets.HTTPTarget{BaseURL: baseURL, Headers: headers}); err != nil {
		return Response{Error: "could not store protected LLM target"}
	}
	policy := input.Egress
	if policy == "" {
		policy = egress.Direct
	}
	llmPolicy := routes.NewLLMPolicy(input.Provider, models, input.MaxOutputTokens)
	if input.RequestsPerMinute > 0 {
		llmPolicy.RequestsPerMinute = input.RequestsPerMinute
	}
	if input.MaxConcurrent > 0 {
		llmPolicy.MaxConcurrent = input.MaxConcurrent
	}
	llmPolicy.TrackUsage = input.TrackUsage
	route := routes.HTTPRoute{
		Name: input.Name, Alias: input.Alias, Kind: routes.KindLLM, Provider: input.Provider,
		TargetSecretRef: reference, CapabilityDigest: digest,
		Policy: llmPolicy,
		Egress: policy, Enabled: false,
	}
	if err := s.registry.Upsert(route); err != nil {
		_ = s.secrets.DeleteTarget(context.Background(), reference)
		return Response{Error: "invalid LLM alias or policy"}
	}
	if err := s.persistence.Save(s.registry.List()); err != nil {
		_, _ = s.registry.Delete(route.Alias)
		_ = s.secrets.DeleteTarget(context.Background(), reference)
		return Response{Error: "could not persist LLM route metadata"}
	}
	response := s.successResponse()
	response.Created = &CreatedRoute{Route: summarize(route, s.httpAddress), Capability: token}
	return response
}

func (s *Server) setLLMPolicy(alias string, input *CreateHTTPRoute) Response {
	if input == nil || !validLLMLimits(input.MaxOutputTokens, input.RequestsPerMinute, input.MaxConcurrent, false) {
		return Response{Error: "invalid LLM policy"}
	}
	models, ok := normalizeLLMModels(input.Models)
	if !ok {
		return Response{Error: "invalid LLM model allowlist"}
	}

	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	previous, err := s.registry.Get(alias)
	if err != nil || previous.EffectiveKind() != routes.KindLLM {
		return Response{Error: "LLM route was not found"}
	}
	updated := previous
	updated.Policy = routes.NewLLMPolicy(previous.Provider, models, input.MaxOutputTokens)
	updated.Policy.RequestsPerMinute = input.RequestsPerMinute
	updated.Policy.MaxConcurrent = input.MaxConcurrent
	updated.Policy.TrackUsage = input.TrackUsage
	if err := s.registry.Upsert(updated); err != nil {
		return Response{Error: "invalid LLM policy"}
	}
	if err := s.persistence.Save(s.registry.List()); err != nil {
		_ = s.registry.Upsert(previous)
		return Response{Error: "could not persist LLM policy"}
	}
	return s.successResponse()
}

func (s *Server) rotateLLMAPIKey(alias string, input *CreateHTTPRoute) Response {
	if input == nil || (input.LocalAPIKey != "" && !validLocalAPIKey(input.LocalAPIKey)) {
		return Response{Error: "invalid local LLM API key"}
	}
	var (
		token  string
		digest capability.Digest
		err    error
	)
	if input.LocalAPIKey == "" {
		token, digest, err = capability.Generate()
		if err != nil {
			return Response{Error: "could not rotate local LLM API key"}
		}
	} else {
		digest = capability.Hash(input.LocalAPIKey)
	}

	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	previous, err := s.registry.Get(alias)
	if err != nil || previous.EffectiveKind() != routes.KindLLM {
		return Response{Error: "LLM route was not found"}
	}
	updated := previous
	updated.CapabilityDigest = digest
	if err := s.registry.Upsert(updated); err != nil {
		return Response{Error: "could not rotate local LLM API key"}
	}
	if err := s.persistence.Save(s.registry.List()); err != nil {
		_ = s.registry.Upsert(previous)
		return Response{Error: "could not persist rotated LLM API key"}
	}
	response := s.successResponse()
	response.Created = &CreatedRoute{Route: summarize(updated, s.httpAddress), Capability: token}
	return response
}

func (s *Server) resetLLMUsage(alias string) Response {
	if err := s.registry.ResetLLMUsage(alias); err != nil {
		return Response{Error: "LLM route was not found"}
	}
	return s.successResponse()
}

func normalizeLLMModels(input []string) ([]string, bool) {
	if len(input) == 0 || len(input) > 32 {
		return nil, false
	}
	seen := make(map[string]struct{}, len(input))
	models := make([]string, 0, len(input))
	for _, model := range input {
		model = strings.TrimSpace(model)
		if model == "" || len(model) > 200 || strings.ContainsAny(model, "\x00\r\n\t") {
			return nil, false
		}
		if _, duplicate := seen[model]; duplicate {
			continue
		}
		seen[model] = struct{}{}
		models = append(models, model)
	}
	return models, len(models) > 0
}

func validLLMLimits(maxOutputTokens, requestsPerMinute, maxConcurrent int, allowDefaults bool) bool {
	if maxOutputTokens < 1 || maxOutputTokens > 1_000_000 {
		return false
	}
	if allowDefaults && requestsPerMinute == 0 && maxConcurrent == 0 {
		return true
	}
	return requestsPerMinute >= 1 && requestsPerMinute <= 60_000 && maxConcurrent >= 1 && maxConcurrent <= 1_024
}

func validLocalAPIKey(value string) bool {
	return len(value) >= 16 && len(value) <= 1024 && !strings.ContainsRune(value, '\x00') && !strings.ContainsFunc(value, unicode.IsSpace)
}

func (s *Server) probeSSHHostKey(input *ProbeSSHHostKey) Response {
	if s.ssh == nil || input == nil || len(input.Address) > 512 || !validEgressPolicy(input.Egress) {
		return Response{Error: "invalid SSH probe details"}
	}
	address, err := normalizeSSHAddress(input.Address)
	if err != nil {
		return Response{Error: "invalid SSH address"}
	}
	if s.sshTargetIsLocalListener(address) {
		return Response{Error: "SSH 上游地址指向 Airlock 本地监听地址"}
	}
	policy := input.Egress
	if policy == "" {
		policy = egress.Direct
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	key, err := sshgw.ProbeHostKey(ctx, s.egress, policy, address)
	if err != nil {
		return Response{Error: "上游 SSH 服务未返回 Host Key，请检查地址、端口和出口策略"}
	}
	response := s.successResponse()
	response.SSHHostKeyProbe = &SSHHostKeyProbe{
		HostKey:     base64.StdEncoding.EncodeToString(key.Marshal()),
		Fingerprint: ssh.FingerprintSHA256(key),
	}
	return response
}

func (s *Server) sshTargetIsLocalListener(address string) bool {
	if s.ssh == nil {
		return false
	}
	targetHost, targetPort, err := net.SplitHostPort(address)
	if err != nil || !hostIsLocal(targetHost) {
		return false
	}
	for _, listener := range []string{s.ssh.ListenAddress, s.ssh.DialAddress} {
		_, listenerPort, splitErr := net.SplitHostPort(listener)
		if splitErr == nil && listenerPort == targetPort {
			return true
		}
	}
	return false
}

func hostIsLocal(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsUnspecified() {
		return true
	}
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, address := range addresses {
		localIP, _, parseErr := net.ParseCIDR(address.String())
		if parseErr == nil && localIP.Equal(ip) {
			return true
		}
	}
	return false
}

func (s *Server) createSSHRoute(input *CreateSSHRoute) Response {
	if s.ssh == nil || input == nil || strings.TrimSpace(input.Name) == "" || len(input.Name) > 80 || len(input.Address) > 512 || input.Username == "" || len(input.Username) > 255 || len(input.Password) == 0 || len(input.Password) > 8<<10 || (input.LocalPassword != "" && !validLocalPassword(input.LocalPassword)) || len(input.ExpectedHostKey) > 16<<10 || !validEgressPolicy(input.Egress) {
		return Response{Error: "invalid SSH route details"}
	}
	address, err := normalizeSSHAddress(input.Address)
	if err != nil {
		return Response{Error: "invalid SSH address"}
	}
	authenticationTimeout, err := sshgw.NormalizeAuthenticationTimeoutSeconds(input.AuthenticationTimeoutSeconds)
	if err != nil {
		return Response{Error: "SSH 认证预算需要在 3 到 120 秒之间"}
	}
	hostKey, err := base64.StdEncoding.DecodeString(input.ExpectedHostKey)
	if err != nil || len(hostKey) == 0 {
		clear(hostKey)
		return Response{Error: "invalid SSH host key"}
	}
	defer clear(hostKey)
	password := []byte(input.Password)
	defer clear(password)

	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	if _, err := s.registry.Get(input.Alias); err == nil {
		return Response{Error: "本地路由别名已存在；相同上游 IP 可以复用，请更换本地别名"}
	} else if !errors.Is(err, routes.ErrNotFound) {
		return Response{Error: "could not inspect route registry"}
	}
	if _, err := s.ssh.Registry.Get(input.Alias); err == nil {
		return Response{Error: "本地路由别名已存在；相同上游 IP 可以复用，请更换本地别名"}
	} else if !errors.Is(err, sshgw.ErrRouteNotFound) {
		return Response{Error: "could not inspect route registry"}
	}
	localUsername := input.LocalUsername
	if localUsername == "" {
		localUsername = input.Alias
	}
	if _, err := s.ssh.Registry.GetByUsername(localUsername); err == nil {
		return Response{Error: "本地 SSH 用户名已存在；相同上游 IP 可以复用，请更换本地 SSH 用户名"}
	} else if !errors.Is(err, sshgw.ErrRouteNotFound) {
		return Response{Error: "could not inspect SSH username mappings"}
	}

	var token string
	var digest capability.Digest
	if input.LocalPassword == "" {
		token, digest, err = capability.Generate()
		if err != nil {
			return Response{Error: "could not create route capability"}
		}
	} else {
		digest = capability.Hash(input.LocalPassword)
	}
	policy := input.Egress
	if policy == "" {
		policy = egress.Direct
	}
	reference := "ssh/" + input.Alias
	target := secrets.SSHTarget{
		Address: address, Username: input.Username, Password: password,
		ExpectedHostKey: hostKey,
	}
	if err := s.secrets.PutSSHTarget(context.Background(), reference, target); err != nil {
		return Response{Error: "could not store protected SSH target"}
	}
	commands := []string{input.AllowedCommand}
	if input.AllowAllCommands {
		commands = nil
	}
	sshPolicy := sshgw.NewPolicyWithOptions(
		commands, nil, false, input.AllowAllCommands, input.RecordCommands,
	)
	sshPolicy.AllowSFTP = input.AllowSFTP
	route := sshgw.Route{
		Name: input.Name, Alias: input.Alias, LocalUsername: localUsername,
		TargetSecretRef:  reference,
		CapabilityDigest: digest,
		Policy:           sshPolicy,
		Egress:           policy, AuthenticationTimeoutSeconds: authenticationTimeout,
		KeywordReplacements: append([]sshgw.KeywordReplacement(nil), input.KeywordReplacements...), Enabled: false,
	}
	if err := s.ssh.Registry.Upsert(route); err != nil {
		_ = s.secrets.DeleteTarget(context.Background(), reference)
		return Response{Error: "invalid SSH alias, local username, or command policy"}
	}
	if err := s.ssh.Persistence.Save(s.ssh.Registry.List()); err != nil {
		_, _ = s.ssh.Registry.Delete(route.Alias)
		_ = s.secrets.DeleteTarget(context.Background(), reference)
		return Response{Error: "could not persist SSH route metadata"}
	}
	response := s.successResponse()
	response.Created = &CreatedRoute{Route: summarizeSSH(route, s.ssh.ListenAddress), Capability: token}
	return response
}

func validLocalPassword(password string) bool {
	return len(password) >= 12 && len(password) <= 1024 && strings.TrimSpace(password) != "" && !strings.ContainsAny(password, "\x00\r\n")
}

func (s *Server) setSSHPolicy(alias string, input *SSHPolicyUpdate) Response {
	if s.ssh == nil || input == nil || (input.Name != "" && (strings.TrimSpace(input.Name) == "" || len(input.Name) > 80)) || !validEgressPolicy(input.Egress) {
		return Response{Error: "SSH 路由策略不可用"}
	}
	commands := []string{input.AllowedCommand}
	if input.AllowAllCommands {
		commands = nil
	}
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	previous, err := s.ssh.Registry.Get(alias)
	if err != nil {
		return Response{Error: "未找到 SSH 路由"}
	}
	localUsername := input.LocalUsername
	if localUsername == "" {
		localUsername = previous.EffectiveLocalUsername()
	}
	fingerprints := make([]string, 0, len(previous.Policy.LocalPublicKeyFingerprints))
	for fingerprint := range previous.Policy.LocalPublicKeyFingerprints {
		fingerprints = append(fingerprints, fingerprint)
	}
	updated := previous
	if input.AuthenticationTimeoutSeconds != 0 {
		authenticationTimeout, timeoutErr := sshgw.NormalizeAuthenticationTimeoutSeconds(input.AuthenticationTimeoutSeconds)
		if timeoutErr != nil {
			return Response{Error: "SSH 认证预算需要在 3 到 120 秒之间"}
		}
		updated.AuthenticationTimeoutSeconds = authenticationTimeout
	} else {
		updated.AuthenticationTimeoutSeconds = previous.EffectiveAuthenticationTimeoutSeconds()
	}
	updated.LocalUsername = localUsername
	updated.Policy = sshgw.NewPolicyWithOptions(commands, fingerprints, false, input.AllowAllCommands, input.RecordCommands)
	updated.Policy.AllowSFTP = input.AllowSFTP
	if input.Name != "" {
		updated.Name = strings.TrimSpace(input.Name)
	}
	if input.Egress != "" {
		updated.Egress = input.Egress
	}
	if err := s.ssh.Registry.Upsert(updated); err != nil {
		if errors.Is(err, sshgw.ErrLocalUsernameInUse) {
			return Response{Error: "本地 SSH 用户名已存在；相同上游 IP 可以复用，请更换本地 SSH 用户名"}
		}
		return Response{Error: "SSH 命令策略无效"}
	}
	if err := s.ssh.Persistence.Save(s.ssh.Registry.List()); err != nil {
		_ = s.ssh.Registry.Upsert(previous)
		return Response{Error: "无法保存 SSH 映射策略"}
	}
	return s.successResponse()
}

func (s *Server) getSSHKeywordReplacements(alias string) Response {
	if s.ssh == nil || strings.TrimSpace(alias) == "" || len(alias) > 63 {
		return Response{Error: "SSH 出口替换不可用"}
	}
	route, err := s.ssh.Registry.Get(alias)
	if err != nil {
		return Response{Error: "未找到 SSH 路由"}
	}
	response := s.successResponse()
	response.KeywordReplacements = append([]sshgw.KeywordReplacement(nil), route.KeywordReplacements...)
	return response
}

func (s *Server) setSSHKeywordReplacements(alias string, replacements []sshgw.KeywordReplacement) Response {
	if s.ssh == nil || strings.TrimSpace(alias) == "" || len(alias) > 63 || sshgw.ValidateKeywordReplacements(replacements) != nil {
		return Response{Error: "SSH 出口替换规则无效"}
	}
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	previous, err := s.ssh.Registry.Get(alias)
	if err != nil {
		return Response{Error: "未找到 SSH 路由"}
	}
	updated := previous
	updated.KeywordReplacements = append([]sshgw.KeywordReplacement(nil), replacements...)
	if err := s.ssh.Registry.Upsert(updated); err != nil {
		return Response{Error: "SSH 出口替换规则无效"}
	}
	if err := s.ssh.Persistence.Save(s.ssh.Registry.List()); err != nil {
		_ = s.ssh.Registry.Upsert(previous)
		return Response{Error: "无法保存 SSH 出口替换规则"}
	}
	return s.successResponse()
}

func (s *Server) updateSSHTarget(alias string, input *CreateSSHRoute) Response {
	if s.ssh == nil || input == nil || input.Username == "" || len(input.Username) > 255 || len(input.Password) == 0 || len(input.Password) > 8<<10 || len(input.ExpectedHostKey) > 16<<10 || !validEgressPolicy(input.Egress) {
		return Response{Error: "SSH 宿主机更新参数无效"}
	}
	address, err := normalizeSSHAddress(input.Address)
	if err != nil || s.sshTargetIsLocalListener(address) {
		return Response{Error: "SSH 宿主地址无效或指向 Airlock 自身"}
	}
	hostKey, err := base64.StdEncoding.DecodeString(input.ExpectedHostKey)
	if err != nil || len(hostKey) == 0 {
		clear(hostKey)
		return Response{Error: "SSH Host Key 无效"}
	}
	defer clear(hostKey)
	password := []byte(input.Password)
	defer clear(password)

	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	route, err := s.ssh.Registry.Get(alias)
	if err != nil {
		return Response{Error: "未找到 SSH 路由"}
	}
	target := secrets.SSHTarget{
		Address: address, Username: input.Username, Password: password,
		ExpectedHostKey: hostKey,
	}
	if err := s.secrets.PutSSHTarget(context.Background(), route.TargetSecretRef, target); err != nil {
		return Response{Error: "无法替换受保护 SSH 宿主机"}
	}
	s.clearRouteHealth(alias)
	return s.successResponse()
}

func (s *Server) rotateSSHCredential(alias string, input *CreateSSHRoute) Response {
	if s.ssh == nil || input == nil || (input.LocalPassword != "" && !validLocalPassword(input.LocalPassword)) {
		return Response{Error: "本地 SSH 凭据无效"}
	}
	var token string
	var digest capability.Digest
	var err error
	if input.LocalPassword == "" {
		token, digest, err = capability.Generate()
		if err != nil {
			return Response{Error: "无法生成本地 SSH 凭据"}
		}
	} else {
		digest = capability.Hash(input.LocalPassword)
	}

	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	previous, err := s.ssh.Registry.Get(alias)
	if err != nil {
		return Response{Error: "未找到 SSH 路由"}
	}
	updated := previous
	updated.CapabilityDigest = digest
	if err := s.ssh.Registry.Upsert(updated); err != nil {
		return Response{Error: "无法轮换本地 SSH 凭据"}
	}
	if err := s.ssh.Persistence.Save(s.ssh.Registry.List()); err != nil {
		_ = s.ssh.Registry.Upsert(previous)
		return Response{Error: "无法保存本地 SSH 凭据"}
	}
	response := s.successResponse()
	response.Created = &CreatedRoute{Route: summarizeSSH(updated, s.ssh.ListenAddress), Capability: token}
	return response
}

func (s *Server) testSSHRoute(alias, token, command string) Response {
	if s.ssh == nil || token == "" {
		return Response{Error: "SSH route test is unavailable"}
	}
	route, err := s.ssh.Registry.Lookup(alias)
	if err != nil || !route.Policy.AllowsCommand(command) {
		return Response{Error: "SSH route test was rejected"}
	}
	dialAddress := s.ssh.DialAddress
	if dialAddress == "" {
		dialAddress = s.ssh.ListenAddress
	}
	raw, err := net.DialTimeout("tcp", dialAddress, 5*time.Second)
	if err != nil {
		return Response{Error: "local SSH route is unavailable"}
	}
	defer raw.Close()
	_ = raw.SetDeadline(time.Now().Add(20 * time.Second))
	algorithms := ssh.SupportedAlgorithms()
	config := &ssh.ClientConfig{
		Config: ssh.Config{KeyExchanges: algorithms.KeyExchanges, Ciphers: algorithms.Ciphers, MACs: algorithms.MACs},
		User:   route.EffectiveLocalUsername(), Auth: []ssh.AuthMethod{ssh.Password(token)},
		HostKeyCallback: ssh.FixedHostKey(s.ssh.HostKey), HostKeyAlgorithms: algorithms.HostKeys,
		ClientVersion: "SSH-2.0-Airlock-Self-Test", Timeout: 5 * time.Second,
	}
	connection, channels, requests, err := ssh.NewClientConn(raw, dialAddress, config)
	if err != nil {
		return Response{Error: "local SSH authentication test failed"}
	}
	client := ssh.NewClient(connection, channels, requests)
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return Response{Error: "upstream SSH session test failed"}
	}
	defer session.Close()
	session.Stdout = io.Discard
	session.Stderr = io.Discard
	if err := session.Run(command); err != nil {
		return Response{Error: "upstream SSH command test failed"}
	}
	return s.successResponse()
}

func (s *Server) testSSHRouteAuthentication(alias string) Response {
	if s.ssh == nil || s.secrets == nil || s.egress == nil || strings.TrimSpace(alias) == "" || len(alias) > 63 {
		return Response{Error: "SSH 认证测试不可用"}
	}
	route, err := s.ssh.Registry.Get(alias)
	if err != nil {
		return Response{Error: "未找到 SSH 路由"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(route.EffectiveAuthenticationTimeoutSeconds())*time.Second)
	defer cancel()
	target, err := s.secrets.ResolveSSHTarget(ctx, route.TargetSecretRef)
	if err != nil {
		return Response{Error: "上游 SSH 认证测试失败"}
	}
	if err := sshgw.VerifyUpstreamAuthentication(ctx, s.egress, route, &target); err != nil {
		switch {
		case errors.Is(err, sshgw.ErrUpstreamAuth):
			return Response{Error: "上游 SSH 账号或密码被拒绝"}
		case errors.Is(err, sshgw.ErrHostKeyMismatch):
			return Response{Error: "上游 SSH Host Key 校验失败"}
		default:
			return Response{Error: "上游 SSH 服务不可达"}
		}
	}
	return s.successResponse()
}

func (s *Server) testRouteHealth(alias string) Response {
	if strings.TrimSpace(alias) == "" || len(alias) > 63 || s.egress == nil || s.secrets == nil {
		return Response{Error: "route health check is unavailable"}
	}
	started := time.Now()
	checkedAt := started.UTC()
	status := "degraded"
	result := "failed"
	message := "上游不可达或受保护配置不可用"
	category := ""
	egressPolicy := egress.Direct

	if route, err := s.registry.Get(alias); err == nil {
		category = route.EffectiveKind()
		egressPolicy = effectiveEgress(route.Egress)
		if checkMessage, healthy := s.checkHTTPRoute(route); healthy {
			status, result, message = "healthy", "allowed", checkMessage
		} else {
			message = checkMessage
		}
	} else if errors.Is(err, routes.ErrNotFound) && s.ssh != nil {
		route, sshErr := s.ssh.Registry.Get(alias)
		if sshErr != nil {
			return Response{Error: "route was not found"}
		}
		category = "SSH"
		egressPolicy = effectiveEgress(route.Egress)
		if checkMessage, healthy := s.checkSSHRoute(route); healthy {
			status, result, message = "healthy", "allowed", checkMessage
		} else {
			message = checkMessage
		}
	} else {
		return Response{Error: "route was not found"}
	}

	duration := time.Since(started)
	s.setRouteHealth(alias, routeHealthState{Status: status, CheckedAt: checkedAt})
	if s.activity != nil {
		_ = s.activity.Record(activity.Event{
			RouteAlias: alias,
			Category:   category,
			EventType:  "health",
			Caller:     "Airlock Desktop",
			Action:     "Manual health check",
			Result:     result,
			DurationMS: duration.Milliseconds(),
			Egress:     egressPolicy,
		})
	}
	response := s.successResponse()
	response.HealthCheck = &HealthCheckSummary{
		Alias: alias, Status: status, Message: message,
		Latency:   fmt.Sprintf("%d ms", duration.Milliseconds()),
		CheckedAt: checkedAt.Local().Format("01-02 15:04:05"),
	}
	return response
}

func (s *Server) testProxyHealth() Response {
	started := time.Now()
	checkedAt := started.UTC()
	status, result, message := "degraded", "failed", "代理尚未配置或受保护配置不可用"
	if s.secrets != nil {
		config, err := s.secrets.ResolveProxyConfig(context.Background(), egress.DefaultSecretReference)
		if err == nil && config.URL != nil {
			host, port := config.URL.Hostname(), config.URL.Port()
			if port == "" {
				switch strings.ToLower(config.URL.Scheme) {
				case "http":
					port = "80"
				case "https":
					port = "443"
				case "socks5", "socks5h":
					port = "1080"
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			connection, dialErr := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(host, port))
			cancel()
			if dialErr == nil {
				_ = connection.Close()
				status, result, message = "healthy", "allowed", "本地代理 TCP 端口可达"
			} else {
				message = "无法连接本地代理 TCP 端口"
			}
		}
	}
	duration := time.Since(started)
	if s.activity != nil {
		_ = s.activity.Record(activity.Event{
			RouteAlias: "proxy", Category: "System", EventType: "health",
			Caller: "Airlock Desktop", Action: "Manual proxy health check", Result: result,
			DurationMS: duration.Milliseconds(), Egress: egress.Direct,
		})
	}
	response := s.successResponse()
	response.HealthCheck = &HealthCheckSummary{
		Alias: "proxy", Status: status, Message: message,
		Latency: fmt.Sprintf("%d ms", duration.Milliseconds()), CheckedAt: checkedAt.Local().Format("01-02 15:04:05"),
	}
	return response
}

func (s *Server) checkHTTPRoute(route routes.HTTPRoute) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	target, err := s.secrets.ResolveHTTPTarget(ctx, route.TargetSecretRef)
	if err != nil || target.BaseURL == nil {
		return "受保护目标无法读取", false
	}
	scheme := strings.ToLower(target.BaseURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return "目标协议不受支持", false
	}
	host := target.BaseURL.Hostname()
	port := target.BaseURL.Port()
	if port == "" {
		port = map[string]string{"http": "80", "https": "443"}[scheme]
	}
	connection, err := s.egress.DialContext(ctx, effectiveEgress(route.Egress), "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return "TCP 连通性检查失败", false
	}
	defer connection.Close()
	if scheme == "https" {
		tlsConnection := tls.Client(connection, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		if err := tlsConnection.HandshakeContext(ctx); err != nil {
			return "TCP 可达，但 TLS 证书或握手校验失败", false
		}
		return "TCP 与 TLS 校验通过，未发送业务请求", true
	}
	return "目标 TCP 端口可达，未发送业务请求", true
}

func (s *Server) checkSSHRoute(route sshgw.Route) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(route.EffectiveAuthenticationTimeoutSeconds())*time.Second)
	defer cancel()
	target, err := s.secrets.ResolveSSHTarget(ctx, route.TargetSecretRef)
	if err != nil {
		return "受保护 SSH 目标无法读取", false
	}
	defer clearResolvedSSHTarget(&target)
	if err := sshgw.VerifyUpstreamAuthentication(ctx, s.egress, route, &target); err != nil {
		switch {
		case errors.Is(err, sshgw.ErrUpstreamAuth):
			return "SSH Host Key 已固定，但上游账号或密码被拒绝", false
		case errors.Is(err, sshgw.ErrHostKeyMismatch):
			return "SSH Host Key 与固定值不一致", false
		default:
			return "上游 SSH 服务不可达或认证超时", false
		}
	}
	return "SSH Host Key 与上游身份认证通过，未执行命令", true
}

func effectiveEgress(policy string) string {
	if policy == "" {
		return egress.Direct
	}
	return policy
}

func (s *Server) setRouteHealth(alias string, state routeHealthState) {
	s.healthMu.Lock()
	defer s.healthMu.Unlock()
	if s.health == nil {
		s.health = make(map[string]routeHealthState)
	}
	s.health[alias] = state
}

func (s *Server) clearRouteHealth(alias string) {
	s.healthMu.Lock()
	defer s.healthMu.Unlock()
	delete(s.health, alias)
}

func (s *Server) routeHealth(alias string) string {
	s.healthMu.RLock()
	defer s.healthMu.RUnlock()
	state, ok := s.health[alias]
	if !ok || (state.Status != "healthy" && state.Status != "degraded") {
		return "unknown"
	}
	return state.Status
}

func (s *Server) setRouteEnabled(alias string, enabled bool) Response {
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	previous, err := s.registry.Get(alias)
	if err == nil {
		if err := s.registry.SetEnabled(alias, enabled); err != nil {
			return Response{Error: "route was not found"}
		}
		if err := s.persistence.Save(s.registry.List()); err != nil {
			_ = s.registry.SetEnabled(alias, previous.Enabled)
			return Response{Error: "could not persist route status"}
		}
		return s.successResponse()
	}
	if !errors.Is(err, routes.ErrNotFound) || s.ssh == nil {
		return Response{Error: "route was not found"}
	}
	previousSSH, err := s.ssh.Registry.Get(alias)
	if err != nil {
		return Response{Error: "route was not found"}
	}
	if err := s.ssh.Registry.SetEnabled(alias, enabled); err != nil {
		return Response{Error: "route was not found"}
	}
	if err := s.ssh.Persistence.Save(s.ssh.Registry.List()); err != nil {
		_ = s.ssh.Registry.SetEnabled(alias, previousSSH.Enabled)
		return Response{Error: "could not persist SSH route status"}
	}
	return s.successResponse()
}

func (s *Server) stopAll() Response {
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	previous := s.registry.List()
	var previousSSH []sshgw.Route
	if s.ssh != nil {
		previousSSH = s.ssh.Registry.List()
	}
	s.registry.DisableAll()
	if s.ssh != nil {
		s.ssh.Registry.DisableAll()
	}
	if err := s.persistence.Save(s.registry.List()); err != nil {
		for _, route := range previous {
			_ = s.registry.SetEnabled(route.Alias, route.Enabled)
		}
		return Response{Error: "could not persist stopped routes"}
	}
	if s.ssh != nil {
		if err := s.ssh.Persistence.Save(s.ssh.Registry.List()); err != nil {
			for _, route := range previous {
				_ = s.registry.SetEnabled(route.Alias, route.Enabled)
			}
			for _, route := range previousSSH {
				_ = s.ssh.Registry.SetEnabled(route.Alias, route.Enabled)
			}
			_ = s.persistence.Save(previous)
			return Response{Error: "could not persist stopped SSH routes"}
		}
	}
	return s.successResponse()
}

func (s *Server) deleteRoute(alias string) Response {
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	deleted, err := s.registry.Delete(alias)
	if err == nil {
		if err := s.persistence.Save(s.registry.List()); err != nil {
			_ = s.registry.Upsert(deleted)
			return Response{Error: "could not persist route deletion"}
		}
		s.clearRouteHealth(alias)
		response := s.successResponse()
		if err := s.secrets.DeleteTarget(context.Background(), deleted.TargetSecretRef); err != nil && !errors.Is(err, secrets.ErrNotFound) {
			response.Warning = "route deleted, but protected target cleanup needs attention"
		}
		return response
	}
	if !errors.Is(err, routes.ErrNotFound) || s.ssh == nil {
		return Response{Error: "route was not found"}
	}
	deletedSSH, err := s.ssh.Registry.Delete(alias)
	if err != nil {
		return Response{Error: "route was not found"}
	}
	if err := s.ssh.Persistence.Save(s.ssh.Registry.List()); err != nil {
		_ = s.ssh.Registry.Upsert(deletedSSH)
		return Response{Error: "could not persist SSH route deletion"}
	}
	s.clearRouteHealth(alias)
	response := s.successResponse()
	if err := s.secrets.DeleteTarget(context.Background(), deletedSSH.TargetSecretRef); err != nil && !errors.Is(err, secrets.ErrNotFound) {
		response.Warning = "route deleted, but protected target cleanup needs attention"
	}
	return response
}

func (s *Server) configureProxy(rawURL string) Response {
	proxyURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || egress.ValidateProxyURL(proxyURL) != nil {
		return Response{Error: "proxy must use HTTP, HTTPS, SOCKS5, or SOCKS5H"}
	}
	proxyURL.Scheme = strings.ToLower(proxyURL.Scheme)
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	if err := s.secrets.PutProxyConfig(context.Background(), egress.DefaultSecretReference, secrets.ProxyConfig{URL: proxyURL}); err != nil {
		return Response{Error: "could not store protected proxy"}
	}
	if err := s.egress.Configure(proxyURL); err != nil {
		_ = s.secrets.DeleteTarget(context.Background(), egress.DefaultSecretReference)
		return Response{Error: "could not configure proxy egress"}
	}
	return s.successResponse()
}

func (s *Server) clearProxy() Response {
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	if err := s.secrets.DeleteTarget(context.Background(), egress.DefaultSecretReference); err != nil && !errors.Is(err, secrets.ErrNotFound) {
		return Response{Error: "could not remove protected proxy"}
	}
	s.egress.Clear()
	return s.successResponse()
}

func (s *Server) proxyConfigured() bool {
	return s.egress != nil && s.egress.Configured()
}

func (s *Server) successResponse() Response {
	return Response{
		OK: true, Running: true, Routes: s.routeSummaries(),
		ProxyConfigured: s.proxyConfigured(), SSHReady: s.ssh != nil,
		Activity: s.recentActivities(),
	}
}

func (s *Server) recentActivities() []ActivitySummary {
	result := make([]ActivitySummary, 0, 50)
	if s.activity != nil {
		for _, event := range s.activity.List(50) {
			result = append(result, ActivitySummary{
				ID: event.ID, Time: event.Time.Local().Format("01-02 15:04:05"),
				RouteName: s.routeDisplayName(event.RouteAlias), Caller: event.Caller,
				Action: event.Action, Detail: event.Action, Result: event.Result,
				Latency: fmt.Sprintf("%d ms", event.DurationMS), Egress: effectiveEgress(event.Egress),
				Category: event.Category, EventType: event.EventType, when: event.Time,
			})
		}
	}
	if s.ssh != nil && s.ssh.CommandAudit != nil {
		for _, event := range s.ssh.CommandAudit.List(50) {
			result = append(result, ActivitySummary{
				ID: event.ID, Time: event.Time.Local().Format("01-02 15:04:05"),
				RouteName: s.routeDisplayName(event.RouteAlias), Caller: event.RouteAlias + "@loopback",
				Action: commandPreview(event.Command), Detail: event.Command, Result: event.Result,
				Latency: fmt.Sprintf("%d ms", event.DurationMS), Egress: effectiveEgress(event.Egress),
				Category: "SSH", EventType: "command", when: event.Time,
			})
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].when.After(result[j].when) })
	if len(result) > 50 {
		result = result[:50]
	}
	return result
}

func (s *Server) routeDisplayName(alias string) string {
	if route, err := s.registry.Get(alias); err == nil && strings.TrimSpace(route.Name) != "" {
		return route.Name
	}
	if s.ssh != nil {
		if route, err := s.ssh.Registry.Get(alias); err == nil && strings.TrimSpace(route.Name) != "" {
			return route.Name
		}
	}
	return alias
}

func commandPreview(command string) string {
	const maxBytes = 512
	if len(command) <= maxBytes {
		return command
	}
	end := maxBytes
	for end > 0 && !utf8.ValidString(command[:end]) {
		end--
	}
	return command[:end] + "... (truncated)"
}

func (s *Server) routeSummaries() []RouteSummary {
	result := summaries(s.registry.List(), s.httpAddress)
	if s.ssh != nil {
		for _, route := range s.ssh.Registry.List() {
			result = append(result, summarizeSSH(route, s.ssh.ListenAddress))
		}
	}
	for index := range result {
		result[index].Health = s.routeHealth(result[index].Alias)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func summaries(all []routes.HTTPRoute, listenAddress string) []RouteSummary {
	result := make([]RouteSummary, 0, len(all))
	for _, route := range all {
		result = append(result, summarize(route, listenAddress))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func summarize(route routes.HTTPRoute, listenAddress string) RouteSummary {
	if listenAddress == "" {
		listenAddress = "127.0.0.1:4768"
	}
	name := strings.TrimSpace(route.Name)
	if name == "" {
		name = route.Alias
	}
	egress := route.Egress
	if egress == "" {
		egress = "Direct"
	}
	status := "disabled"
	if route.Enabled {
		status = "enabled"
	}
	kind := route.EffectiveKind()
	localEndpoint := listenAddress + "/r/" + route.Alias
	permissionSummary := "GET, HEAD · Range"
	var allowedModels []string
	if kind == routes.KindLLM {
		localEndpoint = "http://" + localEndpoint
		provider := "OpenAI"
		if route.Provider == routes.ProviderAnthropic {
			provider = "Anthropic"
		}
		allowedModels = make([]string, 0, len(route.Policy.AllowedModels))
		for model := range route.Policy.AllowedModels {
			allowedModels = append(allowedModels, model)
		}
		sort.Strings(allowedModels)
		permissionSummary = fmt.Sprintf("%s · %d models · output ≤ %d · %d/min · %d concurrent", provider, len(route.Policy.AllowedModels), route.Policy.MaxOutputTokens, route.Policy.RequestsPerMinute, route.Policy.MaxConcurrent)
	}
	return RouteSummary{
		ID: route.Alias, Name: name, Alias: route.Alias, Kind: kind, Status: status,
		LocalEndpoint: localEndpoint, PermissionSummary: permissionSummary,
		Egress: egress, Health: "unknown", LastUsed: "从未", CurrentConnections: 0,
		Provider: route.Provider, AllowedModels: allowedModels,
		MaxOutputTokens: route.Policy.MaxOutputTokens, RequestsPerMinute: route.Policy.RequestsPerMinute,
		MaxConcurrent: route.Policy.MaxConcurrent, TrackUsage: route.Policy.TrackUsage,
		TotalRequests: route.Usage.Requests, InputTokens: route.Usage.InputTokens, OutputTokens: route.Usage.OutputTokens,
	}
}

func summarizeSSH(route sshgw.Route, listenAddress string) RouteSummary {
	if listenAddress == "" {
		listenAddress = "127.0.0.1:4770"
	}
	name := strings.TrimSpace(route.Name)
	if name == "" {
		name = route.Alias
	}
	egressPolicy := route.Egress
	if egressPolicy == "" {
		egressPolicy = egress.Direct
	}
	status := "disabled"
	if route.Enabled {
		status = "enabled"
	}
	permissionSummary := "1 exact command · stdin denied"
	if route.Policy.AllowAllCommands {
		permissionSummary = "all exec commands · high risk"
	}
	if route.Policy.RecordCommands {
		permissionSummary += " · recorded"
	}
	if route.Policy.AllowSFTP {
		permissionSummary += " · SFTP high risk"
	}
	if len(route.KeywordReplacements) > 0 {
		permissionSummary += fmt.Sprintf(" · %d rewrite rules", len(route.KeywordReplacements))
	}
	allowedCommands := make([]string, 0, len(route.Policy.AllowedCommands))
	for command := range route.Policy.AllowedCommands {
		allowedCommands = append(allowedCommands, command)
	}
	sort.Strings(allowedCommands)
	allowedCommand := ""
	if len(allowedCommands) > 0 {
		allowedCommand = allowedCommands[0]
	}
	return RouteSummary{
		ID: route.Alias, Name: name, Alias: route.Alias, Kind: "SSH", Status: status,
		LocalUsername:     route.EffectiveLocalUsername(),
		LocalEndpoint:     route.EffectiveLocalUsername() + "@" + listenAddress,
		PermissionSummary: permissionSummary,
		Egress:            egressPolicy, Health: "unknown", LastUsed: "从未", CurrentConnections: 0,
		AllowAllCommands:             route.Policy.AllowAllCommands,
		RecordCommands:               route.Policy.RecordCommands,
		AllowSFTP:                    route.Policy.AllowSFTP,
		AllowedCommand:               allowedCommand,
		AuthenticationTimeoutSeconds: route.EffectiveAuthenticationTimeoutSeconds(),
		KeywordReplacementCount:      len(route.KeywordReplacements),
	}
}

func validEgressPolicy(policy string) bool {
	return policy == "" || policy == egress.Direct || policy == egress.Proxy || policy == egress.Auto
}

func normalizeSSHAddress(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\x00\r\n\t ") {
		return "", errors.New("invalid SSH address")
	}
	if host, port, err := net.SplitHostPort(value); err == nil {
		if host == "" || port == "" {
			return "", errors.New("invalid SSH address")
		}
		return value, nil
	}
	if ip := net.ParseIP(value); ip != nil {
		return net.JoinHostPort(value, "22"), nil
	}
	if !strings.Contains(value, ":") {
		return net.JoinHostPort(value, "22"), nil
	}
	return "", errors.New("invalid SSH address")
}

func validToken(actual, expected string) bool {
	return len(actual) == len(expected) && subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

func prepareDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return errors.New("create control directory")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("invalid control directory")
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return errors.New("protect control directory")
	}
	return nil
}

func removeStaleSocket(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || info.Mode()&os.ModeSocket == 0 {
		return errors.New("control socket path is not a socket")
	}
	if err := os.Remove(path); err != nil {
		return errors.New("remove stale control socket")
	}
	return nil
}

func writeResponse(writer io.Writer, response Response) {
	_ = json.NewEncoder(writer).Encode(response)
}
