package remote

import (
	"fmt"
	"path/filepath"

	"github.com/SXGC/ctrssh/internal/workspace"
)

// BuildConnectArgs returns argv for the ProxyCommand chain.
// The chain pipes stdio through: local ssh → remote docker exec → container's sshd -i.
// When ws.SSHHost is empty (test-only short-circuit), the outer ssh hop is omitted
// and docker runs locally.
//
// When controlDir is non-empty (and ws.SSHHost is set), ssh ControlMaster
// options are added so the underlying SSH transport can be reused across
// concurrent sessions (VSCode Remote-SSH, JetBrains Gateway, and similar
// clients open many channels in quick succession).
func BuildConnectArgs(ws workspace.Workspace, identityPath, controlDir string) []string {
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
	args := []string{
		"ssh", "-T",
		"-i", identityPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
	}
	if controlDir != "" {
		// %C expands inside ssh to a hash of (local user, host, port, remote user),
		// giving a fixed-length filename safe against the AF_UNIX path limit.
		// ControlPersist keeps the master alive briefly after the last channel
		// closes so the next session attaches instead of re-handshaking.
		args = append(args,
			"-o", "ControlMaster=auto",
			"-o", "ControlPath="+filepath.Join(controlDir, "cm-%C"),
			"-o", "ControlPersist=600",
		)
	}
	args = append(args, ws.SSHHost, remoteCmd)
	return args
}
