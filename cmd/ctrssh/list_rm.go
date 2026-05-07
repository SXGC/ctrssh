package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/cby/ctrssh/internal/config"
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
