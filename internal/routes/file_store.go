package routes

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/LouisonH/airlock-relay/internal/capability"
	"github.com/LouisonH/airlock-relay/internal/securefs"
)

const (
	metadataVersion  = 2
	maxMetadataBytes = 1 << 20
)

type MetadataStore interface {
	Load() ([]HTTPRoute, error)
	Save([]HTTPRoute) error
}

type FileStore struct {
	path string
}

type metadataDocument struct {
	Version int              `json:"version"`
	Routes  []persistedRoute `json:"routes"`
}

type persistedRoute struct {
	Name              string   `json:"name"`
	Alias             string   `json:"alias"`
	Kind              string   `json:"kind,omitempty"`
	Provider          string   `json:"provider,omitempty"`
	TargetSecretRef   string   `json:"target_secret_ref"`
	CapabilityDigest  string   `json:"capability_digest"`
	AllowedMethods    []string `json:"allowed_methods"`
	AllowedQueryKeys  []string `json:"allowed_query_keys,omitempty"`
	AllowedPaths      []string `json:"allowed_paths,omitempty"`
	AllowedModels     []string `json:"allowed_models,omitempty"`
	MaxRequestBytes   int64    `json:"max_request_bytes,omitempty"`
	MaxOutputTokens   int      `json:"max_output_tokens,omitempty"`
	RequestsPerMinute int      `json:"requests_per_minute,omitempty"`
	MaxConcurrent     int      `json:"max_concurrent,omitempty"`
	TrackUsage        bool     `json:"track_usage,omitempty"`
	Egress            string   `json:"egress"`
	Enabled           bool     `json:"enabled"`
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func (s *FileStore) Load() ([]HTTPRoute, error) {
	if err := secureDirectory(filepath.Dir(s.path)); err != nil {
		return nil, err
	}
	info, err := os.Lstat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil || !securefs.IsPrivateRegularFile(info) || info.Size() > maxMetadataBytes {
		return nil, errors.New("invalid route metadata file")
	}
	file, err := os.Open(s.path)
	if err != nil {
		return nil, errors.New("open route metadata")
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !os.SameFile(info, openedInfo) {
		return nil, errors.New("route metadata changed while opening")
	}

	decoder := json.NewDecoder(io.LimitReader(file, maxMetadataBytes+1))
	decoder.DisallowUnknownFields()
	var document metadataDocument
	if err := decoder.Decode(&document); err != nil || (document.Version != 1 && document.Version != metadataVersion) {
		return nil, errors.New("decode route metadata")
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	routes := make([]HTTPRoute, 0, len(document.Routes))
	seen := make(map[string]struct{}, len(document.Routes))
	for _, stored := range document.Routes {
		if _, exists := seen[stored.Alias]; exists || stored.TargetSecretRef != "routes/"+stored.Alias {
			return nil, errors.New("invalid route metadata entry")
		}
		digestBytes, err := hex.DecodeString(stored.CapabilityDigest)
		if err != nil || len(digestBytes) != len(capability.Digest{}) {
			return nil, errors.New("invalid capability digest")
		}
		var digest capability.Digest
		copy(digest[:], digestBytes)
		clear(digestBytes)
		policy := NewHTTPPolicy(stored.AllowedMethods, stored.AllowedQueryKeys)
		policy.AllowedPaths = sliceSet(stored.AllowedPaths)
		policy.AllowedModels = sliceSet(stored.AllowedModels)
		policy.MaxRequestBytes = stored.MaxRequestBytes
		policy.MaxOutputTokens = stored.MaxOutputTokens
		policy.RequestsPerMinute = stored.RequestsPerMinute
		policy.MaxConcurrent = stored.MaxConcurrent
		policy.TrackUsage = stored.TrackUsage
		kind := stored.Kind
		if document.Version == 1 {
			kind = KindHTTP
		}
		route := HTTPRoute{
			Name: stored.Name, Alias: stored.Alias, Kind: kind, Provider: stored.Provider,
			TargetSecretRef: stored.TargetSecretRef, CapabilityDigest: digest, Policy: policy,
			Egress: stored.Egress, Enabled: stored.Enabled,
		}
		if err := route.Validate(); err != nil {
			return nil, errors.New("invalid persisted route")
		}
		seen[stored.Alias] = struct{}{}
		routes = append(routes, route)
	}
	return routes, nil
}

func (s *FileStore) Save(routes []HTTPRoute) error {
	if err := secureDirectory(filepath.Dir(s.path)); err != nil {
		return err
	}
	document := metadataDocument{Version: metadataVersion, Routes: make([]persistedRoute, 0, len(routes))}
	for _, route := range routes {
		if err := route.Validate(); err != nil || route.TargetSecretRef != "routes/"+route.Alias {
			return errors.New("refuse to persist invalid route")
		}
		methods := sortedKeys(route.Policy.AllowedMethods)
		queries := sortedKeys(route.Policy.AllowedQueryKeys)
		paths := sortedKeys(route.Policy.AllowedPaths)
		models := sortedKeys(route.Policy.AllowedModels)
		document.Routes = append(document.Routes, persistedRoute{
			Name: route.Name, Alias: route.Alias, Kind: route.EffectiveKind(), Provider: route.Provider,
			TargetSecretRef:  route.TargetSecretRef,
			CapabilityDigest: hex.EncodeToString(route.CapabilityDigest[:]), AllowedMethods: methods,
			AllowedQueryKeys: queries, AllowedPaths: paths, AllowedModels: models,
			MaxRequestBytes: route.Policy.MaxRequestBytes, MaxOutputTokens: route.Policy.MaxOutputTokens,
			RequestsPerMinute: route.Policy.RequestsPerMinute, MaxConcurrent: route.Policy.MaxConcurrent,
			TrackUsage: route.Policy.TrackUsage,
			Egress:     route.Egress, Enabled: route.Enabled,
		})
	}
	sort.Slice(document.Routes, func(i, j int) bool { return document.Routes[i].Alias < document.Routes[j].Alias })

	temporary, err := os.CreateTemp(filepath.Dir(s.path), ".routes-*.tmp")
	if err != nil {
		return errors.New("create route metadata")
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := securefs.PreparePrivateFile(temporary); err != nil {
		temporary.Close()
		return errors.New("protect route metadata")
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		temporary.Close()
		return errors.New("encode route metadata")
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return errors.New("sync route metadata")
	}
	if err := temporary.Close(); err != nil {
		return errors.New("close route metadata")
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return errors.New("install route metadata")
	}
	if err := securefs.SyncDirectory(filepath.Dir(s.path)); err != nil {
		return errors.New("sync route metadata directory")
	}
	return nil
}

func sliceSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func secureDirectory(path string) error {
	if err := securefs.EnsurePrivateDirectory(path); err != nil {
		return errors.New("create or protect route metadata directory")
	}
	return nil
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	return keys
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("route metadata has trailing data")
	}
	return nil
}
