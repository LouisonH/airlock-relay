//go:build windows

package secrets

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	keyring "github.com/zalando/go-keyring"
)

// Windows Credential Manager limits one generic credential payload. Keep each
// protected entry below that ceiling while retaining an atomic index switch.
const windowsCredentialChunkBytes = 1200

type windowsKeyringBackend struct{}

type windowsCredentialIndex struct {
	generation string
	chunks     int
}

func NewPlatformStore() (MutableStore, error) {
	return newKeychainStore(windowsKeyringBackend{}), nil
}

func (windowsKeyringBackend) Put(reference string, data []byte) error {
	previous, _ := loadWindowsCredentialIndex(reference)
	generation, err := windowsCredentialGeneration()
	if err != nil {
		return err
	}
	chunks := (len(data) + windowsCredentialChunkBytes - 1) / windowsCredentialChunkBytes
	if chunks == 0 {
		chunks = 1
	}
	for index := 0; index < chunks; index++ {
		start := index * windowsCredentialChunkBytes
		end := start + windowsCredentialChunkBytes
		if end > len(data) {
			end = len(data)
		}
		encoded := base64.RawStdEncoding.EncodeToString(data[start:end])
		if err := keyring.Set(platformStoreService, windowsCredentialChunkName(reference, generation, index), encoded); err != nil {
			deleteWindowsCredentialChunks(reference, generation, index)
			return err
		}
	}
	indexValue := fmt.Sprintf("v1:%s:%d", generation, chunks)
	if err := keyring.Set(platformStoreService, windowsCredentialIndexName(reference), indexValue); err != nil {
		deleteWindowsCredentialChunks(reference, generation, chunks)
		return err
	}
	if previous.chunks > 0 {
		deleteWindowsCredentialChunks(reference, previous.generation, previous.chunks)
	}
	return nil
}

func (windowsKeyringBackend) Get(reference string) ([]byte, error) {
	index, err := loadWindowsCredentialIndex(reference)
	if err != nil {
		return nil, err
	}
	data := make([]byte, 0, index.chunks*windowsCredentialChunkBytes)
	for part := 0; part < index.chunks; part++ {
		encoded, err := keyring.Get(platformStoreService, windowsCredentialChunkName(reference, index.generation, part))
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		chunk, err := base64.RawStdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, errors.New("decode platform secret")
		}
		data = append(data, chunk...)
	}
	return data, nil
}

func (windowsKeyringBackend) Delete(reference string) error {
	index, err := loadWindowsCredentialIndex(reference)
	if err != nil {
		return err
	}
	deleteWindowsCredentialChunks(reference, index.generation, index.chunks)
	err = keyring.Delete(platformStoreService, windowsCredentialIndexName(reference))
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

func loadWindowsCredentialIndex(reference string) (windowsCredentialIndex, error) {
	value, err := keyring.Get(platformStoreService, windowsCredentialIndexName(reference))
	if errors.Is(err, keyring.ErrNotFound) {
		return windowsCredentialIndex{}, ErrNotFound
	}
	if err != nil {
		return windowsCredentialIndex{}, err
	}
	parts := strings.Split(value, ":")
	if len(parts) != 3 || parts[0] != "v1" || len(parts[1]) != 24 {
		return windowsCredentialIndex{}, errors.New("invalid platform secret index")
	}
	chunks, err := strconv.Atoi(parts[2])
	if err != nil || chunks < 1 || chunks > 1024 {
		return windowsCredentialIndex{}, errors.New("invalid platform secret index")
	}
	return windowsCredentialIndex{generation: parts[1], chunks: chunks}, nil
}

func windowsCredentialGeneration() (string, error) {
	bytes := make([]byte, 18)
	if _, err := rand.Read(bytes); err != nil {
		return "", errors.New("generate platform secret reference")
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func windowsCredentialIndexName(reference string) string {
	return reference + ":index"
}

func windowsCredentialChunkName(reference, generation string, index int) string {
	return reference + ":" + generation + ":" + strconv.Itoa(index)
}

func deleteWindowsCredentialChunks(reference, generation string, chunks int) {
	for index := 0; index < chunks; index++ {
		_ = keyring.Delete(platformStoreService, windowsCredentialChunkName(reference, generation, index))
	}
}
