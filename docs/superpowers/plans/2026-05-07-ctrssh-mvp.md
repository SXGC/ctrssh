# ctrssh MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI (`ctrssh`) that exposes a remote container as a regular SSH endpoint by chaining local-ssh + docker-exec + container's `sshd -i` over a stdio pipe, so users can `ssh work.dev` / open in VSCode Remote-SSH / JetBrains Gateway.

**Architecture:** Single static binary, cobra-based CLI. Local YAML registry at `~/.config/ctrssh/workspaces.yaml`. Connection chain: `local ssh client → ProxyCommand → ctrssh connect --stdio → ssh user@host → docker exec -i ctr sshd -i`. No daemons, no exposed ports, no custom SSH server. Reuses container's `openssh-server` package via `sshd -i` inetd mode.

**Tech Stack:** Go 1.22+, `github.com/spf13/cobra`, `gopkg.in/yaml.v3`, `github.com/gofrs/flock`, `golang.org/x/crypto/ssh`, stdlib `crypto/ed25519`, `os/exec`, `os/signal`. E2E tests use real Docker via `os/exec`.

**Spec:** `docs/superpowers/specs/2026-05-07-ctrssh-mvp-design.md`

---

## File Structure (created across tasks)

```
ctrssh/
├── go.mod, go.sum                              # Task 1
├── .gitignore, README.md                       # Task 1
├── cmd/ctrssh/
│   ├── main.go                                 # Task 1 (root cobra)
│   ├── add.go                                  # Task 11
│   ├── list_rm.go                              # Task 12
│   ├── config_ssh.go                           # Task 13
│   ├── connect.go                              # Task 14
│   ├── prepare.go                              # Task 15
│   └── doctor.go                               # Task 16
├── internal/
│   ├── workspace/
│   │   ├── workspace.go                        # Task 2
│   │   └── workspace_test.go                   # Task 2
│   ├── config/
│   │   ├── config.go, config_test.go           # Task 3
│   │   └── keypair.go, keypair_test.go         # Task 4
│   ├── sshconfig/
│   │   ├── sshconfig.go                        # Task 5
│   │   └── sshconfig_test.go                   # Task 5
│   ├── remote/
│   │   ├── connect.go, connect_test.go         # Task 6
│   │   ├── prepare.go, prepare_test.go         # Task 7
│   │   └── doctor.go, doctor_test.go           # Task 8
│   ├── connect/
│   │   ├── bridge.go                           # Task 9
│   │   └── bridge_test.go                      # Task 9
│   └── prepare/
│       ├── runner.go                           # Task 10
│       └── runner_test.go                      # Task 10
├── testdata/                                   # Task 6, 7
│   ├── connect_args.golden
│   └── prepare_script.golden
├── e2e/
│   ├── prepare_connect_test.go                 # Task 17
│   └── doctor_test.go                          # Task 18
├── scripts/smoke.sh                            # Task 19
└── .github/workflows/ci.yml                    # Task 19
```

Each `internal/<x>` package has one job, communicates only via plain structs. No shared global state.

---

### Task 1: Project skeleton

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `README.md`
- Create: `cmd/ctrssh/main.go`

- [ ] **Step 1: Initialize Go module**

Run: `cd /workspaces/dev_docker/ctrssh && go mod init github.com/SXGC/ctrssh`
Expected: writes `go.mod` with `module github.com/SXGC/ctrssh` and current Go version.

- [ ] **Step 2: Add cobra dependency**

Run: `go get github.com/spf13/cobra@latest`
Expected: updates `go.mod` and `go.sum`.

- [ ] **Step 3: Create `.gitignore`**

```
# binaries
/ctrssh
/dist/

# IDE
/.idea/
/.vscode/

# Go
/coverage.out
```

- [ ] **Step 4: Create stub `README.md`**

```markdown
# ctrssh

Expose a remote container as a regular SSH endpoint.

```bash
ctrssh add work --host me@server --container devbox --user vscode
ctrssh prepare work
ctrssh config-ssh
ssh work.dev
```

See `docs/superpowers/specs/` for design.
```

- [ ] **Step 5: Create `cmd/ctrssh/main.go` with cobra root**

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var verbose bool

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "ctrssh",
		Short:         "Expose a remote container as a regular SSH endpoint",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 6: Verify build**

Run: `go build ./... && ./ctrssh --help`
Expected: prints usage starting with `Expose a remote container as a regular SSH endpoint`.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum .gitignore README.md cmd/
git commit -m "chore: initial project skeleton with cobra root command"
```

---

### Task 2: Workspace type and validation

**Files:**
- Create: `internal/workspace/workspace.go`
- Test: `internal/workspace/workspace_test.go`

- [ ] **Step 1: Write the failing test**

`internal/workspace/workspace_test.go`:

```go
package workspace_test

