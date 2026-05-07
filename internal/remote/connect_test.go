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
	got := remote.BuildConnectArgs(ws, "/home/u/.config/ctrssh/id_ctrssh", "/home/u/.config/ctrssh/cm")
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
	got := remote.BuildConnectArgs(ws, "", "/tmp/cm")
	if got[0] != "docker" {
		t.Fatalf("expected docker as first arg in short-circuit mode, got %v", got)
	}
	for _, a := range got {
		if strings.HasPrefix(a, "ControlMaster") || strings.HasPrefix(a, "ControlPath") {
			t.Fatalf("local short-circuit must not carry ssh ControlMaster opts, got %v", got)
		}
	}
}

// When controlDir is empty, BuildConnectArgs must not emit any ControlMaster /
// ControlPath / ControlPersist options — callers opt in by supplying a dir.
func TestBuildConnectArgsNoControlMasterWhenDirEmpty(t *testing.T) {
	ws := workspace.Workspace{
		Name:       "work",
		SSHHost:    "me@host",
		Container:  "devbox",
		RemoteUser: "vscode",
	}
	got := remote.BuildConnectArgs(ws, "/k", "")
	for _, a := range got {
		if strings.HasPrefix(a, "ControlMaster=") ||
			strings.HasPrefix(a, "ControlPath=") ||
			strings.HasPrefix(a, "ControlPersist=") {
			t.Fatalf("empty controlDir must not produce ssh control opts, got %v", got)
		}
	}
}

// When controlDir is non-empty and SSHHost is set, the three control opts
// must appear in order so a remote ssh master is created and reused.
func TestBuildConnectArgsIncludesControlMaster(t *testing.T) {
	ws := workspace.Workspace{
		Name:       "work",
		SSHHost:    "me@host",
		Container:  "devbox",
		RemoteUser: "vscode",
	}
	got := remote.BuildConnectArgs(ws, "/k", "/var/run/ctrssh-cm")
	joined := strings.Join(got, "\x00")
	for _, want := range []string{
		"ControlMaster=auto",
		"ControlPath=/var/run/ctrssh-cm/cm-%C",
		"ControlPersist=600",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in args: %v", want, got)
		}
	}
}
