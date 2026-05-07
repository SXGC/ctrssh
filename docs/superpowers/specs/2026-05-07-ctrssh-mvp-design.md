# ctrssh MVP — Design Spec

**Date:** 2026-05-07
**Status:** Draft, awaiting review
**Author:** brainstorming session

## Goal

Extract the "remote container as a regular SSH server" capability from [devpod](https://github.com/loft-sh/devpod) into a small, standalone CLI tool. After running it, a user can `ssh work.dev` (or open the host in VSCode Remote-SSH / JetBrains Gateway / Cursor) and land inside a Docker container running on a remote host — without exposing any TCP port from the container.

## Non-Goals

- Container lifecycle (creation, build, start/stop) — assumes container is already running
- devcontainer.json compliance, features, postCreate hooks
- Kubernetes, local-only Docker, or non-SSH-reachable targets
- Multi-tenant platform features (web UI, user management, RBAC)
- Custom Go SSH server or binary injection — reuses container's existing `openssh-server`
- Cross-compilation matrix for in-container binaries
- Windows-without-OpenSSH support (relies on system `ssh` client)

## High-Level Approach

A single Go binary plus a local YAML registry. Each connection chains:

```
local ssh client
  → ProxyCommand: ctrssh connect --stdio <name>
  → exec: ssh user@host docker exec -i <ctr> sshd -i
  → container's sshd in inetd mode handles SSH protocol over the stdio pipe
```

No daemons, no servers, no exposed ports. Pure stdio transport, identical in spirit to devpod's design but reusing `sshd -i` instead of bundling a custom Go SSH server.

## Architecture

### Filesystem layout (user)

```
~/.config/ctrssh/
  workspaces.yaml      # registry (plaintext YAML, local only)
  id_ctrssh            # auto-generated ed25519 keypair (avoids polluting ~/.ssh/id_*)
  id_ctrssh.pub
  connect.log          # opt-in debug log; off by default

~/.ssh/config          # ctrssh writes Host blocks delimited by markers
```

### External runtime dependencies

- **Local**: system `ssh` client (OpenSSH). POSIX systems have it; Windows 10+ ships it.
- **Remote host**: `docker` CLI on PATH; user has docker permissions.
- **Container**: `openssh-server` package. Installed by `prepare` if missing.

### Repository layout

```
ctrssh/
├── cmd/ctrssh/main.go              # cobra entry, subcommand dispatch
├── internal/
│   ├── workspace/                  # Workspace struct + Validate()
│   ├── config/                     # workspaces.yaml read/write, keypair gen, flock
│   ├── sshconfig/                  # ~/.ssh/config marker-block management
│   ├── remote/                     # command-string builders (pure functions)
│   ├── connect/                    # stdio bridge for ProxyCommand
│   └── prepare/                    # one-time remote setup script driver
├── testdata/                       # golden files
├── scripts/smoke.sh                # manual end-to-end verification
├── go.mod
└── README.md
```

## Components

Each `internal/<x>` package has one job, communicates via plain structs, and shares no global state.

### `internal/workspace`
- **What**: domain model
- **Interface**: `type Workspace struct { Name, SSHHost, Container, RemoteUser string }`; `Validate() error`
- **Depends on**: nothing
- **Field semantics**: `SSHHost` is `user@host` (the SSH-server user); `RemoteUser` is the in-container user we authenticate as. Empty `SSHHost` is a test-only short-circuit (see Design Choices).

### `internal/config`
- **What**: load/save `workspaces.yaml`, generate keypair, file locking
- **Interface**: `Load() ([]Workspace, error)`; `Save([]Workspace) error`; `Add/Remove/Get(name)`; `EnsureKeypair() (privPath string, pub []byte, err error)`
- **Depends on**: `gopkg.in/yaml.v3`, `golang.org/x/crypto/ed25519`, `github.com/gofrs/flock` (or equivalent)
- **Concurrency**: flock around all writes; 5-second timeout, then error rather than block forever

### `internal/sshconfig`
- **What**: idempotent management of marker-delimited blocks in `~/.ssh/config`
- **Markers**: `# ctrssh start <name>` / `# ctrssh end <name>`
- **Interface**: `Upsert(name, block string) error`; `Remove(name string) error`; `UpsertAll([]Workspace, execPath string) error`
- **Depends on**: stdlib only
- **Inspiration**: simplified version of devpod's `pkg/ssh/config.go`

### `internal/remote`
- **What**: pure functions that build command-line strings; no IO
- **Interface**:
  - `BuildConnectArgs(ws Workspace, identityPath string) []string` — identityPath is the absolute path to the private key
  - `BuildPrepareScript(ws Workspace, pubkey []byte) string`
  - `BuildDoctorChecks(ws Workspace) []DoctorCheck`
- **Depends on**: only the workspace type

### `internal/connect`
- **What**: ProxyCommand body — exec the chain, bind stdio transparently, forward signals
- **Interface**: `Run(ctx context.Context, args []string) error`
- **Depends on**: `os/exec`, `os/signal`
- **Invariant**: never writes to its own stdout. Logs go to stderr or `connect.log`.

### `internal/prepare`
- **What**: drives the one-time remote setup
- **Interface**: `Run(ctx context.Context, ws Workspace, pubkey []byte, log io.Writer) error`
- **Mechanism**: ssh + heredoc-piped bash script; parses last stdout line for `OK` sentinel

### `cmd/ctrssh/*.go`
- One file per subcommand (cobra). Pure parameter parsing + delegation to internal packages. No business logic.

## Data Flow

### A. Daily SSH access (`ssh work.dev`)

```
local ssh client
   │  reads ~/.ssh/config → ProxyCommand
   ▼
ctrssh connect --stdio work
   │  load workspace; build args:
   │    ssh -T -i ~/.config/ctrssh/id_ctrssh user@host \
   │       docker exec -i -u root <container> \
   │       /usr/sbin/sshd -i -e -f /etc/ssh/sshd_config_ctrssh
   │  exec.Cmd with stdin=os.Stdin, stdout=os.Stdout (no buffering, no io.Copy)
   ▼
remote sshd → exec docker
   ▼
docker exec -i ... sshd -i
   │  container's sshd treats this stdio as one SSH connection
   ▼
SSH handshake → publickey auth against <RemoteUser>'s authorized_keys → shell
```

**Critical invariant**: `ctrssh connect --stdio` must never write a single byte to its own stdout outside of the child process's pass-through. Any extra byte breaks the outer SSH handshake. All logging goes to stderr or `connect.log`.

### B. One-time setup (`ctrssh prepare work`)

```
ctrssh prepare work
   │  ① EnsureKeypair() — generate ed25519 if absent
   │  ② render prepare.sh (heredoc) embedding pubkey text
   │  ③ ssh user@host 'docker exec -i -u root <ctr> bash -s' < prepare.sh
   ▼
prepare.sh (idempotent):
   1. command -v sshd || install via apt/apk/yum
   2. [ -f /etc/ssh/ssh_host_ed25519_key ] || ssh-keygen -A
   3. mkdir -p ~<RemoteUser>/.ssh && chmod 700
   4. grep -qxF "<pubkey>" authorized_keys || echo "<pubkey>" >> authorized_keys
   5. chown <RemoteUser>: -R ~<RemoteUser>/.ssh
   6. write /etc/ssh/sshd_config_ctrssh with:
        PubkeyAuthentication yes
        PasswordAuthentication no
        UsePAM no
        HostKey /etc/ssh/ssh_host_ed25519_key
        Subsystem sftp internal-sftp
   7. /usr/sbin/sshd -t -f /etc/ssh/sshd_config_ctrssh
   8. echo "OK"
   ▼
ctrssh parses last line; OK = success, otherwise surface stderr verbatim
```

Re-running `prepare` is a no-op (deduped authorized_keys, install detection, config overwrite is byte-stable).

### C. Registration (`add` + `config-ssh`)

```
ctrssh add work --host me@server --container devbox --user vscode
   │  flock workspaces.yaml; append; save
   │  auto-invokes config-ssh
ctrssh config-ssh
   │  Load(); for each ws render block:
   │     # ctrssh start work
   │     Host work.dev
   │       ForwardAgent yes
   │       StrictHostKeyChecking no
   │       UserKnownHostsFile /dev/null
   │       IdentityFile ~/.config/ctrssh/id_ctrssh
   │       IdentitiesOnly yes
   │       User vscode
   │       ProxyCommand "/abs/path/ctrssh" connect --stdio work
   │     # ctrssh end work
   │  flock ~/.ssh/config; atomic write (.tmp + rename)
```

## Design Choices Worth Calling Out

1. **Separate sshd config file** (`/etc/ssh/sshd_config_ctrssh`) rather than mutating the container's existing `sshd_config`. Avoids breaking images that ship with restrictive defaults.
2. **`-u root` on docker exec** is required because `sshd -i` reads host keys and must `setuid` to the target user after authentication.
3. **`StrictHostKeyChecking no` + `UserKnownHostsFile /dev/null`** in the generated config. Container host keys may rotate when the container is recreated; same tradeoff devpod makes.
4. **No ControlMaster / multiplexing in MVP.** Trivial to add later via one-line ssh config change.
5. **Dedicated keypair** at `~/.config/ctrssh/id_ctrssh`, not reusing `~/.ssh/id_*`. Keeps the tool's footprint isolated and prevents `prepare` from accidentally inheriting unrelated identities.
6. **`Workspace.SSHHost == ""` is a test-only short-circuit** that skips the outer ssh hop and runs `docker exec` directly. Lets e2e tests run on a single CI machine without provisioning a jump host. Not a user feature.

## Error Handling

The chain has 5 segments. Strategy: fail fast, surface where it broke, never paper over.

| Segment | Failure cause | User sees | Strategy |
|---|---|---|---|
| Local ssh client missing | `LookPath("ssh")` fails | clear error + install hint | fail-fast at startup |
| ssh dial to remote | network/auth | OpenSSH stderr (`Permission denied` etc.) | passthrough — OpenSSH messages are good |
| docker exec | container missing/unauthorized | docker stderr | passthrough; `doctor` catches earlier |
| `sshd -i` startup | binary missing/config bad | EOF visible to local ssh; stderr lost in protocol | mitigate via `prepare`'s `sshd -t` precheck and `doctor` segment-by-segment probes |
| SSH handshake | auth failed | local ssh's own error | passthrough |

### `doctor` command

Each check is an independent `ssh ...` invocation, stops at first failure, surfaces it cleanly:

```
$ ctrssh doctor work
 ✓ local: ssh client found
 ✓ ssh: connected to me@server (1.2.3.4)
 ✓ docker: container 'devbox' is running
 ✓ container: /usr/sbin/sshd present (OpenSSH_9.6)
 ✓ container: sshd -t passes
 ✓ container: authorized_keys contains our pubkey
 ✓ end-to-end: SSH handshake OK (whoami=vscode)
```

### Signals & exit codes

- `SIGTERM` / `SIGINT` → forward to child process
- Child exit code → propagated as own exit code
- No panic recovery; let the process die so ssh handles it

### File errors

| Case | Behavior |
|---|---|
| missing `workspaces.yaml` | treated as empty list |
| corrupt yaml | error + path to backup; no auto-repair |
| user-edited content outside markers in `~/.ssh/config` | preserved verbatim |
| `add` with duplicate name | reject; suggest `rm` or `--force` |
| concurrent yaml writes | flock serializes; 5s timeout |

### Logging

- Subcommands silent on success (Unix style)
- `--verbose` echoes ssh/docker invocations to stderr
- `connect --stdio` never writes to stdout; debug log to `~/.config/ctrssh/connect.log` only when `CTRSSH_DEBUG=1`

### Explicitly not done

- Auto-retry (would mask real failures)
- Wrapping/translating OpenSSH errors (originals are clearer)
- Global timeout (let SSH keepalive handle it)

## Testing Strategy

### Tier 1 — Unit tests (most coverage here)

| Package | What | How |
|---|---|---|
| `workspace` | `Validate()` rejects bad names (empty, spaces, slashes, reserved) | table-driven |
| `remote` | `BuildConnectArgs` / `BuildPrepareScript` produce exact strings | golden files in `testdata/` |
| `sshconfig` | upsert/remove idempotency; preserves user content outside markers | text input → expected text |
| `config` | yaml stable; flock works; concurrent `add` is safe | tempdir + goroutines |

Goal: 80%+ line coverage, full unit suite < 5 seconds.

### Tier 2 — Integration tests (`go test -tags=e2e`)

Real container, no mocks:

```go
func TestPrepareAndConnect(t *testing.T) {
    if testing.Short() { t.Skip() }
    // 1. docker run -d --name ctrssh-test alpine:latest sleep infinity
    // 2. ws := Workspace{Name: "test", SSHHost: "", Container: "ctrssh-test", RemoteUser: "root"}
    //    SSHHost == "" → bypass outer ssh, docker exec directly (test-only path)
    // 3. ctrssh prepare test
    // 4. ctrssh connect --stdio test, dialed via golang.org/x/crypto/ssh.NewClientConn
    // 5. exec "whoami" → expect "root"
    // 6. defer docker rm -f
}
```

Coverage targets:
- `prepare` on clean alpine (full install path)
- `prepare` re-run idempotency
- `connect --stdio` end-to-end SSH handshake + command execution
- `doctor` on intentionally broken container (sshd removed, authorized_keys cleared) localizes the right segment

Budget: < 30 seconds total. Requires local docker. Runs on its own CI job (`-tags=e2e`).

### Tier 3 — Manual smoke (`scripts/smoke.sh`)

Run before each release against a real ssh-reachable host. Not in CI (jump-host provisioning is too heavy).

```bash
ctrssh add smoke --host "$REMOTE_HOST" --container smoke-test --user root
ctrssh prepare smoke
ctrssh config-ssh
ssh smoke.dev whoami       # expect: root
ssh smoke.dev "ls /"       # expect: non-empty
ctrssh rm smoke
```

### Explicitly not tested

- mocked `ssh` / `docker` invocations (mock success doesn't prove real pipe works)
- in-process stdio bridge mocks (e2e covers this directly)
- OpenSSH / docker behavior itself (trust system tools)

### CI

GitHub Actions matrix:
- `go test ./...` — unit, every push
- `go test -tags=e2e ./...` — integration with docker service, every push
- Manual smoke not gated

## End-to-End User Flow (target experience)

```bash
# one-time per workspace
ctrssh add work --host me@server.example.com --container devbox --user vscode
ctrssh prepare work        # installs sshd, injects pubkey
ctrssh config-ssh          # writes ~/.ssh/config

# daily use
ssh work.dev                                      # native ssh
code --remote ssh-remote+work.dev /workspaces     # VSCode
# JetBrains Gateway: pick host work.dev
# Cursor / Windsurf: same as VSCode
```

## Open Questions / Deferred Decisions

None blocking. Tracking these as post-MVP work:
- ControlMaster multiplexing (one ssh config flag)
- Approach B upgrade: native Go SSH client driving the chain (better Windows story)
- GPG agent forwarding (one ssh config flag, like devpod)
- Workdir auto-cd on connect

## References

- devpod SSH server: `pkg/ssh/server/ssh.go`
- devpod stdio listener: `cmd/helper/ssh_server.go:123`
- devpod ssh config writer: `pkg/ssh/config.go:64`
- devpod tunnel: `pkg/tunnel/container.go:47`
- OpenSSH inetd mode: `sshd(8)` `-i` flag