import (
	"strings"
	"testing"

	"github.com/SXGC/ctrssh/internal/workspace"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		ws      workspace.Workspace
		wantErr string
	}{
		{
			name: "valid",
			ws:   workspace.Workspace{Name: "work", SSHHost: "me@host", Container: "ctr", RemoteUser: "vscode"},
		},
		{
			name: "valid empty SSHHost (test short-circuit)",
			ws:   workspace.Workspace{Name: "work", Container: "ctr", RemoteUser: "root"},
		},
		{
			name:    "empty name",
			ws:      workspace.Workspace{SSHHost: "me@host", Container: "ctr", RemoteUser: "vscode"},
			wantErr: "name is required",
		},
		{
			name:    "name with space",
			ws:      workspace.Workspace{Name: "my work", Container: "ctr", RemoteUser: "vscode"},
			wantErr: "name must match",
		},
		{
			name:    "name with slash",
			ws:      workspace.Workspace{Name: "a/b", Container: "ctr", RemoteUser: "vscode"},
			wantErr: "name must match",
		},
		{
			name:    "empty container",
			ws:      workspace.Workspace{Name: "work", SSHHost: "me@host", RemoteUser: "vscode"},
			wantErr: "container is required",
		},
		{
			name:    "empty remote user",
			ws:      workspace.Workspace{Name: "work", SSHHost: "me@host", Container: "ctr"},
			wantErr: "remote user is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ws.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}
```

- [ ] **Step 2: Run test, expect FAIL**

Run: `go test ./internal/workspace/...`
Expected: FAIL with `package github.com/SXGC/ctrssh/internal/workspace is not in std`.

- [ ] **Step 3: Implement workspace**

`internal/workspace/workspace.go`:

```go
package workspace

import (
	"fmt"
	"regexp"
)

// Workspace identifies a target container reachable via an SSH host.
//
// SSHHost is in user@host form. Empty SSHHost is a test-only short-circuit
// that bypasses the outer ssh hop and runs docker locally.
// RemoteUser is the in-container user we authenticate as via sshd.
type Workspace struct {
	Name       string `yaml:"name"`
	SSHHost    string `yaml:"ssh_host,omitempty"`
	Container  string `yaml:"container"`
	RemoteUser string `yaml:"remote_user"`
}

var nameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func (w Workspace) Validate() error {
	if w.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !nameRe.MatchString(w.Name) {
		return fmt.Errorf("name must match %s", nameRe.String())
	}
	if w.Container == "" {
		return fmt.Errorf("container is required")
	}
	if w.RemoteUser == "" {
		return fmt.Errorf("remote user is required")
	}
	return nil
}
```

- [ ] **Step 4: Run test, expect PASS**

Run: `go test ./internal/workspace/...`
Expected: `PASS` with all subtests passing.

- [ ] **Step 5: Commit**

```bash
git add internal/workspace/
git commit -m "feat: add Workspace type with validation"
```

---

### Task 3: Config — YAML load/save with flock

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Add dependencies**

Run: `go get gopkg.in/yaml.v3 github.com/gofrs/flock`
Expected: `go.mod` updated.

- [ ] **Step 2: Write the failing test**

`internal/config/config_test.go`:

```go
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
```

- [ ] **Step 3: Run test, expect FAIL**

Run: `go test ./internal/config/...`
Expected: FAIL with package not found.

- [ ] **Step 4: Implement config store**

`internal/config/config.go`:

```go
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/SXGC/ctrssh/internal/workspace"
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
```

- [ ] **Step 5: Run test, expect PASS**

Run: `go test ./internal/config/...`
Expected: all subtests PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/config/
git commit -m "feat: add config store with yaml persistence and flock"
```

---

### Task 4: Config — keypair generation

**Files:**
- Create: `internal/config/keypair.go`
- Test: `internal/config/keypair_test.go`

- [ ] **Step 1: Write the failing test**

`internal/config/keypair_test.go`:

```go
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
```

- [ ] **Step 2: Run test, expect FAIL**

Run: `go test ./internal/config/...`
Expected: FAIL with `s.EnsureKeypair undefined`.

- [ ] **Step 3: Add ssh dep**

Run: `go get golang.org/x/crypto/ssh`
Expected: `go.mod` updated.

- [ ] **Step 4: Implement keypair**

`internal/config/keypair.go`:

```go
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
```

- [ ] **Step 5: Run test, expect PASS**

Run: `go test ./internal/config/...`
Expected: all subtests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config/keypair.go internal/config/keypair_test.go go.mod go.sum
git commit -m "feat: add ed25519 keypair generation under config dir"
```

---

### Task 5: SSH config marker block management

**Files:**
- Create: `internal/sshconfig/sshconfig.go`
- Test: `internal/sshconfig/sshconfig_test.go`

- [ ] **Step 1: Write the failing test**

`internal/sshconfig/sshconfig_test.go`:

```go
package sshconfig_test

import (
	"strings"
	"testing"

	"github.com/SXGC/ctrssh/internal/sshconfig"
)

func TestUpsertOnEmpty(t *testing.T) {
	got := sshconfig.Upsert("", "work", "Host work.dev\n  User vscode\n")
	want := "# ctrssh start work\nHost work.dev\n  User vscode\n# ctrssh end work\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUpsertReplacesExisting(t *testing.T) {
	in := "Host other\n  User x\n# ctrssh start work\nHost old\n# ctrssh end work\nHost after\n"
	got := sshconfig.Upsert(in, "work", "Host work.dev\n  User new\n")
	if !strings.Contains(got, "Host other\n") {
		t.Fatal("user content above the block was lost")
	}
	if !strings.Contains(got, "Host after\n") {
		t.Fatal("user content below the block was lost")
	}
	if strings.Contains(got, "Host old") {
		t.Fatal("old block content was not replaced")
	}
	if !strings.Contains(got, "Host work.dev\n  User new\n") {
		t.Fatal("new block content not present")
	}
}

func TestRemoveStripsBlockOnly(t *testing.T) {
	in := "Host other\n# ctrssh start work\nHost work.dev\n# ctrssh end work\nHost after\n"
	got := sshconfig.Remove(in, "work")
	if strings.Contains(got, "ctrssh start work") || strings.Contains(got, "Host work.dev") {
		t.Fatal("block was not fully removed")
	}
	if !strings.Contains(got, "Host other\n") || !strings.Contains(got, "Host after\n") {
		t.Fatal("user content outside the block was modified")
	}
}

func TestUpsertIdempotent(t *testing.T) {
	once := sshconfig.Upsert("", "work", "Host work.dev\n")
	twice := sshconfig.Upsert(once, "work", "Host work.dev\n")
	if once != twice {
		t.Fatalf("not idempotent: \n%s\n!=\n%s", once, twice)
	}
}

func TestRemoveAbsent(t *testing.T) {
	in := "Host other\n"
	got := sshconfig.Remove(in, "work")
	if got != in {
		t.Fatalf("Remove changed content when block was absent: %q", got)
	}
}
```

- [ ] **Step 2: Run test, expect FAIL**

Run: `go test ./internal/sshconfig/...`
Expected: package not found.

- [ ] **Step 3: Implement marker block manager**

`internal/sshconfig/sshconfig.go`:

```go
// Package sshconfig manages marker-delimited blocks within an OpenSSH client
// config file. Blocks are bounded by lines:
//   # ctrssh start <name>
//   ...
//   # ctrssh end <name>
// Content outside markers is preserved verbatim. All operations are pure
// string transformations; callers handle file IO.
package sshconfig

import (
	"strings"
)

const (
	StartPrefix = "# ctrssh start "
	EndPrefix   = "# ctrssh end "
)

// Upsert returns config with the named block replaced (or appended if absent).
// The supplied body should NOT include the marker lines; they are added.
func Upsert(config, name, body string) string {
	stripped, hadBlock := stripBlock(config, name)
	block := StartPrefix + name + "\n" + ensureTrailingNL(body) + EndPrefix + name + "\n"
	if hadBlock {
		// Re-insert at the original location (top of remaining content).
		return block + ensureSeparator(stripped)
	}
	if stripped == "" {
		return block
	}
	return block + ensureSeparator(stripped)
}

// Remove returns config with the named block stripped. Returns config unchanged
// if no such block exists.
func Remove(config, name string) string {
	out, _ := stripBlock(config, name)
	return out
}

// stripBlock removes the named marker block. Returns the new content and
// a flag indicating whether a block was found.
func stripBlock(config, name string) (string, bool) {
	start := StartPrefix + name
	end := EndPrefix + name
	lines := strings.Split(config, "\n")
	out := make([]string, 0, len(lines))
	inBlock := false
	found := false
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		switch {
		case !inBlock && trimmed == start:
			inBlock = true
			found = true
		case inBlock && trimmed == end:
			inBlock = false
		case !inBlock:
			out = append(out, ln)
		}
	}
	return strings.Join(out, "\n"), found
}

func ensureTrailingNL(s string) string {
	if s == "" || strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

func ensureSeparator(s string) string {
	if s == "" {
		return s
	}
	if strings.HasPrefix(s, "\n") {
		return s
	}
	return s
}
```

- [ ] **Step 4: Run test, expect PASS**

Run: `go test ./internal/sshconfig/...`
Expected: all subtests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sshconfig/
git commit -m "feat: add ssh config marker block management"
```

---

### Task 6: Remote — BuildConnectArgs (golden test)

**Files:**
- Create: `internal/remote/connect.go`
- Test: `internal/remote/connect_test.go`
- Create: `testdata/connect_args.golden`

- [ ] **Step 1: Create golden file**

`testdata/connect_args.golden`:

```
ssh
-T
-i
/home/u/.config/ctrssh/id_ctrssh
-o
StrictHostKeyChecking=no
-o
UserKnownHostsFile=/dev/null
me@host.example.com
docker
exec
-i
-u
root
devbox
/usr/sbin/sshd
-i
-e
-f
/etc/ssh/sshd_config_ctrssh
```

- [ ] **Step 2: Write the failing test**

`internal/remote/connect_test.go`:

```go
package remote_test

import (
	"os"
	"strings"
	"testing"

	"github.com/SXGC/ctrssh/internal/remote"
	"github.com/SXGC/ctrssh/internal/workspace"
)

func TestBuildConnectArgsGolden(t *testing.T) {
	ws := workspace.Workspace{
		Name:       "work",
		SSHHost:    "me@host.example.com",
		Container:  "devbox",
		RemoteUser: "vscode",
	}
	got := remote.BuildConnectArgs(ws, "/home/u/.config/ctrssh/id_ctrssh")
	want, err := os.ReadFile("../../testdata/connect_args.golden")
	if err != nil {
		t.Fatal(err)
	}
	wantLines := strings.Split(strings.TrimSpace(string(want)), "\n")
	if len(got) != len(wantLines) {
		t.Fatalf("len got=%d want=%d\ngot=%v", len(got), len(wantLines), got)
	}
	for i := range got {
		if got[i] != wantLines[i] {
			t.Fatalf("arg[%d] got=%q want=%q", i, got[i], wantLines[i])
		}
	}
}

func TestBuildConnectArgsLocalShortCircuit(t *testing.T) {
	ws := workspace.Workspace{
		Name:       "work",
		SSHHost:    "",
		Container:  "devbox",
		RemoteUser: "root",
	}
	got := remote.BuildConnectArgs(ws, "")
	if got[0] != "docker" {
		t.Fatalf("expected docker as first arg in short-circuit mode, got %v", got)
	}
}
```

- [ ] **Step 3: Run test, expect FAIL**

Run: `go test ./internal/remote/...`
Expected: package not found.

- [ ] **Step 4: Implement BuildConnectArgs**

`internal/remote/connect.go`:

```go
package remote

import "github.com/SXGC/ctrssh/internal/workspace"

// BuildConnectArgs returns argv for the ProxyCommand chain.
// The chain pipes stdio through: local ssh → remote docker exec → container's sshd -i.
// When ws.SSHHost is empty (test-only short-circuit), the outer ssh hop is omitted
// and docker runs locally.
func BuildConnectArgs(ws workspace.Workspace, identityPath string) []string {
	dockerArgs := []string{
		"docker", "exec", "-i", "-u", "root", ws.Container,
		"/usr/sbin/sshd", "-i", "-e", "-f", "/etc/ssh/sshd_config_ctrssh",
	}
	if ws.SSHHost == "" {
		return dockerArgs
	}
	args := []string{
		"ssh", "-T",
		"-i", identityPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		ws.SSHHost,
	}
	return append(args, dockerArgs...)
}
```

- [ ] **Step 5: Run test, expect PASS**

Run: `go test ./internal/remote/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/remote/connect.go internal/remote/connect_test.go testdata/
git commit -m "feat: add BuildConnectArgs with golden test"
```

---

### Task 7: Remote — BuildPrepareScript (golden test)

**Files:**
- Create: `internal/remote/prepare.go`
- Test: `internal/remote/prepare_test.go`
- Create: `testdata/prepare_script.golden`

- [ ] **Step 1: Implement BuildPrepareScript**

`internal/remote/prepare.go`:

```go
package remote

import (
	"fmt"
	"strings"

	"github.com/SXGC/ctrssh/internal/workspace"
)

// BuildPrepareScript returns a bash script that, when run inside the target
// container as root, idempotently installs openssh-server, ensures host keys,
// writes /etc/ssh/sshd_config_ctrssh, and appends pubkey to the remote user's
// authorized_keys. The final stdout line is "OK" on success.
func BuildPrepareScript(ws workspace.Workspace, pubkey []byte) string {
	pub := strings.TrimSpace(string(pubkey))
	// Single-quote heredoc body to suppress shell expansion of $vars in the
	// script. Variables we *want* expanded are in $...$ regions outside the body.
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

REMOTE_USER=%q
PUBKEY=%q

if ! command -v sshd >/dev/null 2>&1; then
  if command -v apt-get >/dev/null 2>&1; then
    DEBIAN_FRONTEND=noninteractive apt-get update -qq
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq openssh-server >/dev/null
  elif command -v apk >/dev/null 2>&1; then
    apk add --no-cache openssh-server >/dev/null
  elif command -v yum >/dev/null 2>&1; then
    yum install -y -q openssh-server >/dev/null
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y -q openssh-server >/dev/null
  else
    echo "no supported package manager found" >&2
    exit 1
  fi
fi

if [ ! -f /etc/ssh/ssh_host_ed25519_key ]; then
  ssh-keygen -A >/dev/null
fi

HOMEDIR="$(getent passwd "$REMOTE_USER" | cut -d: -f6)"
if [ -z "$HOMEDIR" ]; then
  echo "user $REMOTE_USER does not exist in container" >&2
  exit 1
fi

mkdir -p "$HOMEDIR/.ssh"
chmod 700 "$HOMEDIR/.ssh"
touch "$HOMEDIR/.ssh/authorized_keys"
if ! grep -qxF "$PUBKEY" "$HOMEDIR/.ssh/authorized_keys"; then
  echo "$PUBKEY" >> "$HOMEDIR/.ssh/authorized_keys"
fi
chmod 600 "$HOMEDIR/.ssh/authorized_keys"
chown -R "$REMOTE_USER":"$(id -gn "$REMOTE_USER")" "$HOMEDIR/.ssh"

cat >/etc/ssh/sshd_config_ctrssh <<'CFG'
PubkeyAuthentication yes
PasswordAuthentication no
UsePAM no
HostKey /etc/ssh/ssh_host_ed25519_key
Subsystem sftp internal-sftp
PermitRootLogin prohibit-password
CFG

/usr/sbin/sshd -t -f /etc/ssh/sshd_config_ctrssh
echo OK
`, ws.RemoteUser, pub)
}
```

- [ ] **Step 2: Generate the golden file by running the function once**

Write a small helper test that prints the script (run once, save output):

`internal/remote/prepare_test.go`:

```go
package remote_test

