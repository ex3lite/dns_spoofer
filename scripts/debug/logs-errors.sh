#!/usr/bin/env bash
# Fetch last dnsspoofer logs and show only errors / googlevideo / tunnel close.
# Usage: ./scripts/debug/logs-errors.sh [lines]
# Requires: deploy.local with DNS_SPOOFER_* (or SSH key to root@server)
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
LINES="${1:-500}"
if [[ -f "$REPO_ROOT/scripts/deploy.local" ]]; then
  set -a
  source "$REPO_ROOT/scripts/deploy.local"
  set +a
fi
REMOTE="${DNS_SPOOFER_SERVER:-}"
if [[ -z "$REMOTE" ]]; then
  echo "Error: DNS_SPOOFER_SERVER not set. Set it in scripts/deploy.local or as environment variable." >&2
  exit 1
fi
if [[ "$REMOTE" != *"@"* ]]; then
  REMOTE="root@${REMOTE}"
fi
PORT="${DNS_SPOOFER_PORT:-22}"
SSH_OPTS=(-o StrictHostKeyChecking=no -o ConnectTimeout=10 -p "$PORT")
RUN() {
  if [[ -n "${DNS_SPOOFER_PASSWORD:-}" ]]; then
    sshpass -p "$DNS_SPOOFER_PASSWORD" ssh "${SSH_OPTS[@]}" "$REMOTE" "$@"
  else
    ssh "${SSH_OPTS[@]}" "$REMOTE" "$@"
  fi
}
echo "=== Last ${LINES} lines, filtered for errors / googlevideo / Tunnel closed ==="
RUN "journalctl -u dnsspoofer -n $LINES --no-pager" | grep -E 'ERROR|googlevideo|Tunnel closed|SNI peek|Backend dial|Resolve error|copy error|i/o timeout' || true
