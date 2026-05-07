package main

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/cby/ctrssh/internal/config"
	"github.com/cby/ctrssh/internal/remote"
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
