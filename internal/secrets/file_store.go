package secrets

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const (
	fileBackendVersion  = 1
	maxFileBackendBytes = 16 << 20
)

type fileBackend struct {
	path string
	mu   sync.Mutex
}

type fileBackendDocument struct {
	Version int               `json:"version"`
	Entries map[string][]byte `json:"entries"`
}

// NewFileStore stores secrets in a user-only file without cryptographic
// protection. It is a convenience alternative to the platform credential store.
func NewFileStore(path string) (MutableStore, error) {
	backend := &fileBackend{path: path}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if _, err := backend.load(); err != nil {
		return nil, err
	}
	return newKeychainStore(backend), nil
}

func (b *fileBackend) Put(reference string, data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	document, err := b.load()
	if err != nil {
		return err
	}
	document.Entries[reference] = append([]byte(nil), data...)
	return b.save(document)
}

func (b *fileBackend) Get(reference string) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	document, err := b.load()
	if err != nil {
		return nil, err
	}
	data, ok := document.Entries[reference]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), data...), nil
}

func (b *fileBackend) Delete(reference string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	document, err := b.load()
	if err != nil {
		return err
	}
	if _, ok := document.Entries[reference]; !ok {
		return ErrNotFound
	}
	delete(document.Entries, reference)
	return b.save(document)
}

func (b *fileBackend) load() (fileBackendDocument, error) {
	if err := secureFileBackendDirectory(filepath.Dir(b.path)); err != nil {
		return fileBackendDocument{}, err
	}
	document := fileBackendDocument{Version: fileBackendVersion, Entries: make(map[string][]byte)}
	info, err := os.Lstat(b.path)
	if errors.Is(err, os.ErrNotExist) {
		return document, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 || info.Size() > maxFileBackendBytes {
		return fileBackendDocument{}, errors.New("invalid local secret file")
	}
	file, err := os.Open(b.path)
	if err != nil {
		return fileBackendDocument{}, errors.New("open local secret file")
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !os.SameFile(info, openedInfo) {
		return fileBackendDocument{}, errors.New("local secret file changed while opening")
	}
	decoder := json.NewDecoder(io.LimitReader(file, maxFileBackendBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil || document.Version != fileBackendVersion || document.Entries == nil {
		return fileBackendDocument{}, errors.New("decode local secret file")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fileBackendDocument{}, errors.New("local secret file has trailing data")
	}
	for reference, data := range document.Entries {
		if validateReference(reference) != nil || len(data) > 1<<20 {
			return fileBackendDocument{}, errors.New("invalid local secret entry")
		}
	}
	return document, nil
}

func (b *fileBackend) save(document fileBackendDocument) error {
	temporary, err := os.CreateTemp(filepath.Dir(b.path), ".protected-targets-*.tmp")
	if err != nil {
		return errors.New("create local secret file")
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return errors.New("protect local secret file")
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		temporary.Close()
		return errors.New("encode local secret file")
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return errors.New("sync local secret file")
	}
	if err := temporary.Close(); err != nil {
		return errors.New("close local secret file")
	}
	if err := os.Rename(temporaryPath, b.path); err != nil {
		return errors.New("install local secret file")
	}
	directory, err := os.Open(filepath.Dir(b.path))
	if err != nil {
		return errors.New("open local secret directory")
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return errors.New("sync local secret directory")
	}
	return nil
}

func secureFileBackendDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return errors.New("create local secret directory")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("invalid local secret directory")
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return errors.New("protect local secret directory")
	}
	return nil
}
