package config

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

func (s *Store) PrivateKeyPath() string { return filepath.Join(s.dir, "id_ctrssh") }
func (s *Store) PublicKeyPath() string  { return filepath.Join(s.dir, "id_ctrssh.pub") }

// EnsureKeypair returns the absolute private-key path and the authorized-keys
// formatted public key, generating a new ed25519 pair if the private key file
// is missing.
func (s *Store) EnsureKeypair() (string, []byte, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return "", nil, err
	}
	priv := s.PrivateKeyPath()
	pubPath := s.PublicKeyPath()
	if _, err := os.Stat(priv); errors.Is(err, os.ErrNotExist) {
		if err := generateKeypair(priv, pubPath); err != nil {
			return "", nil, err
		}
	} else if err != nil {
		return "", nil, err
	}
	pub, err := os.ReadFile(pubPath)
	if err != nil {
		return "", nil, fmt.Errorf("read pubkey: %w", err)
	}
	return priv, pub, nil
}

func generateKeypair(privPath, pubPath string) error {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	pemBlock, err := ssh.MarshalPrivateKey(privKey, "ctrssh")
	if err != nil {
		return fmt.Errorf("marshal private key: %w", err)
	}
	if err := os.WriteFile(privPath, pem.EncodeToMemory(pemBlock), 0o600); err != nil {
		return err
	}
	sshPub, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return err
	}
	return os.WriteFile(pubPath, ssh.MarshalAuthorizedKey(sshPub), 0o644)
}
