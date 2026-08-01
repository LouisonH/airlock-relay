//go:build linux

package secrets

import (
	"encoding/base64"
	"errors"

	keyring "github.com/zalando/go-keyring"
)

type linuxKeyringBackend struct{}

func NewPlatformStore() (MutableStore, error) {
	return newKeychainStore(linuxKeyringBackend{}), nil
}

func (linuxKeyringBackend) Put(reference string, data []byte) error {
	return keyring.Set(platformStoreService, reference, base64.RawStdEncoding.EncodeToString(data))
}

func (linuxKeyringBackend) Get(reference string) ([]byte, error) {
	encoded, err := keyring.Get(platformStoreService, reference)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	data, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("decode platform secret")
	}
	return data, nil
}

func (linuxKeyringBackend) Delete(reference string) error {
	err := keyring.Delete(platformStoreService, reference)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
