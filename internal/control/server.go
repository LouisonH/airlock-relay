package control

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/LouisonH/airlock-relay/internal/capability"
	"github.com/LouisonH/airlock-relay/internal/routes"
	"github.com/LouisonH/airlock-relay/internal/secrets"
)

const (
	protocolVersion = 1
	maxMessageBytes = 64 << 10
)

type Paths struct {
	Directory string
	Socket    string
}

func DefaultPaths() (Paths, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, errors.New("locate user configuration directory")
	}
	directory := filepath.Join(configDir, "io.airlock.relay")
	return Paths{
		Directory: directory,
		Socket:    filepath.Join(directory, "control.sock"),
	}, nil
}

type Server struct {
	registry    *routes.Registry
	secrets     secrets.MutableStore
	persistence routes.MetadataStore
	token       string
	mutationMu  sync.Mutex
}

type Request struct {
	Version int              `json:"version"`
	Token   string           `json:"token"`
	Action  string           `json:"action"`
	Create  *CreateHTTPRoute `json:"create,omitempty"`
	Alias   string           `json:"alias,omitempty"`
	Enabled bool             `json:"enabled,omitempty"`
}

type CreateHTTPRoute struct {
	Name          string `json:"name"`
	Alias         string `json:"alias"`
	BaseURL       string `json:"base_url"`
	Authorization string `json:"authorization,omitempty"`
}

type Response struct {
	OK      bool           `json:"ok"`
	Error   string         `json:"error,omitempty"`
	Warning string         `json:"warning,omitempty"`
	Running bool           `json:"running"`
	Routes  []RouteSummary `json:"routes,omitempty"`
	Created *CreatedRoute  `json:"created,omitempty"`
}

type CreatedRoute struct {
	Route      RouteSummary `json:"route"`
	Capability string       `json:"capability"`
}

type RouteSummary struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Alias              string `json:"alias"`
	Kind               string `json:"kind"`
	Status             string `json:"status"`
	LocalEndpoint      string `json:"localEndpoint"`
	PermissionSummary  string `json:"permissionSummary"`
	Egress             string `json:"egress"`
	Health             string `json:"health"`
	LastUsed           string `json:"lastUsed"`
	CurrentConnections int    `json:"currentConnections"`
}

func Listen(paths Paths, token string, registry *routes.Registry, store secrets.MutableStore, persistence routes.MetadataStore) (net.Listener, *Server, error) {
	if len(token) < 32 || len(token) > 128 {
		return nil, nil, errors.New("invalid in-memory control token")
	}
	if persistence == nil {
		return nil, nil, errors.New("route metadata store is required")
	}
	if err := prepareDirectory(paths.Directory); err != nil {
		return nil, nil, err
	}
	if err := removeStaleSocket(paths.Socket); err != nil {
		return nil, nil, err
	}
	listener, err := net.Listen("unix", paths.Socket)
	if err != nil {
		return nil, nil, fmt.Errorf("listen on control socket: %w", err)
	}
	if err := os.Chmod(paths.Socket, 0o600); err != nil {
		_ = listener.Close()
		return nil, nil, errors.New("protect control socket")
	}
	return listener, &Server{registry: registry, secrets: store, persistence: persistence, token: token}, nil
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
	_ = connection.SetDeadline(time.Now().Add(10 * time.Second))

	decoder := json.NewDecoder(io.LimitReader(connection, maxMessageBytes))
	decoder.DisallowUnknownFields()
	var request Request
	if err := decoder.Decode(&request); err != nil {
		writeResponse(connection, Response{Error: "invalid control request"})
		return
	}
	response := s.dispatch(request)
	writeResponse(connection, response)
}

func (s *Server) dispatch(request Request) Response {
	if request.Version != protocolVersion || !validToken(request.Token, s.token) {
		return Response{Error: "control authentication failed"}
	}

	switch request.Action {
	case "status", "list_routes":
		return Response{OK: true, Running: true, Routes: summaries(s.registry.List())}
	case "create_http_route":
		return s.createHTTPRoute(request.Create)
	case "set_route_enabled":
		return s.setRouteEnabled(request.Alias, request.Enabled)
	case "stop_all":
		return s.stopAll()
	case "delete_route":
		return s.deleteRoute(request.Alias)
	default:
		return Response{Error: "unsupported control action"}
	}
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
	route := routes.HTTPRoute{
		Name: input.Name, Alias: input.Alias, TargetSecretRef: reference,
		CapabilityDigest: digest, Policy: routes.NewHTTPPolicy([]string{http.MethodGet, http.MethodHead}, nil),
		Egress: "Direct", Enabled: false,
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
	summary := summarize(route)
	return Response{OK: true, Running: true, Routes: summaries(s.registry.List()), Created: &CreatedRoute{Route: summary, Capability: token}}
}

func (s *Server) setRouteEnabled(alias string, enabled bool) Response {
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	previous, err := s.registry.Get(alias)
	if err != nil {
		return Response{Error: "route was not found"}
	}
	if err := s.registry.SetEnabled(alias, enabled); err != nil {
		return Response{Error: "route was not found"}
	}
	if err := s.persistence.Save(s.registry.List()); err != nil {
		_ = s.registry.SetEnabled(alias, previous.Enabled)
		return Response{Error: "could not persist route status"}
	}
	return Response{OK: true, Running: true, Routes: summaries(s.registry.List())}
}

func (s *Server) stopAll() Response {
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	previous := s.registry.List()
	s.registry.DisableAll()
	if err := s.persistence.Save(s.registry.List()); err != nil {
		for _, route := range previous {
			_ = s.registry.SetEnabled(route.Alias, route.Enabled)
		}
		return Response{Error: "could not persist stopped routes"}
	}
	return Response{OK: true, Running: true, Routes: summaries(s.registry.List())}
}

func (s *Server) deleteRoute(alias string) Response {
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	deleted, err := s.registry.Delete(alias)
	if err != nil {
		return Response{Error: "route was not found"}
	}
	if err := s.persistence.Save(s.registry.List()); err != nil {
		_ = s.registry.Upsert(deleted)
		return Response{Error: "could not persist route deletion"}
	}
	warning := ""
	if err := s.secrets.DeleteTarget(context.Background(), deleted.TargetSecretRef); err != nil && !errors.Is(err, secrets.ErrNotFound) {
		warning = "route deleted, but protected target cleanup needs attention"
	}
	return Response{OK: true, Warning: warning, Running: true, Routes: summaries(s.registry.List())}
}

func summaries(all []routes.HTTPRoute) []RouteSummary {
	result := make([]RouteSummary, 0, len(all))
	for _, route := range all {
		result = append(result, summarize(route))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func summarize(route routes.HTTPRoute) RouteSummary {
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
	return RouteSummary{
		ID: route.Alias, Name: name, Alias: route.Alias, Kind: "HTTP", Status: status,
		LocalEndpoint: "127.0.0.1:4768/r/" + route.Alias, PermissionSummary: "GET, HEAD · Range",
		Egress: egress, Health: "unknown", LastUsed: "从未", CurrentConnections: 0,
	}
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
