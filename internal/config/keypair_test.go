package config_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/SXGC/ctrssh/internal/config"
)

func TestEnsureKeypairGenerates(t *testing.T) {
	dir := t.TempDir()
	s := config.NewStore(dir)
	priv, pub, err := s.EnsureKeypair()
	if err != nil {
		t.Fatalf("EnsureKeypair: %v", err)
	}
	if _, err := os.Stat(priv); err != nil {
		t.Fatalf("private key not written: %v", err)
	}
	st, err := os.Stat(priv)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("priv perms = %o, want 600", st.Mode().Perm())
	}
	if !strings.HasPrefix(string(pub), "ssh-ed25519 ") {
		t.Fatalf("pubkey does not look like ed25519: %q", string(pub))
	}
}

func TestEnsureKeypairIdempotent(t *testing.T) {
	dir := t.TempDir()
	s := config.NewStore(dir)
	_, pub1, err := s.EnsureKeypair()
	if err != nil {
		t.Fatal(err)
	}
	_, pub2, err := s.EnsureKeypair()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pub1, pub2) {
		t.Fatal("EnsureKeypair regenerated key on second call")
	}
}
