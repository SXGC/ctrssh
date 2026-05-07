package remote_test

import (
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/SXGC/ctrssh/internal/remote"
	"github.com/SXGC/ctrssh/internal/workspace"
)

var update = flag.Bool("update", false, "update golden files")

const goldenPath = "../../testdata/prepare_script.golden"

func fixtureWS() workspace.Workspace {
	return workspace.Workspace{
		Name: "work", SSHHost: "me@host", Container: "devbox", RemoteUser: "vscode",
	}
}

const fixturePubkey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExampleKeyBytes000000000000 ctrssh"

func TestBuildPrepareScriptGolden(t *testing.T) {
	got := remote.BuildPrepareScript(fixtureWS(), []byte(fixturePubkey))
	if *update {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Skip("golden updated")
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("script mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestBuildPrepareScriptEndsWithOK(t *testing.T) {
	got := remote.BuildPrepareScript(fixtureWS(), []byte(fixturePubkey))
	if !strings.HasSuffix(strings.TrimSpace(got), "echo OK") {
		t.Fatalf("script must end with `echo OK` so prepare runner can detect success; got tail %q",
			got[len(got)-40:])
	}
}
