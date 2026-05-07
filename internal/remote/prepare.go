package remote

import (
	"fmt"
	"strings"

	"github.com/SXGC/ctrssh/internal/workspace"
)

// BuildPrepareScript returns a bash script that, when run inside the target
// container as root, idempotently installs openssh-server, ensures host keys,
// writes /etc/ssh/sshd_config_ctrssh, and appends pubkey to the remote user's
// authorized_keys. The final stdout line is "OK" on success.
func BuildPrepareScript(ws workspace.Workspace, pubkey []byte) string {
	pub := strings.TrimSpace(string(pubkey))
	// Single-quote heredoc body to suppress shell expansion of $vars in the
	// script. Variables we *want* expanded are in $...$ regions outside the body.
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

REMOTE_USER=%q
PUBKEY=%q

if ! command -v sshd >/dev/null 2>&1; then
  if command -v apt-get >/dev/null 2>&1; then
    DEBIAN_FRONTEND=noninteractive apt-get update -qq
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq openssh-server >/dev/null
  elif command -v apk >/dev/null 2>&1; then
    apk add --no-cache openssh-server >/dev/null
  elif command -v yum >/dev/null 2>&1; then
    yum install -y -q openssh-server >/dev/null
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y -q openssh-server >/dev/null
  else
    echo "no supported package manager found" >&2
    exit 1
  fi
fi

if [ ! -f /etc/ssh/ssh_host_ed25519_key ]; then
  ssh-keygen -A >/dev/null
fi

HOMEDIR="$(getent passwd "$REMOTE_USER" | cut -d: -f6)"
if [ -z "$HOMEDIR" ]; then
  echo "user $REMOTE_USER does not exist in container" >&2
  exit 1
fi

mkdir -p "$HOMEDIR/.ssh"
chmod 700 "$HOMEDIR/.ssh"
touch "$HOMEDIR/.ssh/authorized_keys"
if ! grep -qxF "$PUBKEY" "$HOMEDIR/.ssh/authorized_keys"; then
  echo "$PUBKEY" >> "$HOMEDIR/.ssh/authorized_keys"
fi
chmod 600 "$HOMEDIR/.ssh/authorized_keys"
chown -R "$REMOTE_USER":"$(id -gn "$REMOTE_USER")" "$HOMEDIR/.ssh"

cat >/etc/ssh/sshd_config_ctrssh <<'CFG'
PubkeyAuthentication yes
PasswordAuthentication no
UsePAM no
HostKey /etc/ssh/ssh_host_ed25519_key
Subsystem sftp internal-sftp
PermitRootLogin prohibit-password
CFG

/usr/sbin/sshd -t -f /etc/ssh/sshd_config_ctrssh
echo OK
`, ws.RemoteUser, pub)
}
