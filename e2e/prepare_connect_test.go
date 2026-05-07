//go:build e2e

package e2e_test

import (
	"context"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/SXGC/ctrssh/internal/config"
	"github.com/SXGC/ctrssh/internal/connect"
	"github.com/SXGC/ctrssh/internal/prepare"
	"github.com/SXGC/ctrssh/internal/remote"
	"github.com/SXGC/ctrssh/internal/workspace"
	"golang.org/x/crypto/ssh"
)

const (
	testImage = "alpine:3.19"
	testCtr   = "ctrssh-e2e"
)

func dockerRun(t *testing.T, name, image string) {
	t.Helper()
	_ = exec.Command("docker", "rm", "-f", name).Run()
	out, err := exec.Command("docker", "run", "-d", "--name", name, image,
		"sleep", "infinity").CombinedOutput()
	if err != nil {
		t.Skipf("docker run failed; skipping e2e: %v\n%s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })
}

func TestPrepareThenConnect(t *testing.T) {
	dockerRun(t, testCtr, testImage)

	dir := t.TempDir()
	store := config.NewStore(dir)
	_, pub, err := store.EnsureKeypair()
	if err != nil {
		t.Fatal(err)
	}
	ws := workspace.Workspace{Name: "e2e", SSHHost: "", Container: testCtr, RemoteUser: "root"}
	if err := store.Add(ws); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// 1. prepare
	if err := prepare.Run(ctx, ws, pub, os.Stderr); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	// 2. set up SSH client config from the generated private key
	priv := store.PrivateKeyPath()
	keyBytes, err := os.ReadFile(priv)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}

	// 3. wire ssh.NewClientConn ↔ connect.RunFiles via os.Pipe pairs.
	// Pipes: ssh client → inW (in this proc) → inR (child stdin)
	//        child stdout → outW → outR (read by ssh client)
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = connect.RunFiles(ctx, remote.BuildConnectArgs(ws, priv, ""), inR, outW, os.Stderr)
		_ = outW.Close()
		_ = inR.Close()
	}()
	conn, chans, reqs, err := ssh.NewClientConn(newPipeConn(outR, inW), "ctrssh-e2e", cfg)
	if err != nil {
		t.Fatalf("ssh handshake: %v", err)
	}
	client := ssh.NewClient(conn, chans, reqs)
	defer client.Close()

	// 4. run a command
	sess, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	out, err := sess.CombinedOutput("whoami")
	if err != nil {
		t.Fatalf("whoami: %v (out=%s)", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "root" {
		t.Fatalf("whoami = %q, want root", got)
	}

	// 5. re-run prepare; should be idempotent
	if err := prepare.Run(ctx, ws, pub, os.Stderr); err != nil {
		t.Fatalf("idempotent prepare: %v", err)
	}
}

// pipeConn glues a separate reader and writer into a net.Conn so it can be
// passed to ssh.NewClientConn.
type pipeConn struct {
	r *os.File
	w *os.File
}

func newPipeConn(r, w *os.File) *pipeConn { return &pipeConn{r: r, w: w} }

func (c *pipeConn) Read(p []byte) (int, error)        { return c.r.Read(p) }
func (c *pipeConn) Write(p []byte) (int, error)       { return c.w.Write(p) }
func (c *pipeConn) Close() error                      { _ = c.r.Close(); return c.w.Close() }
func (c *pipeConn) LocalAddr() net.Addr               { return pipeAddr{} }
func (c *pipeConn) RemoteAddr() net.Addr              { return pipeAddr{} }
func (c *pipeConn) SetDeadline(time.Time) error       { return nil }
func (c *pipeConn) SetReadDeadline(time.Time) error   { return nil }
func (c *pipeConn) SetWriteDeadline(time.Time) error  { return nil }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }
