package remote

import "github.com/SXGC/ctrssh/internal/workspace"

// BuildConnectArgs returns argv for the ProxyCommand chain.
// The chain pipes stdio through: local ssh → remote docker exec → container's sshd -i.
// When ws.SSHHost is empty (test-only short-circuit), the outer ssh hop is omitted
// and docker runs locally.
func BuildConnectArgs(ws workspace.Workspace, identityPath string) []string {
	// /run is tmpfs in most containers, so /run/sshd disappears on restart
	// and sshd refuses to start. Re-create it on every connect.
	dockerArgs := []string{
		"docker", "exec", "-i", "-u", "root", ws.Container,
		"sh", "-c", "mkdir -p /run/sshd && exec /usr/sbin/sshd -i -e -f /etc/ssh/sshd_config_ctrssh",
	}
	if ws.SSHHost == "" {
		return dockerArgs
	}
	args := []string{
		"ssh", "-T",
		"-i", identityPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		ws.SSHHost,
	}
	return append(args, dockerArgs...)
}
