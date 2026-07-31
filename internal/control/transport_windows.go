//go:build windows

package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"

	winio "github.com/Microsoft/go-winio"
)

const windowsPipePrefix = `\\.\pipe\airlock-relay-`

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
	identity := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(directory))))
	return Paths{
		Directory: directory,
		Socket:    windowsPipePrefix + hex.EncodeToString(identity[:12]),
	}, nil
}

func validControlPaths(paths Paths) bool {
	return paths.Directory != "" && filepath.IsAbs(paths.Directory) && validControlEndpoint(paths.Socket)
}

func validControlEndpoint(endpoint string) bool {
	return strings.HasPrefix(strings.ToLower(endpoint), windowsPipePrefix)
}

func listenControlEndpoint(paths Paths) (net.Listener, error) {
	if err := prepareDirectory(paths.Directory); err != nil {
		return nil, err
	}
	// OW grants the owner full access and the protected DACL prevents inherited
	// access from making the control endpoint visible to other local users.
	return winio.ListenPipe(paths.Socket, &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;OW)",
		InputBufferSize:    int32(maxMessageBytes),
		OutputBufferSize:   int32(maxMessageBytes),
	})
}

func dialControlEndpoint(ctx context.Context, endpoint string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, endpoint)
}

// Cleanup is unnecessary for named pipes: closing the listener removes it.
func Cleanup(_ Paths) {}
