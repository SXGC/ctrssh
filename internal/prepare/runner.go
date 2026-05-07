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
