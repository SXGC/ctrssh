#!/usr/bin/env bash
# Manual smoke test. Requires:
#   - REMOTE_HOST env var (ssh-reachable, has docker)
#   - a running container named SMOKE_CTR (default: smoke-test) on REMOTE_HOST
set -euo pipefail

: "${REMOTE_HOST:?REMOTE_HOST=user@host required}"
SMOKE_CTR="${SMOKE_CTR:-smoke-test}"
NAME="ctrssh-smoke-$$"

trap 'ctrssh rm "$NAME" >/dev/null 2>&1 || true' EXIT

ctrssh add "$NAME" --host "$REMOTE_HOST" --container "$SMOKE_CTR" --user root
ctrssh prepare "$NAME"
ctrssh config-ssh

ssh -o BatchMode=yes "${NAME}.dev" whoami | grep -qx root
ssh -o BatchMode=yes "${NAME}.dev" "ls /" | head -1 >/dev/null

echo "smoke OK"
