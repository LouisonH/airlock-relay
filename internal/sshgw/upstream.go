package sshgw

import (
	"context"
	"crypto/subtle"
	"errors"
	"net"
	"time"

	"github.com/LouisonH/airlock-relay/internal/secrets"
	"golang.org/x/crypto/ssh"
)

var (
	ErrHostKeyMismatch     = errors.New("upstream host key mismatch")
	ErrUpstreamAuth        = errors.New("upstream authentication is invalid")
	ErrUpstreamUnavailable = errors.New("upstream SSH service unavailable")
)

type EgressDialer interface {
	DialContext(ctx context.Context, policy, network, address string) (net.Conn, error)
}

func pinnedHostKeyCallback(expected []byte) (ssh.HostKeyCallback, error) {
	if _, err := ssh.ParsePublicKey(expected); err != nil {
		return nil, ErrHostKeyMismatch
	}
	pinned := append([]byte(nil), expected...)
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		actual := key.Marshal()
		if subtle.ConstantTimeCompare(actual, pinned) != 1 {
			return ErrHostKeyMismatch
		}
		return nil
	}, nil
}

func upstreamAuthMethod(target secrets.SSHTarget) (ssh.AuthMethod, error) {
	hasPassword := len(target.Password) > 0
	hasPrivateKey := len(target.PrivateKey) > 0
	if hasPassword == hasPrivateKey {
		return nil, ErrUpstreamAuth
	}
	if hasPassword {
		return ssh.Password(string(target.Password)), nil
	}
	var (
		signer ssh.Signer
		err    error
	)
	if len(target.PrivateKeyPassword) > 0 {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(target.PrivateKey, target.PrivateKeyPassword)
	} else {
		signer, err = ssh.ParsePrivateKey(target.PrivateKey)
	}
	if err != nil {
		return nil, ErrUpstreamAuth
	}
	return ssh.PublicKeys(signer), nil
}

func dialUpstream(ctx context.Context, dialer EgressDialer, route Route, target secrets.SSHTarget) (*ssh.Client, error) {
	callback, err := pinnedHostKeyCallback(target.ExpectedHostKey)
	if err != nil {
		return nil, ErrUpstreamUnavailable
	}
	auth, err := upstreamAuthMethod(target)
	if err != nil {
		return nil, ErrUpstreamUnavailable
	}
	raw, err := dialer.DialContext(ctx, route.Egress, "tcp", target.Address)
	if err != nil {
		return nil, ErrUpstreamUnavailable
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(deadline)
	}

	algorithms := ssh.SupportedAlgorithms()
	config := &ssh.ClientConfig{
		Config: ssh.Config{
			KeyExchanges: algorithms.KeyExchanges,
			Ciphers:      algorithms.Ciphers,
			MACs:         algorithms.MACs,
		},
		User:              target.Username,
		Auth:              []ssh.AuthMethod{auth},
		HostKeyCallback:   callback,
		HostKeyAlgorithms: algorithms.HostKeys,
		ClientVersion:     "SSH-2.0-Airlock",
		Timeout:           15 * time.Second,
	}
	connection, channels, requests, err := ssh.NewClientConn(raw, target.Address, config)
	if err != nil {
		_ = raw.Close()
		return nil, ErrUpstreamUnavailable
	}
	_ = raw.SetDeadline(time.Time{})
	return ssh.NewClient(connection, channels, requests), nil
}

func clearTarget(target *secrets.SSHTarget) {
	clear(target.Password)
	clear(target.PrivateKey)
	clear(target.PrivateKeyPassword)
	clear(target.ExpectedHostKey)
}
