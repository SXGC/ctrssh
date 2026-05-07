# ctrssh

[中文](README.zh-CN.md)

Expose a remote container running on an SSH-reachable host as a regular SSH endpoint, so you can `ssh work.dev` (or open in VSCode Remote-SSH / JetBrains Gateway / Cursor) without exposing any TCP port from the container.

## How it works

```
local ssh client ──▶ ProxyCommand ──▶ ctrssh connect --stdio
                                          │
                                          ▼
                              ssh user@host docker exec -i ctr sshd -i
                                          │
                                          ▼
                            container's openssh-server (inetd mode)
                                          │
                                          ▼
                                       your shell
```

No daemons, no exposed ports. The chain is pure stdio.

## Install

```bash
go install github.com/SXGC/ctrssh/cmd/ctrssh@latest
```

## Use

```bash
# one-time per workspace
ctrssh add work --host me@server --container devbox --user vscode
ctrssh prepare work     # installs sshd + injects pubkey in the container
ctrssh config-ssh       # writes ~/.ssh/config (auto-runs after `add`)

# daily
ssh work.dev
code --remote ssh-remote+work.dev /workspaces
# JetBrains Gateway: pick host work.dev
```

## Commands

| command | purpose |
|---|---|
| `add <name>` | register a workspace; auto-refresh ssh config |
| `list` | list registered workspaces |
| `rm <name>` | remove a workspace |
| `prepare <name>` | one-time remote setup (idempotent) |
| `config-ssh` | refresh `~/.ssh/config` entries |
| `connect --stdio <name>` | ProxyCommand body (not invoked manually) |
| `doctor <name>` | diagnose connectivity step-by-step |

## Requirements

- **Local**: OpenSSH client on PATH (POSIX systems and Windows 10+ have it)
- **Remote host**: docker CLI, user has docker permissions, ssh reachable
- **Container**: any Linux base; `prepare` installs `openssh-server` if missing

## Files

```
~/.config/ctrssh/
  workspaces.yaml          # registry
  id_ctrssh, id_ctrssh.pub # tool-specific keypair
~/.ssh/config              # ctrssh writes Host blocks delimited by markers
```
