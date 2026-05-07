package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cby/ctrssh/internal/config"
	"github.com/cby/ctrssh/internal/sshconfig"
	"github.com/cby/ctrssh/internal/workspace"
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
