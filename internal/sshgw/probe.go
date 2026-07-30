package sshgw

import (
	"context"
	"errors"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

var errHostKeyCaptured = errors.New("SSH host key captured")

func ProbeHostKey(ctx context.Context, dialer EgressDialer, policy, address string) (ssh.PublicKey, error) {
	raw, err := dialer.DialContext(ctx, policy, "tcp", address)
	if err != nil {
		return nil, ErrUpstreamUnavailable
	}
	defer raw.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(deadline)
	}
	algorithms := ssh.SupportedAlgorithms()
	var captured ssh.PublicKey
	config := &ssh.ClientConfig{
		Config: ssh.Config{
			KeyExchanges: algorithms.KeyExchanges,
			Ciphers:      algorithms.Ciphers,
			MACs:         algorithms.MACs,
		},
		User: "airlock-host-key-probe",
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			captured = key
			return errHostKeyCaptured
		},
		HostKeyAlgorithms: algorithms.HostKeys,
		ClientVersion:     "SSH-2.0-Airlock-Probe",
		Timeout:           10 * time.Second,
	}
	_, _, _, _ = ssh.NewClientConn(raw, address, config)
	if captured == nil {
		return nil, ErrUpstreamUnavailable
	}
	return captured, nil
}
