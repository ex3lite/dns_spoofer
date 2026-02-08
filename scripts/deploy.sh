#!/usr/bin/env bash
# Deploy DnsSpoofer to a remote server: build, upload, restart service.
#
# Usage:
#   ./scripts/deploy.sh [SERVER]
#   make deploy
#
# Loads scripts/deploy.local if present (gitignored — put credentials there, do not push).
# Env (optional): DNS_SPOOFER_SERVER, DNS_SPOOFER_USER, DNS_SPOOFER_PORT, DNS_SPOOFER_PASSWORD
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load local credentials (scripts/deploy.local is in .gitignore — do not commit it)
if [[ -f "$SCRIPT_DIR/deploy.local" ]]; then
  set -a
  source "$SCRIPT_DIR/deploy.local"
  set +a
fi
BINARY_NAME="dnsspoofer"
REMOTE_BINARY="${DNS_SPOOFER_BINARY:-/usr/local/bin/dnsspoofer}"

# Server: first arg, or env, or default
SERVER_ARG="${1:-}"
if [[ -n "$SERVER_ARG" ]]; then
  REMOTE="$SERVER_ARG"
else
  REMOTE="${DNS_SPOOFER_SERVER:-}"
fi

if [[ -z "$REMOTE" ]]; then
  echo "Usage: $0 <server>   OR   DNS_SPOOFER_SERVER=host $0" >&2
  echo "Example: $0 YOUR_SERVER_IP" >&2
  echo "         $0 root@YOUR_SERVER_IP" >&2
  echo "" >&2
  echo "Or set DNS_SPOOFER_SERVER environment variable or in scripts/deploy.local" >&2
  exit 1
fi

# If REMOTE is just the host, prepend user
if [[ "$REMOTE" != *"@"* ]]; then
  USER="${DNS_SPOOFER_USER:-root}"
  REMOTE="${USER}@${REMOTE}"
fi

PORT="${DNS_SPOOFER_PORT:-22}"
SSH_OPTS=(-o StrictHostKeyChecking=no -o ConnectTimeout=10 -p "$PORT")
SCP_OPTS=(-o StrictHostKeyChecking=no -o ConnectTimeout=10 -P "$PORT")

# Optional sshpass for password auth
RUN_SSH() {
  if [[ -n "${DNS_SPOOFER_PASSWORD:-}" ]]; then
    sshpass -p "$DNS_SPOOFER_PASSWORD" ssh "${SSH_OPTS[@]}" "$REMOTE" "$@"
  else
    ssh "${SSH_OPTS[@]}" "$REMOTE" "$@"
  fi
}

RUN_SCP() {
  local src="$1"
  local dest="$2"
  if [[ -n "${DNS_SPOOFER_PASSWORD:-}" ]]; then
    sshpass -p "$DNS_SPOOFER_PASSWORD" scp "${SCP_OPTS[@]}" "$src" "$REMOTE:$dest"
  else
    scp "${SCP_OPTS[@]}" "$src" "$REMOTE:$dest"
  fi
}

echo "=== DnsSpoofer deploy to $REMOTE ==="
cd "$REPO_ROOT"

echo "[1/5] Building Linux amd64 binary..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$BINARY_NAME-linux-amd64" .
echo "      Built ${BINARY_NAME}-linux-amd64"

echo "[2/5] Uploading binary and unit file..."
RUN_SCP "$BINARY_NAME-linux-amd64" "/tmp/dnsspoofer-new"
RUN_SCP "dnsspoofer.service" "/tmp/dnsspoofer.service"

echo "[3/5] Installing binary and systemd unit..."
RUN_SSH "mv /tmp/dnsspoofer-new $REMOTE_BINARY && chmod +x $REMOTE_BINARY && mv /tmp/dnsspoofer.service /etc/systemd/system/dnsspoofer.service && systemctl daemon-reload"

echo "[4/5] Freeing port 53 and restarting service..."
RUN_SSH "systemctl stop dnsspoofer 2>/dev/null || true; systemctl stop systemd-resolved 2>/dev/null || true; systemctl disable systemd-resolved 2>/dev/null || true; systemctl enable dnsspoofer 2>/dev/null || true; systemctl start dnsspoofer"

echo "[5/5] Status:"
RUN_SSH "systemctl status dnsspoofer --no-pager" || true

echo "=== Deploy done ==="