import (
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/SXGC/ctrssh/internal/remote"
	"github.com/SXGC/ctrssh/internal/workspace"
)

var update = flag.Bool("update", false, "update golden files")

const goldenPath = "../../testdata/prepare_script.golden"

func fixtureWS() workspace.Workspace {
	return workspace.Workspace{
		Name: "work", SSHHost: "me@host", Container: "devbox", RemoteUser: "vscode",
	}
}

const fixturePubkey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExampleKeyBytes000000000000 ctrssh"

func TestBuildPrepareScriptGolden(t *testing.T) {
	got := remote.BuildPrepareScript(fixtureWS(), []byte(fixturePubkey))
	if *update {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Skip("golden updated")
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("script mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestBuildPrepareScriptEndsWithOK(t *testing.T) {
	got := remote.BuildPrepareScript(fixtureWS(), []byte(fixturePubkey))
	if !strings.HasSuffix(strings.TrimSpace(got), "echo OK") {
		t.Fatalf("script must end with `echo OK` so prepare runner can detect success; got tail %q",
			got[len(got)-40:])
	}
}
```

- [ ] **Step 3: Generate golden, then verify**

Run: `go test ./internal/remote/... -run TestBuildPrepareScriptGolden -update`
Expected: `--- SKIP` for the golden test, file written.

Run: `go test ./internal/remote/...`
Expected: PASS.

- [ ] **Step 4: Manually inspect the golden**

Run: `cat testdata/prepare_script.golden`
Expected: matches the script body in `BuildPrepareScript`. Eyeball it; especially check that `REMOTE_USER="vscode"` and the pubkey string appears verbatim.

- [ ] **Step 5: Commit**

```bash
git add internal/remote/prepare.go internal/remote/prepare_test.go testdata/prepare_script.golden
git commit -m "feat: add BuildPrepareScript with golden test"
```

---

### Task 8: Remote — BuildDoctorChecks

**Files:**
- Create: `internal/remote/doctor.go`
- Test: `internal/remote/doctor_test.go`

- [ ] **Step 1: Write the failing test**

`internal/remote/doctor_test.go`:

```go
package remote_test

import (
	"strings"
	"testing"

	"github.com/SXGC/ctrssh/internal/remote"
)

func TestBuildDoctorChecksOrder(t *testing.T) {
	checks := remote.BuildDoctorChecks(fixtureWS())
	wantSequence := []string{
		"local ssh client",
		"ssh: connect to host",
		"docker: container running",
		"container: sshd present",
		"container: sshd -t passes",
		"container: authorized_keys",
	}
	if len(checks) != len(wantSequence) {
		t.Fatalf("got %d checks, want %d", len(checks), len(wantSequence))
	}
	for i, want := range wantSequence {
		if !strings.Contains(checks[i].Label, want) {
			t.Fatalf("check[%d] label = %q, want substring %q", i, checks[i].Label, want)
		}
	}
}

func TestBuildDoctorChecksHaveCommands(t *testing.T) {
	checks := remote.BuildDoctorChecks(fixtureWS())
	for i, c := range checks {
		if i == 0 {
			// local check has no Argv (it inspects PATH in-process)
			continue
		}
		if len(c.Argv) == 0 {
			t.Fatalf("check[%d] %q has no Argv", i, c.Label)
		}
	}
}
```

- [ ] **Step 2: Run test, expect FAIL**

Run: `go test ./internal/remote/...`
Expected: undefined symbols.

- [ ] **Step 3: Implement BuildDoctorChecks**

`internal/remote/doctor.go`:

```go
package remote

import "github.com/SXGC/ctrssh/internal/workspace"

// DoctorCheck describes one diagnostic step.
// Argv == nil means the check is performed in-process by the caller (e.g.,
// PATH lookup for the local ssh client).
type DoctorCheck struct {
	Label string
	Argv  []string
}

func BuildDoctorChecks(ws workspace.Workspace) []DoctorCheck {
	sshOpts := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=5"}
	prefix := func(extra ...string) []string {
		if ws.SSHHost == "" {
			return extra
		}
		out := []string{"ssh"}
		out = append(out, sshOpts...)
		out = append(out, ws.SSHHost)
		return append(out, extra...)
	}
	return []DoctorCheck{
		{Label: "local ssh client present"},
		{
			Label: "ssh: connect to host",
			Argv:  append([]string{"ssh"}, append(sshOpts, ws.SSHHost, "true")...),
		},
		{
			Label: "docker: container running",
			Argv:  prefix("docker", "inspect", "-f", "{{.State.Running}}", ws.Container),
		},
		{
			Label: "container: sshd present",
			Argv:  prefix("docker", "exec", ws.Container, "test", "-x", "/usr/sbin/sshd"),
		},
		{
			Label: "container: sshd -t passes",
			Argv:  prefix("docker", "exec", ws.Container, "/usr/sbin/sshd", "-t", "-f", "/etc/ssh/sshd_config_ctrssh"),
		},
		{
			Label: "container: authorized_keys readable",
			Argv:  prefix("docker", "exec", "-u", ws.RemoteUser, ws.Container, "test", "-r", "/home/"+ws.RemoteUser+"/.ssh/authorized_keys"),
		},
	}
}
```

- [ ] **Step 4: Run test, expect PASS**

Run: `go test ./internal/remote/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/remote/doctor.go internal/remote/doctor_test.go
git commit -m "feat: add BuildDoctorChecks for segment-by-segment diagnosis"
```

---

### Task 9: Connect bridge — stdio passthrough + signal forwarding

**Files:**
- Create: `internal/connect/bridge.go`
- Test: `internal/connect/bridge_test.go`

- [ ] **Step 1: Write the failing test**

`internal/connect/bridge_test.go`:

```go
package connect_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/SXGC/ctrssh/internal/connect"
)

// TestRunPassthrough exercises the stdio bridge using `cat` as the child:
// bytes written to stdin should come back unchanged on stdout.
func TestRunPassthrough(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	in := strings.NewReader("hello world\n")
	var out bytes.Buffer
	var stderr bytes.Buffer
	err := connect.Run(ctx, []string{"cat"}, in, &out, &stderr)
	if err != nil {
		t.Fatalf("Run: %v (stderr=%s)", err, stderr.String())
	}
	if out.String() != "hello world\n" {
		t.Fatalf("output = %q, want %q", out.String(), "hello world\n")
	}
}

func TestRunPropagatesExitCode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := connect.Run(ctx, []string{"sh", "-c", "exit 7"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	ee, ok := connect.AsExitError(err)
	if !ok {
		t.Fatalf("expected ExitError, got %v", err)
	}
	if ee.ExitCode() != 7 {
		t.Fatalf("exit code = %d, want 7", ee.ExitCode())
	}
}

func TestRunMissingBinary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := connect.Run(ctx, []string{"definitely_does_not_exist_99999"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}
```

- [ ] **Step 2: Run test, expect FAIL**

Run: `go test ./internal/connect/...`
Expected: package not found.

- [ ] **Step 3: Implement bridge**

`internal/connect/bridge.go`:

```go
// Package connect implements the ProxyCommand body: an exec wrapper that
// transparently bridges stdio between the calling SSH client and a child
// process chain. It must NEVER write to its own stdout outside of the child's
// pass-through, because the parent ssh client is mid-handshake on that pipe.
package connect

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"os/signal"
	"syscall"
)

// Run executes argv with stdin/stdout wired directly to the supplied readers
// and writers. Signals SIGINT/SIGTERM received by this process are forwarded
// to the child. Returns nil on clean exit, or an *exec.ExitError on non-zero
// child exit. Other errors indicate a setup failure (e.g. binary not found).
func Run(ctx context.Context, argv []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(argv) == 0 {
		return errors.New("argv is empty")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	sigs := make(chan signalEvent, 4)
	stop := installSignalHandler(sigs)
	defer stop()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", argv[0], err)
	}

	// Forward signals to the child until it exits.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for {
		select {
		case s := <-sigs:
			_ = cmd.Process.Signal(s.sig)
		case err := <-done:
			return err
		}
	}
}

// AsExitError reports whether err is an *exec.ExitError and returns it.
func AsExitError(err error) (*exec.ExitError, bool) {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee, true
	}
	return nil, false
}

