package remote

import "strings"

// shellSingleQuote wraps s in POSIX shell single quotes so the remote shell
// parses it as a single argument. Use this whenever a command string is sent
// through ssh: openssh joins all post-host argv tokens with spaces before
// shipping them to the remote shell, so any metacharacter in the inner
// command (notably && and ||) gets interpreted at the wrong level otherwise.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
