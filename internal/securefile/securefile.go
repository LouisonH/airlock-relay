// Package securefile provides small, fail-closed helpers for operator-managed
// token and route specification files.
package securefile

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LouisonH/airlock-relay/internal/securefs"
)

const maxTokenBytes = 128

func Read(path string, maxBytes int64) ([]byte, error) {
	if path == "" || maxBytes < 1 {
		return nil, errors.New("invalid protected file path")
	}
	info, err := os.Lstat(path)
	if err != nil || !securefs.IsPrivateRegularFile(info) || info.Size() > maxBytes {
		return nil, errors.New("protected file must be a regular 0600 file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("open protected file")
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !os.SameFile(info, openedInfo) {
		return nil, errors.New("protected file changed while opening")
	}
	contents, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil || int64(len(contents)) > maxBytes {
		clear(contents)
		return nil, errors.New("read protected file")
	}
	return contents, nil
}

func ReadToken(path string) (string, error) {
	contents, err := Read(path, maxTokenBytes)
	if err != nil {
		return "", err
	}
	defer clear(contents)
	token := strings.TrimSpace(string(contents))
	if len(token) < 32 || len(token) > maxTokenBytes || strings.ContainsAny(token, "\x00\r\n\t ") {
		return "", errors.New("invalid protected token")
	}
	return token, nil
}

func CreateToken(path string) (string, error) {
	if path == "" || !filepath.IsAbs(path) {
		return "", errors.New("token output path must be absolute")
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", errors.New("generate token")
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	clear(random)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", errors.New("create protected token file")
	}
	if err := securefs.PreparePrivateFile(file); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", errors.New("protect token file")
	}
	if _, err := file.WriteString(token + "\n"); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", errors.New("write protected token file")
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", errors.New("sync protected token file")
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", errors.New("close protected token file")
	}
	return token, nil
}
