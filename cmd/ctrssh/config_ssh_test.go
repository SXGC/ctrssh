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
