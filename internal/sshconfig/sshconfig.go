// Package sshconfig manages marker-delimited blocks within an OpenSSH client
// config file. Blocks are bounded by lines:
//
//	# ctrssh start <name>
//	...
//	# ctrssh end <name>
//
// Content outside markers is preserved verbatim. All operations are pure
// string transformations; callers handle file IO.
package sshconfig

import (
	"strings"
)

const (
	StartPrefix = "# ctrssh start "
	EndPrefix   = "# ctrssh end "
)

// Upsert returns config with the named block replaced (or appended if absent).
// The supplied body should NOT include the marker lines; they are added.
func Upsert(config, name, body string) string {
	stripped, hadBlock := stripBlock(config, name)
	block := StartPrefix + name + "\n" + ensureTrailingNL(body) + EndPrefix + name + "\n"
	if hadBlock {
		// Re-insert at the original location (top of remaining content).
		return block + ensureSeparator(stripped)
	}
	if stripped == "" {
		return block
	}
	return block + ensureSeparator(stripped)
}

// Remove returns config with the named block stripped. Returns config unchanged
// if no such block exists.
func Remove(config, name string) string {
	out, _ := stripBlock(config, name)
	return out
}

// stripBlock removes the named marker block. Returns the new content and
// a flag indicating whether a block was found.
func stripBlock(config, name string) (string, bool) {
	start := StartPrefix + name
	end := EndPrefix + name
	lines := strings.Split(config, "\n")
	out := make([]string, 0, len(lines))
	inBlock := false
	found := false
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		switch {
		case !inBlock && trimmed == start:
			inBlock = true
			found = true
		case inBlock && trimmed == end:
			inBlock = false
		case !inBlock:
			out = append(out, ln)
		}
	}
	return strings.Join(out, "\n"), found
}

func ensureTrailingNL(s string) string {
	if s == "" || strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

func ensureSeparator(s string) string {
	if s == "" {
		return s
	}
	if strings.HasPrefix(s, "\n") {
		return s
	}
	return s
}