type signalEvent struct{ sig syscall.Signal }

func installSignalHandler(out chan<- signalEvent) func() {
	c := make(chan _OSSignal, 4)
	signal.Notify(c, _signals...)
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case s := <-c:
				if sig, ok := toSyscallSignal(s); ok {
					out <- signalEvent{sig: sig}
				}
			case <-stop:
				signal.Stop(c)
				return
			}
		}
	}()
	return func() { close(stop) }
}
```

`internal/connect/bridge_unix.go` (separate file so we can keep stdlib imports clean):

```go
//go:build unix

package connect

import (
	"os"
	"syscall"
)

type _OSSignal = os.Signal

var _signals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT}

func toSyscallSignal(s os.Signal) (syscall.Signal, bool) {
	if sig, ok := s.(syscall.Signal); ok {
		return sig, true
	}
	return 0, false
}
```

- [ ] **Step 4: Run test, expect PASS**

Run: `go test ./internal/connect/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/connect/
git commit -m "feat: add stdio bridge with signal forwarding"
```

---

### Task 10: Prepare runner

**Files:**
- Create: `internal/prepare/runner.go`
- Test: `internal/prepare/runner_test.go`

- [ ] **Step 1: Write the failing test**

`internal/prepare/runner_test.go`:

```go
package prepare_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/SXGC/ctrssh/internal/prepare"
)

// The runner sends a script to a command via stdin and validates the last
// line is "OK". We exercise that contract using `bash -c "cat >/dev/null; echo $1"`
// as a stand-in shell that consumes the script and emits a controlled tail.

func TestRunSuccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var log bytes.Buffer
	err := prepare.RunCommand(ctx,
		[]string{"bash", "-c", "cat >/dev/null; echo OK"},
		"the-script",
		&log,
	)
	if err != nil {
		t.Fatalf("Run: %v\nlog:\n%s", err, log.String())
	}
}

func TestRunFailureSurfacesStderr(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var log bytes.Buffer
	err := prepare.RunCommand(ctx,
		[]string{"bash", "-c", "echo something_failed >&2; echo NOT_OK"},
		"x",
		&log,
	)
	if err == nil {
		t.Fatal("expected error when last line is not OK")
	}
	if !strings.Contains(err.Error(), "OK") {
		t.Fatalf("error should mention OK sentinel; got %q", err.Error())
	}
	if !strings.Contains(log.String(), "something_failed") {
		t.Fatalf("stderr not surfaced; log=%q", log.String())
	}
}
```

- [ ] **Step 2: Run test, expect FAIL**

Run: `go test ./internal/prepare/...`
Expected: package not found.

- [ ] **Step 3: Implement runner**

`internal/prepare/runner.go`:

```go
// Package prepare drives the one-time remote setup: it pipes a bash script
// to a command (typically `ssh user@host docker exec -i ctr bash -s`), captures
// its output, and verifies the last stdout line is "OK".
package prepare

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/SXGC/ctrssh/internal/remote"
	"github.com/SXGC/ctrssh/internal/workspace"
)

// Run prepares the workspace by sending the prepare script to the appropriate
// remote command. log receives a verbose transcript when verbose is true; on
// failure, stderr from the remote command is also written to log.
func Run(ctx context.Context, ws workspace.Workspace, pubkey []byte, log io.Writer) error {
	script := remote.BuildPrepareScript(ws, pubkey)
	argv := buildPrepareArgv(ws)
	return RunCommand(ctx, argv, script, log)
}

// RunCommand is the testable inner: pipes script to argv's stdin and asserts
// trailing "OK". Used by Run with a real ssh+docker chain.
func RunCommand(ctx context.Context, argv []string, script string, log io.Writer) error {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdin = strings.NewReader(script)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := strings.TrimRight(stdout.String(), "\n")
	lastLine := lastNonEmptyLine(out)
	if log != nil {
		if stderr.Len() > 0 {
			fmt.Fprintf(log, "remote stderr:\n%s\n", stderr.String())
		}
		if stdout.Len() > 0 {
			fmt.Fprintf(log, "remote stdout:\n%s\n", stdout.String())
		}
	}
	if err != nil {
		return fmt.Errorf("prepare command failed: %w (last stdout line: %q)", err, lastLine)
	}
	if lastLine != "OK" {
		return fmt.Errorf("prepare did not end with OK (got %q)", lastLine)
	}
	return nil
}

func lastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return strings.TrimSpace(lines[i])
		}
	}
	return ""
}

func buildPrepareArgv(ws workspace.Workspace) []string {
	docker := []string{
		"docker", "exec", "-i", "-u", "root", ws.Container, "bash", "-s",
	}
	if ws.SSHHost == "" {
		return docker
	}
	args := []string{"ssh", "-T", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", ws.SSHHost}
	return append(args, docker...)
}
```

- [ ] **Step 4: Run test, expect PASS**

Run: `go test ./internal/prepare/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/prepare/
git commit -m "feat: add prepare runner with OK sentinel detection"
```

---

### Task 11: cobra `add` command

**Files:**
- Create: `cmd/ctrssh/add.go`
- Modify: `cmd/ctrssh/main.go`

- [ ] **Step 1: Wire root command to register subcommands**

Modify `cmd/ctrssh/main.go` to add `register()` and call subcommand factories. Replace the file contents with:

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/SXGC/ctrssh/internal/config"
	"github.com/spf13/cobra"
)

var verbose bool

func defaultConfigDir() string {
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "ctrssh")
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "ctrssh")
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "ctrssh",
		Short:         "Expose a remote container as a regular SSH endpoint",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	store := config.NewStore(defaultConfigDir())
	root.AddCommand(newAddCmd(store))
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Implement `add` subcommand**

`cmd/ctrssh/add.go`:

```go
package main

import (
	"fmt"

	"github.com/SXGC/ctrssh/internal/config"
	"github.com/SXGC/ctrssh/internal/workspace"
	"github.com/spf13/cobra"
)

func newAddCmd(store *config.Store) *cobra.Command {
	var host, container, user string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Register a new workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := workspace.Workspace{
				Name:       args[0],
				SSHHost:    host,
				Container:  container,
				RemoteUser: user,
			}
			if err := store.Add(ws); err != nil {
				return err
			}
			fmt.Printf("registered workspace %q\n", ws.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&host, "host", "", "ssh host in user@host form")
	cmd.Flags().StringVar(&container, "container", "", "container name on the remote host")
	cmd.Flags().StringVar(&user, "user", "", "remote user inside the container")
	_ = cmd.MarkFlagRequired("container")
	_ = cmd.MarkFlagRequired("user")
	return cmd
}
```

- [ ] **Step 3: Build and verify command runs**

Run: `go build ./... && ./ctrssh add testws --host me@host --container ctr --user vscode`
Expected: prints `registered workspace "testws"`. A `~/.config/ctrssh/workspaces.yaml` exists with the entry.

Run: `./ctrssh add testws --host me@host --container ctr --user vscode`
Expected: error `workspace "testws" already exists`, exit code 1.

Cleanup: `rm -rf ~/.config/ctrssh`

- [ ] **Step 4: Commit**

```bash
git add cmd/ctrssh/
git commit -m "feat: add 'add' subcommand for workspace registration"
```

---

### Task 12: cobra `list` and `rm` commands

**Files:**
- Create: `cmd/ctrssh/list_rm.go`
- Modify: `cmd/ctrssh/main.go`

- [ ] **Step 1: Implement list and rm**

`cmd/ctrssh/list_rm.go`:

```go
package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/SXGC/ctrssh/internal/config"
	"github.com/spf13/cobra"
)

func newListCmd(store *config.Store) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered workspaces",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			list, err := store.Load()
			if err != nil {
				return err
			}
			if len(list) == 0 {
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tHOST\tCONTAINER\tUSER")
			for _, w := range list {
				host := w.SSHHost
				if host == "" {
					host = "(local)"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", w.Name, host, w.Container, w.RemoteUser)
			}
			return tw.Flush()
		},
	}
}

func newRmCmd(store *config.Store) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a registered workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store.Remove(args[0]); err != nil {
				return err
			}
			fmt.Printf("removed workspace %q\n", args[0])
			return nil
		},
	}
}
```

- [ ] **Step 2: Register commands in main.go**

Modify `cmd/ctrssh/main.go`. In `newRootCmd`, after `root.AddCommand(newAddCmd(store))`, add:

```go
	root.AddCommand(newListCmd(store))
	root.AddCommand(newRmCmd(store))
