//go:build unix

package connect

import (
	"os"
	"syscall"
)

type _OSSignal = os.Signal

var _signals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT}

func toSyscallSignal(s os.Signal) (syscall.Signal, bool) {
	if sig, ok := s.(syscall.Signal); ok {
		return sig, true
	}
	return 0, false
}
