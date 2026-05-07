package remote

import "testing"

func TestShellSingleQuote(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"plain", `'plain'`},
		{"a b", `'a b'`},
		{"mkdir -p /run/sshd && sshd -i", `'mkdir -p /run/sshd && sshd -i'`},
		{"it's broken", `'it'\''s broken'`},
		{"", `''`},
	}
	for _, c := range cases {
		got := shellSingleQuote(c.in)
		if got != c.want {
			t.Errorf("shellSingleQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
