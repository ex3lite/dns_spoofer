# Debug Tools

This directory contains debugging and troubleshooting utilities for DnsSpoofer.

## Tools

### `fetch-server-debug.sh`

Fetches detailed debug logs from the server when `DEBUG_LOG_PATH` is configured.

**Prerequisites:**
- Server must have `DEBUG_LOG_PATH` environment variable set in systemd service
- SSH access to server (via `scripts/deploy.local` or SSH keys)

**Usage:**
```bash
./scripts/debug/fetch-server-debug.sh [output_file]
```

### `logs-errors.sh`

Filters server logs to show only errors, connection issues, and tunnel closures.

**Usage:**
```bash
./scripts/debug/logs-errors.sh [lines]
# Default: 500 lines
```

### `check_resolver.go`

Tests DNS resolver behavior for repeated hostname lookups.

**Usage:**
```bash
go run scripts/debug/check_resolver.go
```

## Configuration

All tools use the same server configuration as deployment scripts:
- `scripts/deploy.local` file (gitignored)
- Environment variables: `DNS_SPOOFER_SERVER`, `DNS_SPOOFER_USER`, `DNS_SPOOFER_PORT`, `DNS_SPOOFER_PASSWORD`
- SSH key authentication

For detailed usage instructions, see the main [README.md](../../README.md) and [README.ru.md](../../README.ru.md).
