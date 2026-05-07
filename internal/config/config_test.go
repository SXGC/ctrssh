package config_test

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/SXGC/ctrssh/internal/config"
	"github.com/SXGC/ctrssh/internal/workspace"
)

func tempStore(t *testing.T) *config.Store {
	t.Helper()
	dir := t.TempDir()
	return config.NewStore(dir)
}

func TestLoadEmpty(t *testing.T) {
	s := tempStore(t)
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d entries", len(got))
	}
}

func TestAddGetRemove(t *testing.T) {
	s := tempStore(t)
	ws := workspace.Workspace{Name: "work", SSHHost: "me@host", Container: "ctr", RemoteUser: "vscode"}
	if err := s.Add(ws); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got, err := s.Get("work")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Container != "ctr" {
		t.Fatalf("expected ctr, got %q", got.Container)
	}
	if err := s.Add(ws); err == nil {
		t.Fatal("expected duplicate-add error, got nil")
	}
	if err := s.Remove("work"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := s.Get("work"); err == nil {
		t.Fatal("expected not-found after Remove, got nil")
	}
}

func TestPersistAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	s1 := config.NewStore(dir)
	if err := s1.Add(workspace.Workspace{Name: "a", Container: "c", RemoteUser: "u"}); err != nil {
		t.Fatal(err)
	}
	s2 := config.NewStore(dir)
	got, err := s2.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "a" {
		t.Fatalf("expected one workspace 'a', got %+v", got)
	}
}

func TestConcurrentAddIsSerialized(t *testing.T) {
	s := tempStore(t)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = s.Add(workspace.Workspace{
				Name:       string(rune('a' + n)),
				Container:  "c",
				RemoteUser: "u",
			})
		}(i)
	}
	wg.Wait()
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(got))
	}
}

func TestPath(t *testing.T) {
	dir := t.TempDir()
	s := config.NewStore(dir)
	want := filepath.Join(dir, "workspaces.yaml")
	if s.Path() != want {
		t.Fatalf("Path() = %q, want %q", s.Path(), want)
	}
}
