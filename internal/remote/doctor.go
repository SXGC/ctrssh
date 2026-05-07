package remote

import "github.com/SXGC/ctrssh/internal/workspace"

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
			Argv:  prefix("docker", "exec", ws.Container, "sh", "-c", "mkdir -p /run/sshd && /usr/sbin/sshd -t -f /etc/ssh/sshd_config_ctrssh"),
		},
		{
			Label: "container: authorized_keys readable",
			Argv:  prefix("docker", "exec", "-u", ws.RemoteUser, ws.Container, "test", "-r", "/home/"+ws.RemoteUser+"/.ssh/authorized_keys"),
		},
	}
}
