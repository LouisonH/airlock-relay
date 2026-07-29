package sshgw

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestPinnedHostKeyCallbackRequiresExactKey(t *testing.T) {
	expected := generateSigner(t).PublicKey()
	callback, err := pinnedHostKeyCallback(expected.Marshal())
	if err != nil {
		t.Fatal(err)
	}
	if err := callback("protected.invalid", testAddress("127.0.0.1:22"), expected); err != nil {
		t.Fatalf("expected key rejected: %v", err)
	}
	if err := callback("protected.invalid", testAddress("127.0.0.1:22"), generateSigner(t).PublicKey()); !errors.Is(err, ErrHostKeyMismatch) {
		t.Fatalf("mismatched key error = %v", err)
	}
	if _, err := pinnedHostKeyCallback([]byte("not-a-host-key")); !errors.Is(err, ErrHostKeyMismatch) {
		t.Fatalf("invalid pinned key error = %v", err)
	}
}

func generateSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

type testAddress string

func (a testAddress) Network() string { return "tcp" }
func (a testAddress) String() string  { return string(a) }

var _ net.Addr = testAddress("")
