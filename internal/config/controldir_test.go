package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SXGC/ctrssh/internal/config"
)

func TestControlDirPath(t *testing.T) {
	dir := t.TempDir()
	s := config.NewStore(dir)
	want := filepath.Join(dir, "cm")
	if got := s.ControlDir(); got != want {
		t.Fatalf("ControlDir() = %q, want %q", got, want)
	}
}

func TestEnsureControlDirCreatesWith0700(t *testing.T) {
	dir := t.TempDir()
	s := config.NewStore(dir)
	got, err := s.EnsureControlDir()
	if err != nil {
		t.Fatalf("EnsureControlDir: %v", err)
	}
	want := filepath.Join(dir, "cm")
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	fi, err := os.Stat(got)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.IsDir() {
		t.Fatal("expected directory")
	}
	if perm := fi.Mode().Perm(); perm != 0o700 {
		t.Fatalf("perm = %#o, want 0700", perm)
	}
}

func TestEnsureControlDirIdempotent(t *testing.T) {
	dir := t.TempDir()
	s := config.NewStore(dir)
	if _, err := s.EnsureControlDir(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnsureControlDir(); err != nil {
		t.Fatalf("second call should succeed, got %v", err)
	}
}
