package prepare_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cby/ctrssh/internal/prepare"
)

// The runner sends a script to a command via stdin and validates the last
// line is "OK". We exercise that contract using `bash -c "cat >/dev/null; echo $1"`
// as a stand-in shell that consumes the script and emits a controlled tail.

func TestRunSuccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var log bytes.Buffer
	err := prepare.RunCommand(ctx,
		[]string{"bash", "-c", "cat >/dev/null; echo OK"},
		"the-script",
		&log,
	)
	if err != nil {
		t.Fatalf("Run: %v\nlog:\n%s", err, log.String())
	}
}

func TestRunFailureSurfacesStderr(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var log bytes.Buffer
	err := prepare.RunCommand(ctx,
		[]string{"bash", "-c", "echo something_failed >&2; echo NOT_OK"},
		"x",
		&log,
	)
	if err == nil {
		t.Fatal("expected error when last line is not OK")
	}
	if !strings.Contains(err.Error(), "OK") {
		t.Fatalf("error should mention OK sentinel; got %q", err.Error())
	}
	if !strings.Contains(log.String(), "something_failed") {
		t.Fatalf("stderr not surfaced; log=%q", log.String())
	}
}
