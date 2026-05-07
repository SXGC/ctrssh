package remote_test

import (
	"os"
	"strings"
	"testing"

	"github.com/SXGC/ctrssh/internal/remote"
	"github.com/SXGC/ctrssh/internal/workspace"
)

func TestBuildConnectArgsGolden(t *testing.T) {
	ws := workspace.Workspace{
		Name:       "work",
		SSHHost:    "me@host.example.com",
		Container:  "devbox",
		RemoteUser: "vscode",
	}
	got := remote.BuildConnectArgs(ws, "/home/u/.config/ctrssh/id_ctrssh")
	want, err := os.ReadFile("../../testdata/connect_args.golden")
	if err != nil {
		t.Fatal(err)
	}
	wantLines := strings.Split(strings.TrimSpace(string(want)), "\n")
	if len(got) != len(wantLines) {
		t.Fatalf("len got=%d want=%d\ngot=%v", len(got), len(wantLines), got)
	}
	for i := range got {
		if got[i] != wantLines[i] {
			t.Fatalf("arg[%d] got=%q want=%q", i, got[i], wantLines[i])
		}
	}
}

func TestBuildConnectArgsLocalShortCircuit(t *testing.T) {
	ws := workspace.Workspace{
		Name:       "work",
		SSHHost:    "",
		Container:  "devbox",
		RemoteUser: "root",
	}
	got := remote.BuildConnectArgs(ws, "")
	if got[0] != "docker" {
		t.Fatalf("expected docker as first arg in short-circuit mode, got %v", got)
	}
}
