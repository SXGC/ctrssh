package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cby/ctrssh/internal/config"
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
	root.AddCommand(newListCmd(store))
	root.AddCommand(newRmCmd(store))
	root.AddCommand(newConfigSSHCmd(store))
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
