//go:build !windows

package control

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
)

func DefaultPaths() (Paths, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, errors.New("locate user configuration directory")
	}
	return PathsForDirectory(filepath.Join(configDir, "io.airlock.relay"))
}

func PathsForDirectory(directory string) (Paths, error) {
	if !filepath.IsAbs(directory) {
		return Paths{}, errors.New("control directory must be absolute")
	}
	return Paths{Directory: directory, Socket: filepath.Join(directory, "control.sock")}, nil
}

func validControlPaths(paths Paths) bool {
	return paths.Directory != "" && paths.Socket != "" && filepath.IsAbs(paths.Directory) && filepath.IsAbs(paths.Socket) && filepath.Dir(paths.Socket) == paths.Directory
}

func validControlEndpoint(endpoint string) bool {
	return filepath.IsAbs(endpoint)
}

func listenControlEndpoint(paths Paths) (net.Listener, error) {
	if err := prepareDirectory(paths.Directory); err != nil {
		return nil, err
	}
	if err := removeStaleSocket(paths.Socket); err != nil {
		return nil, err
	}
	listener, err := net.Listen("unix", paths.Socket)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(paths.Socket, 0o600); err != nil {
		_ = listener.Close()
		return nil, errors.New("protect control socket")
	}
	return listener, nil
}

func dialControlEndpoint(ctx context.Context, endpoint string) (net.Conn, error) {
	dialer := net.Dialer{Timeout: controlExchangeTimeout}
	return dialer.DialContext(ctx, "unix", endpoint)
}

// Cleanup removes a Unix domain socket left after normal daemon shutdown.
func Cleanup(paths Paths) {
	_ = os.Remove(paths.Socket)
}
