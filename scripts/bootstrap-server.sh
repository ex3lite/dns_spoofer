#!/usr/bin/env bash
# Bootstrap DnsSpoofer on a fresh server (first-time installation).
#
# Usage:
#   ./scripts/bootstrap-server.sh [SERVER]
#   ./scripts/bootstrap-server.sh --force [SERVER]  # Force reinstall
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

# Parse flags
FORCE=false
SERVER_ARG=""
if [[ "${1:-}" == "--force" ]]; then
  FORCE=true
  SERVER_ARG="${2:-}"
else
  SERVER_ARG="${1:-}"
fi

# Server: first arg, or env, or default
if [[ -n "$SERVER_ARG" ]]; then
  REMOTE="$SERVER_ARG"
else
  REMOTE="${DNS_SPOOFER_SERVER:-}"
fi

if [[ -z "$REMOTE" ]]; then
  echo "Usage: $0 [--force] <server>   OR   DNS_SPOOFER_SERVER=host $0" >&2
  echo "Example: $0 95.164.123.192" >&2
  echo "         $0 --force root@95.164.123.192" >&2
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

echo "=== DnsSpoofer bootstrap (first-time install) to $REMOTE ==="
cd "$REPO_ROOT"

# Check if already installed
if [[ "$FORCE" != "true" ]]; then
  if RUN_SSH "test -f $REMOTE_BINARY" 2>/dev/null; then
    echo "Warning: DnsSpoofer appears to be already installed at $REMOTE_BINARY" >&2
    echo "Use --force to reinstall, or use scripts/deploy.sh for updates" >&2
    exit 1
  fi
fi

echo "[1/6] Checking systemd availability..."
if ! RUN_SSH "command -v systemctl >/dev/null 2>&1"; then
  echo "Error: systemd is required but not found on the server" >&2
  exit 1
fi
echo "      ✓ systemd found"

echo "[2/6] Building Linux amd64 binary..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$BINARY_NAME-linux-amd64" .
echo "      Built ${BINARY_NAME}-linux-amd64"

echo "[3/6] Uploading binary and unit file..."
RUN_SCP "$BINARY_NAME-linux-amd64" "/tmp/dnsspoofer-new"
RUN_SCP "dnsspoofer.service" "/tmp/dnsspoofer.service"

echo "[4/6] Installing binary and systemd unit..."
RUN_SSH "mv /tmp/dnsspoofer-new $REMOTE_BINARY && chmod +x $REMOTE_BINARY && mv /tmp/dnsspoofer.service /etc/systemd/system/dnsspoofer.service && systemctl daemon-reload"
echo "      ✓ Binary installed to $REMOTE_BINARY"
echo "      ✓ Systemd unit installed"

echo "[5/6] Freeing port 53 and enabling service..."
RUN_SSH "systemctl stop dnsspoofer 2>/dev/null || true; systemctl stop systemd-resolved 2>/dev/null || true; systemctl disable systemd-resolved 2>/dev/null || true; systemctl enable dnsspoofer && systemctl start dnsspoofer"
echo "      ✓ Port 53 freed (systemd-resolved stopped/disabled)"
echo "      ✓ Service enabled and started"

echo "[6/6] Verifying installation..."
sleep 2
if RUN_SSH "systemctl is-active --quiet dnsspoofer"; then
  echo "      ✓ Service is running"
else
  echo "      ⚠ Service may not be running, check logs: journalctl -u dnsspoofer -f"
fi

echo ""
echo "=== Bootstrap complete ==="
echo "Service status:"
RUN_SSH "systemctl status dnsspoofer --no-pager -l" || true
echo ""
echo "To view logs: ssh $REMOTE 'journalctl -u dnsspoofer -f'"
echo "To update later: use scripts/deploy.sh"
