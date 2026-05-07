// Package connect implements the ProxyCommand body: an exec wrapper that
// transparently bridges stdio between the calling SSH client and a child
// process chain. It must NEVER write to its own stdout outside of the child's
// pass-through, because the parent ssh client is mid-handshake on that pipe.
package connect

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"os/signal"
	"syscall"
)

// Run executes argv with stdin/stdout wired directly to the supplied readers
// and writers. Signals SIGINT/SIGTERM received by this process are forwarded
// to the child. Returns nil on clean exit, or an *exec.ExitError on non-zero
// child exit. Other errors indicate a setup failure (e.g. binary not found).
func Run(ctx context.Context, argv []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(argv) == 0 {
		return errors.New("argv is empty")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	sigs := make(chan signalEvent, 4)
	stop := installSignalHandler(sigs)
	defer stop()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", argv[0], err)
	}

	// Forward signals to the child until it exits.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for {
		select {
		case s := <-sigs:
			_ = cmd.Process.Signal(s.sig)
		case err := <-done:
			return err
		}
	}
}

// AsExitError reports whether err is an *exec.ExitError and returns it.
func AsExitError(err error) (*exec.ExitError, bool) {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee, true
	}
	return nil, false
}

type signalEvent struct{ sig syscall.Signal }

func installSignalHandler(out chan<- signalEvent) func() {
	c := make(chan _OSSignal, 4)
	signal.Notify(c, _signals...)
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case s := <-c:
				if sig, ok := toSyscallSignal(s); ok {
					out <- signalEvent{sig: sig}
				}
			case <-stop:
				signal.Stop(c)
				return
			}
		}
	}()
	return func() { close(stop) }
}
