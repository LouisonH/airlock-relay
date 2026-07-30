package sshgw

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"

	"github.com/LouisonH/airlock-relay/internal/secrets"
	"golang.org/x/crypto/ssh"
)

const HostIdentitySecretReference = "ssh/host-identity"

func LoadOrCreateHostSigner(ctx context.Context, store secrets.MutableStore) (ssh.Signer, error) {
	identity, err := store.ResolveSSHHostIdentity(ctx, HostIdentitySecretReference)
	if err == nil {
		defer clear(identity.PrivateKey)
		signer, parseErr := ssh.ParsePrivateKey(identity.PrivateKey)
		if parseErr != nil {
			return nil, errors.New("load protected SSH host identity")
		}
		return signer, nil
	}
	if !errors.Is(err, secrets.ErrNotFound) {
		return nil, errors.New("load protected SSH host identity")
	}

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, errors.New("generate SSH host identity")
	}
	block, err := ssh.MarshalPrivateKey(privateKey, "Airlock local SSH host")
	clear(privateKey)
	if err != nil {
		return nil, errors.New("encode SSH host identity")
	}
	encoded := pem.EncodeToMemory(block)
	clear(block.Bytes)
	if len(encoded) == 0 {
		return nil, errors.New("encode SSH host identity")
	}
	defer clear(encoded)
	if err := store.PutSSHHostIdentity(ctx, HostIdentitySecretReference, secrets.SSHHostIdentity{PrivateKey: encoded}); err != nil {
		return nil, errors.New("store protected SSH host identity")
	}
	signer, err := ssh.ParsePrivateKey(encoded)
	if err != nil {
		return nil, errors.New("initialize SSH host identity")
	}
	return signer, nil
}
