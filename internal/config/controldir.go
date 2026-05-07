package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// ControlDir returns the directory used for SSH ControlPath sockets.
// The directory is not guaranteed to exist; use EnsureControlDir before
// passing the path to ssh.
func (s *Store) ControlDir() string { return filepath.Join(s.dir, "cm") }

// EnsureControlDir creates the control-socket directory with 0700 perms
// (sockets there carry an authenticated SSH master) and returns its path.
func (s *Store) EnsureControlDir() (string, error) {
	d := s.ControlDir()
	if err := os.MkdirAll(d, 0o700); err != nil {
		return "", fmt.Errorf("ensure control dir: %w", err)
	}
	return d, nil
}
