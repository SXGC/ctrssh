package sshconfig_test

import (
	"strings"
	"testing"

	"github.com/SXGC/ctrssh/internal/sshconfig"
)

func TestUpsertOnEmpty(t *testing.T) {
	got := sshconfig.Upsert("", "work", "Host work.dev\n  User vscode\n")
	want := "# ctrssh start work\nHost work.dev\n  User vscode\n# ctrssh end work\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUpsertReplacesExisting(t *testing.T) {
	in := "Host other\n  User x\n# ctrssh start work\nHost old\n# ctrssh end work\nHost after\n"
	got := sshconfig.Upsert(in, "work", "Host work.dev\n  User new\n")
	if !strings.Contains(got, "Host other\n") {
		t.Fatal("user content above the block was lost")
	}
	if !strings.Contains(got, "Host after\n") {
		t.Fatal("user content below the block was lost")
	}
	if strings.Contains(got, "Host old") {
		t.Fatal("old block content was not replaced")
	}
	if !strings.Contains(got, "Host work.dev\n  User new\n") {
		t.Fatal("new block content not present")
	}
}

func TestRemoveStripsBlockOnly(t *testing.T) {
	in := "Host other\n# ctrssh start work\nHost work.dev\n# ctrssh end work\nHost after\n"
	got := sshconfig.Remove(in, "work")
	if strings.Contains(got, "ctrssh start work") || strings.Contains(got, "Host work.dev") {
		t.Fatal("block was not fully removed")
	}
	if !strings.Contains(got, "Host other\n") || !strings.Contains(got, "Host after\n") {
		t.Fatal("user content outside the block was modified")
	}
}

func TestUpsertIdempotent(t *testing.T) {
	once := sshconfig.Upsert("", "work", "Host work.dev\n")
	twice := sshconfig.Upsert(once, "work", "Host work.dev\n")
	if once != twice {
		t.Fatalf("not idempotent: \n%s\n!=\n%s", once, twice)
	}
}

func TestRemoveAbsent(t *testing.T) {
	in := "Host other\n"
	got := sshconfig.Remove(in, "work")
	if got != in {
		t.Fatalf("Remove changed content when block was absent: %q", got)
	}
}
