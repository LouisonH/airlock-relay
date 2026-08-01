//go:build darwin && cgo

package secrets

import (
	"errors"

	keychain "github.com/keybase/go-keychain"
)

type darwinKeychainBackend struct{}

func NewPlatformStore() (MutableStore, error) {
	return newKeychainStore(darwinKeychainBackend{}), nil
}

func (darwinKeychainBackend) Put(reference string, data []byte) error {
	item := keychain.NewGenericPassword(platformStoreService, reference, "Airlock protected target", data, "")
	item.SetSynchronizable(keychain.SynchronizableNo)
	item.SetAccessible(keychain.AccessibleAfterFirstUnlockThisDeviceOnly)
	if err := keychain.AddItem(item); !errors.Is(err, keychain.ErrorDuplicateItem) {
		return err
	}

	query := keychain.NewItem()
	query.SetSecClass(keychain.SecClassGenericPassword)
	query.SetService(platformStoreService)
	query.SetAccount(reference)
	update := keychain.NewItem()
	update.SetData(data)
	return keychain.UpdateItem(query, update)
}

func (darwinKeychainBackend) Get(reference string) ([]byte, error) {
	query := keychain.NewItem()
	query.SetSecClass(keychain.SecClassGenericPassword)
	query.SetService(platformStoreService)
	query.SetAccount(reference)
	query.SetMatchLimit(keychain.MatchLimitOne)
	query.SetReturnData(true)
	results, err := keychain.QueryItem(query)
	if err != nil {
		return nil, err
	}
	if len(results) != 1 {
		return nil, ErrNotFound
	}
	return append([]byte(nil), results[0].Data...), nil
}

func (darwinKeychainBackend) Delete(reference string) error {
	item := keychain.NewItem()
	item.SetSecClass(keychain.SecClassGenericPassword)
	item.SetService(platformStoreService)
	item.SetAccount(reference)
	if err := keychain.DeleteItem(item); err != nil {
		if errors.Is(err, keychain.ErrorItemNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}
