package main

import (
	"fmt"
	"os"
	"path/filepath"

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
		},
	}
	cmd.Flags().StringVar(&host, "host", "", "ssh host in user@host form")
	cmd.Flags().StringVar(&container, "container", "", "container name on the remote host")
	cmd.Flags().StringVar(&user, "user", "", "remote user inside the container")
	_ = cmd.MarkFlagRequired("container")
	_ = cmd.MarkFlagRequired("user")
	return cmd
}