```

- [ ] **Step 3: Verify**

Run:
```bash
go build ./...
./ctrssh add a --host me@h --container c1 --user u1
./ctrssh add b --host me@h --container c2 --user u2
./ctrssh list
```
Expected: tabular output with two rows for `a` and `b`.

Run: `./ctrssh rm a && ./ctrssh list`
Expected: `removed workspace "a"`, then list shows only `b`.

Cleanup: `rm -rf ~/.config/ctrssh`

- [ ] **Step 4: Commit**

```bash
git add cmd/ctrssh/
git commit -m "feat: add 'list' and 'rm' subcommands"
```

---

### Task 13: cobra `config-ssh` command

**Files:**
- Create: `cmd/ctrssh/config_ssh.go`
- Modify: `cmd/ctrssh/main.go`
- Modify: `cmd/ctrssh/add.go` (auto-invoke after add)

- [ ] **Step 1: Implement config-ssh**

`cmd/ctrssh/config_ssh.go`:

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/SXGC/ctrssh/internal/config"
	"github.com/SXGC/ctrssh/internal/sshconfig"
	"github.com/SXGC/ctrssh/internal/workspace"
	"github.com/spf13/cobra"
)

var sshConfigMu sync.Mutex

func defaultSSHConfigPath() string {
	return filepath.Join(os.Getenv("HOME"), ".ssh", "config")
}

func newConfigSSHCmd(store *config.Store) *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:   "config-ssh",
		Short: "Write/refresh ~/.ssh/config entries for all workspaces",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" {
				path = defaultSSHConfigPath()
			}
			list, err := store.Load()
			if err != nil {
				return err
			}
			privPath, _, err := store.EnsureKeypair()
			if err != nil {
				return err
			}
			exec, err := os.Executable()
			if err != nil {
				return err
			}
			abs, err := filepath.Abs(exec)
			if err != nil {
				return err
			}
			return rewriteSSHConfig(path, list, abs, privPath)
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "ssh config path (default ~/.ssh/config)")
	return cmd
}

func rewriteSSHConfig(path string, list []workspace.Workspace, execPath, identityPath string) error {
	sshConfigMu.Lock()
	defer sshConfigMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	cur := ""
	if b, err := os.ReadFile(path); err == nil {
		cur = string(b)
	} else if !os.IsNotExist(err) {
		return err
	}
	// Remove all known names first to ensure deletions take effect.
	for _, w := range list {
		cur = sshconfig.Remove(cur, w.Name)
	}
	// Re-upsert each.
	for _, w := range list {
		body := renderHostBlock(w, execPath, identityPath)
		cur = sshconfig.Upsert(cur, w.Name, body)
	}
	return atomicWrite(path, []byte(cur), 0o600)
}

func renderHostBlock(w workspace.Workspace, execPath, identityPath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Host %s.dev\n", w.Name)
	b.WriteString("  ForwardAgent yes\n")
	b.WriteString("  StrictHostKeyChecking no\n")
	b.WriteString("  UserKnownHostsFile /dev/null\n")
	fmt.Fprintf(&b, "  IdentityFile %s\n", identityPath)
	b.WriteString("  IdentitiesOnly yes\n")
	fmt.Fprintf(&b, "  User %s\n", w.RemoteUser)
	fmt.Fprintf(&b, "  ProxyCommand %q connect --stdio %s\n", execPath, w.Name)
	return b.String()
}

func atomicWrite(path string, b []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
```

- [ ] **Step 2: Register command and auto-invoke after add**

Modify `cmd/ctrssh/main.go` `newRootCmd` to add:

```go
	root.AddCommand(newConfigSSHCmd(store))
```

Modify `cmd/ctrssh/add.go` `RunE`. Replace the existing RunE body's tail (after `if err := store.Add(ws); err != nil`) with:

```go
			if err := store.Add(ws); err != nil {
				return err
			}
			fmt.Printf("registered workspace %q\n", ws.Name)
			// Auto-refresh ssh config so users don't have to remember the second step.
			list, err := store.Load()
			if err != nil {
				return err
			}
			privPath, _, err := store.EnsureKeypair()
			if err != nil {
				return err
			}
			exec, err := os.Executable()
			if err != nil {
				return err
			}
			abs, err := filepath.Abs(exec)
			if err != nil {
				return err
			}
			if err := rewriteSSHConfig(defaultSSHConfigPath(), list, abs, privPath); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: ssh config refresh failed: %v\n", err)
			}
			return nil
```

Add imports `"os"` and `"path/filepath"` to `cmd/ctrssh/add.go` if not already.

- [ ] **Step 3: Verify**

Run:
```bash
go build ./...
mkdir -p /tmp/ctrssh-test/.ssh
HOME=/tmp/ctrssh-test ./ctrssh add work --host me@host --container ctr --user vscode
cat /tmp/ctrssh-test/.ssh/config
```
Expected output includes:
```
# ctrssh start work
Host work.dev
  ForwardAgent yes
  StrictHostKeyChecking no
  ...
  ProxyCommand "/...absolute/.../ctrssh" connect --stdio work
# ctrssh end work
```

Cleanup: `rm -rf /tmp/ctrssh-test`

- [ ] **Step 4: Commit**

```bash
git add cmd/ctrssh/
git commit -m "feat: add 'config-ssh' command and auto-refresh on add"
```

---

### Task 14: cobra `connect --stdio` command

**Files:**
- Create: `cmd/ctrssh/connect.go`
- Modify: `cmd/ctrssh/main.go`

- [ ] **Step 1: Implement connect**

`cmd/ctrssh/connect.go`:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/SXGC/ctrssh/internal/config"
	"github.com/SXGC/ctrssh/internal/connect"
	"github.com/SXGC/ctrssh/internal/remote"
	"github.com/spf13/cobra"
)

func newConnectCmd(store *config.Store) *cobra.Command {
	var stdio bool
	cmd := &cobra.Command{
		Use:   "connect <name>",
		Short: "Connect to a workspace (used as ProxyCommand)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdio {
				return errors.New("only --stdio mode is supported in MVP")
			}
			ws, err := store.Get(args[0])
			if err != nil {
				return err
			}
			privPath, _, err := store.EnsureKeypair()
			if err != nil {
				return err
			}
			argv := remote.BuildConnectArgs(ws, privPath)
			ctx := context.Background()
			err = connect.Run(ctx, argv, os.Stdin, os.Stdout, os.Stderr)
			if ee, ok := connect.AsExitError(err); ok {
				os.Exit(ee.ExitCode())
			}
			if err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&stdio, "stdio", false, "use stdio (required; bridges through ProxyCommand)")
	return cmd
}
```

- [ ] **Step 2: Register command**

Modify `cmd/ctrssh/main.go`:

```go
	root.AddCommand(newConnectCmd(store))
```

- [ ] **Step 3: Smoke build**

Run: `go build ./... && ./ctrssh connect --help`
Expected: help output includes `--stdio` flag.

Run: `./ctrssh connect --stdio nonexistent`
Expected: exit code 1 with `error: workspace "nonexistent" not found`.

- [ ] **Step 4: Commit**

```bash
git add cmd/ctrssh/
git commit -m "feat: add 'connect --stdio' subcommand"
```

---

### Task 15: cobra `prepare` command

**Files:**
- Create: `cmd/ctrssh/prepare.go`
- Modify: `cmd/ctrssh/main.go`

- [ ] **Step 1: Implement prepare**

`cmd/ctrssh/prepare.go`:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/SXGC/ctrssh/internal/config"
	"github.com/SXGC/ctrssh/internal/prepare"
	"github.com/spf13/cobra"
)

func newPrepareCmd(store *config.Store) *cobra.Command {
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "prepare <name>",
		Short: "Install sshd and inject pubkey in the target container (idempotent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := store.Get(args[0])
			if err != nil {
				return err
			}
			_, pub, err := store.EnsureKeypair()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			var log = os.Stderr
			if !verbose {
				log = nil
			}
			if err := prepare.Run(ctx, ws, pub, log); err != nil {
				return err
			}
			fmt.Printf("workspace %q is ready\n", ws.Name)
			return nil
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", 2*time.Minute, "remote prepare timeout")
	return cmd
}
```

