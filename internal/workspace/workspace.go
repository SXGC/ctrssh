package workspace

import (
	"fmt"
	"regexp"
)

// Workspace identifies a target container reachable via an SSH host.
//
// SSHHost is in user@host form. Empty SSHHost is a test-only short-circuit
// that bypasses the outer ssh hop and runs docker locally.
// RemoteUser is the in-container user we authenticate as via sshd.
type Workspace struct {
	Name       string `yaml:"name"`
	SSHHost    string `yaml:"ssh_host,omitempty"`
	Container  string `yaml:"container"`
	RemoteUser string `yaml:"remote_user"`
}

var nameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func (w Workspace) Validate() error {
	if w.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !nameRe.MatchString(w.Name) {
		return fmt.Errorf("name must match %s", nameRe.String())
	}
	if w.Container == "" {
		return fmt.Errorf("container is required")
	}
	if w.RemoteUser == "" {
		return fmt.Errorf("remote user is required")
	}
	return nil
}
