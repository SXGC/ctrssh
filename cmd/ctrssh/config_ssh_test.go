package main

import (
	"strings"
	"testing"

	"github.com/SXGC/ctrssh/internal/workspace"
)

func TestRenderHostBlockQuotesPaths(t *testing.T) {
	ws := workspace.Workspace{Name: "work", SSHHost: "me@host", Container: "ctr", RemoteUser: "vscode"}
	got := renderHostBlock(ws, "/Users/some user/go/bin/ctrssh", "/Users/some user/.config/ctrssh/id_ctrssh")

	wantIdentity := `  IdentityFile "/Users/some user/.config/ctrssh/id_ctrssh"`
	if !strings.Contains(got, wantIdentity) {
		t.Errorf("IdentityFile not quoted. block:\n%s", got)
	}

	wantProxy := `  ProxyCommand "/Users/some user/go/bin/ctrssh" connect --stdio work`
	if !strings.Contains(got, wantProxy) {
		t.Errorf("ProxyCommand not quoted. block:\n%s", got)
	}
}

// Inner SSH compression cuts bytes-on-wire for the local-client ↔ container-sshd
// session. Critical on upload-bound links where the docker-exec stdio path is
// not the real bottleneck.
func TestRenderHostBlockEnablesCompression(t *testing.T) {
	ws := workspace.Workspace{Name: "work", SSHHost: "me@host", Container: "ctr", RemoteUser: "vscode"}
	got := renderHostBlock(ws, "/usr/local/bin/ctrssh", "/home/u/.config/ctrssh/id_ctrssh")
	if !strings.Contains(got, "Compression yes") {
		t.Errorf("expected `Compression yes` in host block, got:\n%s", got)
	}
}