- [ ] **Step 2: Register command**

Modify `cmd/ctrssh/main.go`:

```go
	root.AddCommand(newPrepareCmd(store))
```

- [ ] **Step 3: Smoke build**

Run: `go build ./... && ./ctrssh prepare --help`
Expected: help output. Trying `./ctrssh prepare nonexistent` → `workspace "nonexistent" not found`.

- [ ] **Step 4: Commit**

```bash
git add cmd/ctrssh/
git commit -m "feat: add 'prepare' subcommand driving remote setup"
```

---

### Task 16: cobra `doctor` command

**Files:**
- Create: `cmd/ctrssh/doctor.go`
- Modify: `cmd/ctrssh/main.go`

- [ ] **Step 1: Implement doctor**

`cmd/ctrssh/doctor.go`:

```go
package main

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/SXGC/ctrssh/internal/config"
	"github.com/SXGC/ctrssh/internal/remote"
	"github.com/spf13/cobra"
)

func newDoctorCmd(store *config.Store) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor <name>",
		Short: "Diagnose workspace connectivity step by step",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := store.Get(args[0])
			if err != nil {
				return err
			}
			checks := remote.BuildDoctorChecks(ws)
			out := cmd.OutOrStdout()
			for i, c := range checks {
				ok, detail := runCheck(c)
				mark := "✓"
				if !ok {
					mark = "✗"
				}
				fmt.Fprintf(out, " %s %s", mark, c.Label)
				if detail != "" {
					fmt.Fprintf(out, " — %s", detail)
				}
				fmt.Fprintln(out)
				if !ok {
					return fmt.Errorf("doctor: failed at step %d (%s)", i+1, c.Label)
				}
			}
			fmt.Fprintln(out, "all checks passed")
			return nil
		},
	}
	return cmd
}

func runCheck(c remote.DoctorCheck) (bool, string) {
	if c.Argv == nil {
		_, err := exec.LookPath("ssh")
		if err != nil {
			return false, err.Error()
		}
		return true, ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Argv[0], c.Argv[1:]...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		// Trim noisy output to one line for readability.
		return false, firstLine(string(b))
	}
	return true, ""
}

func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}
```

- [ ] **Step 2: Register command**

Modify `cmd/ctrssh/main.go`:

```go
	root.AddCommand(newDoctorCmd(store))
```

- [ ] **Step 3: Smoke build**

Run: `go build ./... && ./ctrssh doctor --help`
Expected: help output. `./ctrssh doctor nonexistent` → not-found error.

- [ ] **Step 4: Commit**

```bash
git add cmd/ctrssh/
git commit -m "feat: add 'doctor' subcommand"
```

---

### Task 17: E2E test — prepare + connect against alpine

**Files:**
- Create: `e2e/prepare_connect_test.go`

- [ ] **Step 1: Add `connect.RunFiles` helper for tests**

Modify `internal/connect/bridge.go`. Add `"os"` to imports if missing, and append at end of file:

```go
// RunFiles is Run but with *os.File stdio so exec.Cmd can dup the FDs directly
// rather than spawning copy goroutines. Used by e2e tests piping into ssh.NewClientConn.
func RunFiles(ctx context.Context, argv []string, stdin, stdout *os.File, stderr io.Writer) error {
	return Run(ctx, argv, stdin, stdout, stderr)
}
```

- [ ] **Step 2: Write the e2e test**

`e2e/prepare_connect_test.go`:

```go
//go:build e2e

package e2e_test

import (
	"context"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/SXGC/ctrssh/internal/config"
	"github.com/SXGC/ctrssh/internal/connect"
	"github.com/SXGC/ctrssh/internal/prepare"
	"github.com/SXGC/ctrssh/internal/remote"
	"github.com/SXGC/ctrssh/internal/workspace"
	"golang.org/x/crypto/ssh"
)

const (
	testImage = "alpine:3.19"
	testCtr   = "ctrssh-e2e"
)

func dockerRun(t *testing.T, name, image string) {
	t.Helper()
	_ = exec.Command("docker", "rm", "-f", name).Run()
	out, err := exec.Command("docker", "run", "-d", "--name", name, image,
		"sleep", "infinity").CombinedOutput()
	if err != nil {
		t.Skipf("docker run failed; skipping e2e: %v\n%s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })
}

func TestPrepareThenConnect(t *testing.T) {
	dockerRun(t, testCtr, testImage)

	dir := t.TempDir()
	store := config.NewStore(dir)
	_, pub, err := store.EnsureKeypair()
	if err != nil {
		t.Fatal(err)
	}
	ws := workspace.Workspace{Name: "e2e", SSHHost: "", Container: testCtr, RemoteUser: "root"}
	if err := store.Add(ws); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// 1. prepare
	if err := prepare.Run(ctx, ws, pub, os.Stderr); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	// 2. set up SSH client config from the generated private key
	priv := store.PrivateKeyPath()
	keyBytes, err := os.ReadFile(priv)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}

	// 3. wire ssh.NewClientConn ↔ connect.RunFiles via os.Pipe pairs.
	// Pipes: ssh client → inW (in this proc) → inR (child stdin)
	//        child stdout → outW → outR (read by ssh client)
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = connect.RunFiles(ctx, remote.BuildConnectArgs(ws, priv), inR, outW, os.Stderr)
		_ = outW.Close()
		_ = inR.Close()
	}()
	conn, chans, reqs, err := ssh.NewClientConn(newPipeConn(outR, inW), "ctrssh-e2e", cfg)
	if err != nil {
		t.Fatalf("ssh handshake: %v", err)
	}
	client := ssh.NewClient(conn, chans, reqs)
	defer client.Close()

	// 4. run a command
	sess, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	out, err := sess.CombinedOutput("whoami")
	if err != nil {
		t.Fatalf("whoami: %v (out=%s)", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "root" {
		t.Fatalf("whoami = %q, want root", got)
	}

	// 5. re-run prepare; should be idempotent
	if err := prepare.Run(ctx, ws, pub, os.Stderr); err != nil {
		t.Fatalf("idempotent prepare: %v", err)
	}
}

// pipeConn glues a separate reader and writer into a net.Conn so it can be
// passed to ssh.NewClientConn.
type pipeConn struct {
	r *os.File
	w *os.File
}

func newPipeConn(r, w *os.File) *pipeConn { return &pipeConn{r: r, w: w} }

func (c *pipeConn) Read(p []byte) (int, error)   { return c.r.Read(p) }
func (c *pipeConn) Write(p []byte) (int, error)  { return c.w.Write(p) }
func (c *pipeConn) Close() error                 { _ = c.r.Close(); return c.w.Close() }
func (c *pipeConn) LocalAddr() net.Addr          { return pipeAddr{} }
func (c *pipeConn) RemoteAddr() net.Addr         { return pipeAddr{} }
func (c *pipeConn) SetDeadline(time.Time) error      { return nil }
func (c *pipeConn) SetReadDeadline(time.Time) error  { return nil }
func (c *pipeConn) SetWriteDeadline(time.Time) error { return nil }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }
```

- [ ] **Step 3: Run e2e test**

Run: `go test -tags=e2e -v ./e2e/... -timeout 5m`
Expected: PASS in ~30-60 seconds. If docker isn't available, the test skips with a clear message.

- [ ] **Step 4: Commit**

```bash
git add e2e/ internal/connect/bridge.go
git commit -m "test: add e2e prepare+connect against alpine container"
```

---

### Task 18: E2E test — doctor on broken container

**Files:**
- Create: `e2e/doctor_test.go`

- [ ] **Step 1: Write the test**

`e2e/doctor_test.go`:

