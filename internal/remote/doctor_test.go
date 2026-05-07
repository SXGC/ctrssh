package remote_test

import (
	"strings"
	"testing"

	"github.com/cby/ctrssh/internal/remote"
)

func TestBuildDoctorChecksOrder(t *testing.T) {
	checks := remote.BuildDoctorChecks(fixtureWS())
	wantSequence := []string{
		"local ssh client",
		"ssh: connect to host",
		"docker: container running",
		"container: sshd present",
		"container: sshd -t passes",
		"container: authorized_keys",
	}
	if len(checks) != len(wantSequence) {
		t.Fatalf("got %d checks, want %d", len(checks), len(wantSequence))
	}
	for i, want := range wantSequence {
		if !strings.Contains(checks[i].Label, want) {
			t.Fatalf("check[%d] label = %q, want substring %q", i, checks[i].Label, want)
		}
	}
}

func TestBuildDoctorChecksHaveCommands(t *testing.T) {
	checks := remote.BuildDoctorChecks(fixtureWS())
	for i, c := range checks {
		if i == 0 {
			// local check has no Argv (it inspects PATH in-process)
			continue
		}
		if len(c.Argv) == 0 {
			t.Fatalf("check[%d] %q has no Argv", i, c.Label)
		}
	}
}
