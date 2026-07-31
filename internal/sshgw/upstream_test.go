package sshgw

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/LouisonH/airlock-relay/internal/egress"
	"github.com/LouisonH/airlock-relay/internal/secrets"
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

func TestVerifyUpstreamAuthenticationSupportsSharedTargetWithoutExecutingCommands(t *testing.T) {
	upstream := startUpstream(t, upstreamAuth{username: "shared-upstream", password: "shared-password"})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	for _, route := range []Route{
		{Alias: "first", LocalUsername: "alpha", Egress: egress.Direct},
		{Alias: "second", LocalUsername: "beta", Egress: egress.Direct},
	} {
		target := secrets.SSHTarget{
			Address:         upstream.address(),
			Username:        "shared-upstream",
			Password:        []byte("shared-password"),
			ExpectedHostKey: upstream.hostSigner.PublicKey().Marshal(),
		}
		if err := VerifyUpstreamAuthentication(ctx, egress.NewManager(nil), route, &target); err != nil {
			t.Fatalf("verify %s = %v", route.LocalUsername, err)
		}
		for _, secret := range [][]byte{target.Password, target.ExpectedHostKey} {
			for _, byteValue := range secret {
				if byteValue != 0 {
					t.Fatal("verification did not clear the temporary protected target")
				}
			}
		}
	}

	state := upstream.snapshot()
	if state.passwordAuths != 2 || state.commands != 0 {
		t.Fatalf("authentication should use the shared target twice without exec: %+v", state)
	}
}

func TestVerifyUpstreamAuthenticationClassifiesRejectedPassword(t *testing.T) {
	upstream := startUpstream(t, upstreamAuth{username: "shared-upstream", password: "correct-password"})
	target := secrets.SSHTarget{
		Address:         upstream.address(),
		Username:        "shared-upstream",
		Password:        []byte("wrong-password"),
		ExpectedHostKey: upstream.hostSigner.PublicKey().Marshal(),
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	err := VerifyUpstreamAuthentication(ctx, egress.NewManager(nil), Route{Alias: "test", Egress: egress.Direct}, &target)
	if !errors.Is(err, ErrUpstreamAuth) {
		t.Fatalf("rejected password error = %v, want ErrUpstreamAuth", err)
	}
}

func TestVerifyUpstreamAuthenticationSupportsKeyboardInteractivePassword(t *testing.T) {
	upstream := startUpstream(t, upstreamAuth{username: "interactive-upstream", password: "interactive-password", keyboardInteractiveOnly: true})
	target := secrets.SSHTarget{
		Address:         upstream.address(),
		Username:        "interactive-upstream",
		Password:        []byte("interactive-password"),
		ExpectedHostKey: upstream.hostSigner.PublicKey().Marshal(),
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	if err := VerifyUpstreamAuthentication(ctx, egress.NewManager(nil), Route{Alias: "test", Egress: egress.Direct}, &target); err != nil {
		t.Fatalf("keyboard-interactive authentication = %v", err)
	}
	state := upstream.snapshot()
	if state.passwordAuths != 1 || state.keyboardInteractiveAuths != 1 {
		t.Fatalf("authentication methods = %+v", state)
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
