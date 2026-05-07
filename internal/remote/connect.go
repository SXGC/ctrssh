package remote

import "github.com/cby/ctrssh/internal/workspace"

// BuildConnectArgs returns argv for the ProxyCommand chain.
// The chain pipes stdio through: local ssh → remote docker exec → container's sshd -i.
// When ws.SSHHost is empty (test-only short-circuit), the outer ssh hop is omitted
// and docker runs locally.
func BuildConnectArgs(ws workspace.Workspace, identityPath string) []string {
	dockerArgs := []string{
		"docker", "exec", "-i", "-u", "root", ws.Container,
		"/usr/sbin/sshd", "-i", "-e", "-f", "/etc/ssh/sshd_config_ctrssh",
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
