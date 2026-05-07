//go:build e2e

package e2e_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/cby/ctrssh/internal/config"
	"github.com/cby/ctrssh/internal/prepare"
	"github.com/cby/ctrssh/internal/remote"
	"github.com/cby/ctrssh/internal/workspace"
)

const brokenCtr = "ctrssh-doctor-e2e"

func TestDoctorDetectsMissingSshd(t *testing.T) {
	_ = exec.Command("docker", "rm", "-f", brokenCtr).Run()
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", brokenCtr).Run() })

	if out, err := exec.Command("docker", "run", "-d", "--name", brokenCtr, "alpine:3.19",
		"sleep", "infinity").CombinedOutput(); err != nil {
		t.Skipf("docker run failed; skipping: %v\n%s", err, out)
	}

	ws := workspace.Workspace{
		Name: "broken", SSHHost: "", Container: brokenCtr, RemoteUser: "root",
	}

	checks := remote.BuildDoctorChecks(ws)

	// Find the "sshd present" check; before prepare it should fail.
	failedAtSshd := false
	for _, c := range checks {
		if c.Argv == nil {
			continue
		}
		cmd := exec.Command(c.Argv[0], c.Argv[1:]...)
		if err := cmd.Run(); err != nil {
			if strings.Contains(c.Label, "sshd present") {
				failedAtSshd = true
			}
			break
		}
	}
	if !failedAtSshd {
		t.Fatal("doctor did not detect missing sshd before prepare")
	}

	// Run prepare; checks should then pass.
	dir := t.TempDir()
	store := config.NewStore(dir)
	_, pub, err := store.EnsureKeypair()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := prepare.Run(ctx, ws, pub, os.Stderr); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	for _, c := range checks {
		if c.Argv == nil {
			continue
		}
		cmd := exec.Command(c.Argv[0], c.Argv[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("after prepare, check %q failed: %v\n%s", c.Label, err, out)
		}
	}
}
