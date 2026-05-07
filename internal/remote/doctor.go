package remote

import (
	"fmt"

	"github.com/SXGC/ctrssh/internal/workspace"
)

// DoctorCheck describes one diagnostic step.
// Argv == nil means the check is performed in-process by the caller (e.g.,
// PATH lookup for the local ssh client).
type DoctorCheck struct {
	Label string
	Argv  []string
}

func BuildDoctorChecks(ws workspace.Workspace) []DoctorCheck {
	sshOpts := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=5"}
	prefix := func(extra ...string) []string {
		if ws.SSHHost == "" {
			return extra
		}
		out := []string{"ssh"}
		out = append(out, sshOpts...)
		out = append(out, ws.SSHHost)
		return append(out, extra...)
	}

	// Step 5 needs a shell so it can run mkdir && sshd -t. When we go through
	// ssh, the remote shell would re-parse our argv and split on &&, so the
	// whole inner command must be packaged as a single shell-quoted string.
	step5InnerSh := "mkdir -p /run/sshd && /usr/sbin/sshd -t -f /etc/ssh/sshd_config_ctrssh"
	var step5Argv []string
	// Run as root to mirror connect's docker-exec invocation; otherwise sshd
	// can't read /etc/ssh/ssh_host_*_key (mode 0600, root-owned) and reports
	// "Unable to load host key" even when connect would succeed.
	if ws.SSHHost == "" {
		step5Argv = []string{"docker", "exec", "-u", "root", ws.Container, "sh", "-c", step5InnerSh}
	} else {
		remoteCmd := fmt.Sprintf("docker exec -u root %s sh -c %s",
			shellSingleQuote(ws.Container),
			shellSingleQuote(step5InnerSh),
		)
		step5Argv = append([]string{"ssh"}, append(sshOpts, ws.SSHHost, remoteCmd)...)
	}

	return []DoctorCheck{
		{Label: "local ssh client present"},
		{
			Label: "ssh: connect to host",
			Argv:  append([]string{"ssh"}, append(sshOpts, ws.SSHHost, "true")...),
		},
		{
			Label: "docker: container running",
			Argv:  prefix("docker", "inspect", "-f", "{{.State.Running}}", ws.Container),
		},
		{
			Label: "container: sshd present",
			Argv:  prefix("docker", "exec", ws.Container, "test", "-x", "/usr/sbin/sshd"),
		},
		{
			// /run/sshd is on tmpfs and disappears on container restart;
			// recreate it so the config test mirrors how connect runs sshd.
			Label: "container: sshd -t passes",
			Argv:  step5Argv,
		},
		{
			Label: "container: authorized_keys readable",
			Argv:  prefix("docker", "exec", "-u", ws.RemoteUser, ws.Container, "test", "-r", "/home/"+ws.RemoteUser+"/.ssh/authorized_keys"),
		},
	}
}
