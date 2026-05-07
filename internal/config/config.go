package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cby/ctrssh/internal/workspace"
	"github.com/gofrs/flock"
	"gopkg.in/yaml.v3"
)

// Store manages the workspaces.yaml registry under a base directory.
type Store struct {
	dir string
}

func NewStore(dir string) *Store { return &Store{dir: dir} }

func (s *Store) Path() string { return filepath.Join(s.dir, "workspaces.yaml") }

func (s *Store) lockPath() string { return s.Path() + ".lock" }

type fileShape struct {
	Workspaces []workspace.Workspace `yaml:"workspaces"`
}

func (s *Store) Load() ([]workspace.Workspace, error) {
	b, err := os.ReadFile(s.Path())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", s.Path(), err)
	}
	var f fileShape
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.Path(), err)
	}
	return f.Workspaces, nil
}

func (s *Store) save(list []workspace.Workspace) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(fileShape{Workspaces: list})
	if err != nil {
		return err
	}
	tmp := s.Path() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path())
}

func (s *Store) withLock(fn func() error) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	lk := flock.New(s.lockPath())
	ctx, locked := tryLock(lk, 5*time.Second)
	if !locked {
		return fmt.Errorf("could not acquire lock on %s within 5s", s.lockPath())
	}
	defer lk.Unlock()
	_ = ctx
	return fn()
}

func tryLock(lk *flock.Flock, timeout time.Duration) (struct{}, bool) {
	deadline := time.Now().Add(timeout)
	for {
		ok, err := lk.TryLock()
		if err == nil && ok {
			return struct{}{}, true
		}
		if time.Now().After(deadline) {
			return struct{}{}, false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (s *Store) Add(ws workspace.Workspace) error {
	if err := ws.Validate(); err != nil {
		return err
	}
	return s.withLock(func() error {
		list, err := s.Load()
		if err != nil {
			return err
		}
		for _, w := range list {
			if w.Name == ws.Name {
				return fmt.Errorf("workspace %q already exists", ws.Name)
			}
		}
		list = append(list, ws)
		return s.save(list)
	})
}

func (s *Store) Remove(name string) error {
	return s.withLock(func() error {
		list, err := s.Load()
		if err != nil {
			return err
		}
		out := make([]workspace.Workspace, 0, len(list))
		found := false
		for _, w := range list {
			if w.Name == name {
				found = true
				continue
			}
			out = append(out, w)
		}
		if !found {
			return fmt.Errorf("workspace %q not found", name)
		}
		return s.save(out)
	})
}

func (s *Store) Get(name string) (workspace.Workspace, error) {
	list, err := s.Load()
	if err != nil {
		return workspace.Workspace{}, err
	}
	for _, w := range list {
		if w.Name == name {
			return w, nil
		}
	}
	return workspace.Workspace{}, fmt.Errorf("workspace %q not found", name)
}
