package main

import (
	"fmt"

	"github.com/cby/ctrssh/internal/config"
	"github.com/cby/ctrssh/internal/workspace"
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
