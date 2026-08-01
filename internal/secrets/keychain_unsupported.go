//go:build (!darwin || !cgo) && !windows && !linux

package secrets

func NewPlatformStore() (MutableStore, error) {
	return nil, ErrUnsupported
}
