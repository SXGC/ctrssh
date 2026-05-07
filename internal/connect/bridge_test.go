package connect_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/SXGC/ctrssh/internal/connect"
)

// TestRunPassthrough exercises the stdio bridge using `cat` as the child:
// bytes written to stdin should come back unchanged on stdout.
func TestRunPassthrough(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	in := strings.NewReader("hello world\n")
	var out bytes.Buffer
	var stderr bytes.Buffer
	err := connect.Run(ctx, []string{"cat"}, in, &out, &stderr)
	if err != nil {
		t.Fatalf("Run: %v (stderr=%s)", err, stderr.String())
	}
	if out.String() != "hello world\n" {
		t.Fatalf("output = %q, want %q", out.String(), "hello world\n")
	}
}

func TestRunPropagatesExitCode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := connect.Run(ctx, []string{"sh", "-c", "exit 7"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	ee, ok := connect.AsExitError(err)
	if !ok {
		t.Fatalf("expected ExitError, got %v", err)
	}
	if ee.ExitCode() != 7 {
		t.Fatalf("exit code = %d, want 7", ee.ExitCode())
	}
}

func TestRunMissingBinary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := connect.Run(ctx, []string{"definitely_does_not_exist_99999"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}
