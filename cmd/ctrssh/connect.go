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
