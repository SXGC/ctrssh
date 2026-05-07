package remote

import (
	"fmt"

	"github.com/SXGC/ctrssh/internal/workspace"
)

// BuildConnectArgs returns argv for the ProxyCommand chain.
// The chain pipes stdio through: local ssh → remote docker exec → container's sshd -i.
// When ws.SSHHost is empty (test-only short-circuit), the outer ssh hop is omitted
// and docker runs locally.
func BuildConnectArgs(ws workspace.Workspace, identityPath string) []string {
	// /run is tmpfs in most containers, so /run/sshd disappears on restart
	// and sshd refuses to start. Re-create it on every connect.
	innerSh := "mkdir -p /run/sshd && exec /usr/sbin/sshd -i -e -f /etc/ssh/sshd_config_ctrssh"

	if ws.SSHHost == "" {
		// Local short-circuit: argv goes straight to docker via execve, no shell.
		return []string{
			"docker", "exec", "-i", "-u", "root", ws.Container,
			"sh", "-c", innerSh,
		}
	}

	// Going through ssh: the remote sshd joins our post-host argv with spaces
	// and feeds the result to the user's login shell, which would parse the
	// inner && itself unless we ship the whole docker-exec invocation as one
	// already-quoted string.
	remoteCmd := fmt.Sprintf(
		"docker exec -i -u root %s sh -c %s",
		shellSingleQuote(ws.Container),
		shellSingleQuote(innerSh),
	)
	return []string{
		"ssh", "-T",
		"-i", identityPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		ws.SSHHost,
		remoteCmd,
	}
}
