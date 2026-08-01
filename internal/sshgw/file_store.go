package sshgw

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/LouisonH/airlock-relay/internal/capability"
)

const (
	sshMetadataVersion  = 1
	maxSSHMetadataBytes = 1 << 20
)

type MetadataStore interface {
	Load() ([]Route, error)
	Save([]Route) error
}

type FileStore struct{ path string }

type metadataDocument struct {
	Version int              `json:"version"`
	Routes  []persistedRoute `json:"routes"`
}

type persistedRoute struct {
	Name                         string   `json:"name"`
	Alias                        string   `json:"alias"`
	LocalUsername                string   `json:"local_username,omitempty"`
	TargetSecretRef              string   `json:"target_secret_ref"`
	CapabilityDigest             string   `json:"capability_digest"`
	AllowedCommands              []string `json:"allowed_commands"`
	LocalPublicKeyFingerprints   []string `json:"local_public_key_fingerprints,omitempty"`
	AllowStdin                   bool     `json:"allow_stdin"`
	AllowAllCommands             bool     `json:"allow_all_commands,omitempty"`
	RecordCommands               bool     `json:"record_commands,omitempty"`
	AllowSFTP                    bool     `json:"allow_sftp,omitempty"`
	Egress                       string   `json:"egress"`
	AuthenticationTimeoutSeconds int      `json:"authentication_timeout_seconds,omitempty"`
	Enabled                      bool     `json:"enabled"`
}

func NewFileStore(path string) *FileStore { return &FileStore{path: path} }

func (s *FileStore) Load() ([]Route, error) {
	if err := secureMetadataDirectory(filepath.Dir(s.path)); err != nil {
		return nil, err
	}
	info, err := os.Lstat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 || info.Size() > maxSSHMetadataBytes {
		return nil, errors.New("invalid SSH route metadata file")
	}
	file, err := os.Open(s.path)
	if err != nil {
		return nil, errors.New("open SSH route metadata")
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !os.SameFile(info, openedInfo) {
		return nil, errors.New("SSH route metadata changed while opening")
	}
	decoder := json.NewDecoder(io.LimitReader(file, maxSSHMetadataBytes+1))
	decoder.DisallowUnknownFields()
	var document metadataDocument
	if err := decoder.Decode(&document); err != nil || document.Version != sshMetadataVersion {
		return nil, errors.New("decode SSH route metadata")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("SSH route metadata has trailing data")
	}

	routes := make([]Route, 0, len(document.Routes))
	seen := make(map[string]struct{}, len(document.Routes))
	seenUsernames := make(map[string]struct{}, len(document.Routes))
	for _, stored := range document.Routes {
		if _, exists := seen[stored.Alias]; exists || stored.TargetSecretRef != "ssh/"+stored.Alias {
			return nil, errors.New("invalid SSH route metadata entry")
		}
		digestBytes, err := hex.DecodeString(stored.CapabilityDigest)
		if err != nil || len(digestBytes) != len(capability.Digest{}) {
			return nil, errors.New("invalid SSH capability digest")
		}
		var digest capability.Digest
		copy(digest[:], digestBytes)
		clear(digestBytes)
		policy := NewPolicyWithOptions(
			stored.AllowedCommands,
			stored.LocalPublicKeyFingerprints,
			stored.AllowStdin,
			stored.AllowAllCommands,
			stored.RecordCommands,
		)
		policy.AllowSFTP = stored.AllowSFTP
		route := Route{
			Name: stored.Name, Alias: stored.Alias, LocalUsername: stored.LocalUsername,
			TargetSecretRef:  stored.TargetSecretRef,
			CapabilityDigest: digest,
			Policy:           policy,
			Egress:           stored.Egress, AuthenticationTimeoutSeconds: stored.AuthenticationTimeoutSeconds, Enabled: stored.Enabled,
		}
		if route.AuthenticationTimeoutSeconds == 0 {
			route.AuthenticationTimeoutSeconds = DefaultAuthenticationTimeoutSeconds
		}
		if err := route.Validate(); err != nil {
			return nil, errors.New("invalid persisted SSH route")
		}
		if _, exists := seenUsernames[route.EffectiveLocalUsername()]; exists {
			return nil, errors.New("duplicate SSH local username")
		}
		seen[route.Alias] = struct{}{}
		seenUsernames[route.EffectiveLocalUsername()] = struct{}{}
		routes = append(routes, route)
	}
	return routes, nil
}

func (s *FileStore) Save(routes []Route) error {
	if err := secureMetadataDirectory(filepath.Dir(s.path)); err != nil {
		return err
	}
	document := metadataDocument{Version: sshMetadataVersion, Routes: make([]persistedRoute, 0, len(routes))}
	seenUsernames := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		if err := route.Validate(); err != nil || route.TargetSecretRef != "ssh/"+route.Alias {
			return errors.New("refuse to persist invalid SSH route")
		}
		if _, exists := seenUsernames[route.EffectiveLocalUsername()]; exists {
			return errors.New("refuse to persist duplicate SSH local username")
		}
		seenUsernames[route.EffectiveLocalUsername()] = struct{}{}
		document.Routes = append(document.Routes, persistedRoute{
			Name: route.Name, Alias: route.Alias, LocalUsername: route.EffectiveLocalUsername(),
			TargetSecretRef:            route.TargetSecretRef,
			CapabilityDigest:           hex.EncodeToString(route.CapabilityDigest[:]),
			AllowedCommands:            sortedSet(route.Policy.AllowedCommands),
			LocalPublicKeyFingerprints: sortedSet(route.Policy.LocalPublicKeyFingerprints),
			AllowStdin:                 route.Policy.AllowStdin, Egress: route.Egress, Enabled: route.Enabled,
			AllowAllCommands:             route.Policy.AllowAllCommands,
			RecordCommands:               route.Policy.RecordCommands,
			AllowSFTP:                    route.Policy.AllowSFTP,
			AuthenticationTimeoutSeconds: route.EffectiveAuthenticationTimeoutSeconds(),
		})
	}
	sort.Slice(document.Routes, func(i, j int) bool { return document.Routes[i].Alias < document.Routes[j].Alias })

	temporary, err := os.CreateTemp(filepath.Dir(s.path), ".ssh-routes-*.tmp")
	if err != nil {
		return errors.New("create SSH route metadata")
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return errors.New("protect SSH route metadata")
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		temporary.Close()
		return errors.New("encode SSH route metadata")
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return errors.New("sync SSH route metadata")
	}
	if err := temporary.Close(); err != nil {
		return errors.New("close SSH route metadata")
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return errors.New("install SSH route metadata")
	}
	directory, err := os.Open(filepath.Dir(s.path))
	if err != nil {
		return errors.New("open SSH route metadata directory")
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return errors.New("sync SSH route metadata directory")
	}
	return nil
}

func secureMetadataDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return errors.New("create SSH route metadata directory")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("invalid SSH route metadata directory")
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return errors.New("protect SSH route metadata directory")
	}
	return nil
}

func sortedSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
