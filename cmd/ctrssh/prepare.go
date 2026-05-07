package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cby/ctrssh/internal/config"
	"github.com/cby/ctrssh/internal/prepare"
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
