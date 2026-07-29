package capability

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
)

const tokenBytes = 32

var ErrInvalidToken = errors.New("invalid capability token")

type Digest [sha256.Size]byte

func Generate() (string, Digest, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", Digest{}, err
	}

	token := "airlock_" + base64.RawURLEncoding.EncodeToString(raw)
	return token, Hash(token), nil
}

func Hash(token string) Digest {
	return sha256.Sum256([]byte(token))
}

func Verify(token string, expected Digest) error {
	if token == "" {
		return ErrInvalidToken
	}

	actual := Hash(token)
	if subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
		return ErrInvalidToken
	}
	return nil
}