```go
//go:build e2e

package e2e_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/SXGC/ctrssh/internal/config"
	"github.com/SXGC/ctrssh/internal/prepare"
	"github.com/SXGC/ctrssh/internal/remote"
	"github.com/SXGC/ctrssh/internal/workspace"
)

const brokenCtr = "ctrssh-doctor-e2e"

func TestDoctorDetectsMissingSshd(t *testing.T) {
	_ = exec.Command("docker", "rm", "-f", brokenCtr).Run()
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", brokenCtr).Run() })

	if out, err := exec.Command("docker", "run", "-d", "--name", brokenCtr, "alpine:3.19",
		"sleep", "infinity").CombinedOutput(); err != nil {
		t.Skipf("docker run failed; skipping: %v\n%s", err, out)
	}

	ws := workspace.Workspace{
		Name: "broken", SSHHost: "", Container: brokenCtr, RemoteUser: "root",
	}

	checks := remote.BuildDoctorChecks(ws)

	// Find the "sshd present" check; before prepare it should fail.
	failedAtSshd := false
	for _, c := range checks {
		if c.Argv == nil {
			continue
		}
		cmd := exec.Command(c.Argv[0], c.Argv[1:]...)
		if err := cmd.Run(); err != nil {
			if strings.Contains(c.Label, "sshd present") {
				failedAtSshd = true
			}
			break
		}
	}
	if !failedAtSshd {
		t.Fatal("doctor did not detect missing sshd before prepare")
	}

	// Run prepare; checks should then pass.
	dir := t.TempDir()
	store := config.NewStore(dir)
	_, pub, err := store.EnsureKeypair()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := prepare.Run(ctx, ws, pub, os.Stderr); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	for _, c := range checks {
		if c.Argv == nil {
			continue
		}
		cmd := exec.Command(c.Argv[0], c.Argv[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("after prepare, check %q failed: %v\n%s", c.Label, err, out)
		}
	}
}
```

- [ ] **Step 2: Run e2e test**

Run: `go test -tags=e2e -v ./e2e/... -run TestDoctor -timeout 5m`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add e2e/doctor_test.go
git commit -m "test: add e2e doctor test for broken-then-prepared container"
```

---

### Task 19: smoke.sh, README, GitHub Actions CI

**Files:**
- Create: `scripts/smoke.sh`
- Modify: `README.md` (full version)
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Create smoke.sh**

`scripts/smoke.sh`:

```bash
#!/usr/bin/env bash
# Manual smoke test. Requires:
#   - REMOTE_HOST env var (ssh-reachable, has docker)
#   - a running container named SMOKE_CTR (default: smoke-test) on REMOTE_HOST
set -euo pipefail

: "${REMOTE_HOST:?REMOTE_HOST=user@host required}"
SMOKE_CTR="${SMOKE_CTR:-smoke-test}"
NAME="ctrssh-smoke-$$"

trap 'ctrssh rm "$NAME" >/dev/null 2>&1 || true' EXIT

ctrssh add "$NAME" --host "$REMOTE_HOST" --container "$SMOKE_CTR" --user root
ctrssh prepare "$NAME"
ctrssh config-ssh

ssh -o BatchMode=yes "${NAME}.dev" whoami | grep -qx root
ssh -o BatchMode=yes "${NAME}.dev" "ls /" | head -1 >/dev/null

echo "smoke OK"
```

Run: `chmod +x scripts/smoke.sh`

- [ ] **Step 2: Replace README with full version**

Overwrite `README.md`:

```markdown
# ctrssh

Expose a remote container running on an SSH-reachable host as a regular SSH endpoint, so you can `ssh work.dev` (or open in VSCode Remote-SSH / JetBrains Gateway / Cursor) without exposing any TCP port from the container.

## How it works

```
local ssh client ──▶ ProxyCommand ──▶ ctrssh connect --stdio
                                          │
                                          ▼
                              ssh user@host docker exec -i ctr sshd -i
                                          │
                                          ▼
                            container's openssh-server (inetd mode)
                                          │
                                          ▼
                                       your shell
```

No daemons, no exposed ports. The chain is pure stdio.

## Install

```bash
go install github.com/SXGC/ctrssh/cmd/ctrssh@latest
```

## Use

```bash
# one-time per workspace
ctrssh add work --host me@server --container devbox --user vscode
ctrssh prepare work     # installs sshd + injects pubkey in the container
ctrssh config-ssh       # writes ~/.ssh/config (auto-runs after `add`)

# daily
ssh work.dev
code --remote ssh-remote+work.dev /workspaces
# JetBrains Gateway: pick host work.dev
```

## Commands

| command | purpose |
|---|---|
| `add <name>` | register a workspace; auto-refresh ssh config |
| `list` | list registered workspaces |
| `rm <name>` | remove a workspace |
| `prepare <name>` | one-time remote setup (idempotent) |
| `config-ssh` | refresh `~/.ssh/config` entries |
| `connect --stdio <name>` | ProxyCommand body (not invoked manually) |
| `doctor <name>` | diagnose connectivity step-by-step |

## Requirements

- **Local**: OpenSSH client on PATH (POSIX systems and Windows 10+ have it)
- **Remote host**: docker CLI, user has docker permissions, ssh reachable
- **Container**: any Linux base; `prepare` installs `openssh-server` if missing

## Files

```
~/.config/ctrssh/
  workspaces.yaml          # registry
  id_ctrssh, id_ctrssh.pub # tool-specific keypair
~/.ssh/config              # ctrssh writes Host blocks delimited by markers
```

## Design

See `docs/superpowers/specs/2026-05-07-ctrssh-mvp-design.md`.
```

- [ ] **Step 3: Create CI workflow**

`.github/workflows/ci.yml`:

```yaml
name: ci

on:
  push:
  pull_request:

jobs:
  unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go vet ./...
      - run: go test -race ./...
      - run: go build ./...

  e2e:
    runs-on: ubuntu-latest
    needs: unit
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - name: Verify docker available
        run: docker version
      - run: go test -tags=e2e -timeout 5m -v ./e2e/...
```

- [ ] **Step 4: Verify locally**

Run: `go vet ./... && go test ./... && go build ./...`
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add scripts/ README.md .github/
git commit -m "chore: add smoke script, README, and GitHub Actions CI"
```

---

## Self-Review

**Spec coverage:** every spec section maps to one or more tasks above:
- Repo layout → Task 1
- workspace package → Task 2
- config package (yaml + keypair) → Tasks 3, 4
- sshconfig package → Task 5
- remote package (connect/prepare/doctor) → Tasks 6, 7, 8
- connect bridge → Task 9
- prepare runner → Task 10
- cmd subcommands → Tasks 11–16
- Data flow A (connect) → Tasks 6, 9, 14
- Data flow B (prepare) → Tasks 7, 10, 15
- Data flow C (add+config-ssh) → Tasks 11, 13
- Error handling (signals, exit codes, OK sentinel, doctor) → Tasks 9, 10, 16
- Testing tier 1 → all tasks
- Testing tier 2 (e2e) → Tasks 17, 18
- Testing tier 3 (smoke) → Task 19
- CI → Task 19
- Filesystem layout (`~/.config/ctrssh/...`) → Task 11

**Type consistency:** `Workspace` struct fields (`Name`, `SSHHost`, `Container`, `RemoteUser`) are used identically across tasks. `BuildConnectArgs(ws, identityPath)` signature matches caller in Task 14. `connect.Run` and `connect.RunFiles` signatures consistent. `prepare.Run` and `prepare.RunCommand` consistent.

**Placeholder scan:** None. Each step has either complete code, exact commands, or both.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-07-ctrssh-mvp.md`. Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?
