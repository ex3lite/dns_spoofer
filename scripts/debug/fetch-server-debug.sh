#!/usr/bin/env bash
# Fetch server-side debug log from dnsspoofer (when DEBUG_LOG_PATH is set on server).
# On server, add to [Service]: Environment=DEBUG_LOG_PATH=/tmp/dnsspoofer_debug.log
# Then: systemctl restart dnsspoofer. After repro, run this script.
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
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
[[ "$REMOTE" != *"@"* ]] && REMOTE="${DNS_SPOOFER_USER:-root}@${REMOTE}"
PORT="${DNS_SPOOFER_PORT:-22}"
SSH_OPTS=(-o StrictHostKeyChecking=no -o ConnectTimeout=10 -p "$PORT")
RUN_SSH() {
  if [[ -n "${DNS_SPOOFER_PASSWORD:-}" ]]; then
    sshpass -p "$DNS_SPOOFER_PASSWORD" ssh "${SSH_OPTS[@]}" "$REMOTE" "$@"
  else
    ssh "${SSH_OPTS[@]}" "$REMOTE" "$@"
  fi
}
OUT="${1:-$REPO_ROOT/.cursor/debug_server.log}"
RUN_SSH "cat /tmp/dnsspoofer_debug.log 2>/dev/null || echo 'No /tmp/dnsspoofer_debug.log (set DEBUG_LOG_PATH and restart dnsspoofer)'" > "$OUT"
echo "Server debug log written to $OUT"
