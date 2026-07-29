//go:build !darwin || !cgo

package secrets

func NewPlatformStore() (MutableStore, error) {
	return nil, ErrUnsupported
}
