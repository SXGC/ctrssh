package workspace_test

import (
	"strings"
	"testing"

	"github.com/SXGC/ctrssh/internal/workspace"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		ws      workspace.Workspace
		wantErr string
	}{
		{
			name: "valid",
			ws:   workspace.Workspace{Name: "work", SSHHost: "me@host", Container: "ctr", RemoteUser: "vscode"},
		},
		{
			name: "valid empty SSHHost (test short-circuit)",
			ws:   workspace.Workspace{Name: "work", Container: "ctr", RemoteUser: "root"},
		},
		{
			name:    "empty name",
			ws:      workspace.Workspace{SSHHost: "me@host", Container: "ctr", RemoteUser: "vscode"},
			wantErr: "name is required",
		},
		{
			name:    "name with space",
			ws:      workspace.Workspace{Name: "my work", Container: "ctr", RemoteUser: "vscode"},
			wantErr: "name must match",
		},
		{
			name:    "name with slash",
			ws:      workspace.Workspace{Name: "a/b", Container: "ctr", RemoteUser: "vscode"},
			wantErr: "name must match",
		},
		{
			name:    "empty container",
			ws:      workspace.Workspace{Name: "work", SSHHost: "me@host", RemoteUser: "vscode"},
			wantErr: "container is required",
		},
		{
			name:    "empty remote user",
			ws:      workspace.Workspace{Name: "work", SSHHost: "me@host", Container: "ctr"},
			wantErr: "remote user is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ws.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}
