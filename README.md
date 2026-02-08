![DNS Spoofer](header.png)

**DNS relay with selective spoofing + transparent TCP proxy** for redirecting service traffic (OpenAI, ChatGPT, Google Gemini, Cursor, Microsoft Copilot) through your own server. One binary: DNS answers with your IP for chosen domains, then proxies raw TCP (HTTP/HTTPS) to the real backends.

**Purpose:** This tool is designed to bypass regional restrictions and geo-blocking by routing traffic through your own server located in an unrestricted region. It allows access to AI services that may be blocked or restricted in certain countries.

**Russian version:** [README.ru.md](README.ru.md)

**Author:** [Kakadu Secure Technologies](https://github.com/Matrena-VPN)

---

## What it does

1. **DNS server (UDP :53)**  
   - For configured domain suffixes → returns your server's IP (spoof).  
   - For HTTPS/SVCB records (type 65/64) on spoofed domains → returns NODATA to prevent QUIC/HTTP3 hints and ECH keys.
   - For everything else → forwards to upstream DNS (8.8.8.8, 1.1.1.1 with failover).

2. **TCP proxy (:80, :443)**  
   - Accepts connections to your IP.  
   - Reads SNI (TLS) or `Host` (HTTP), resolves the host via upstream DNS (to avoid loops), then tunnels raw bytes to the real server. No TLS decryption.

3. **UDP sink (:443)**  
   - Listens on UDP 443 and drops all packets (no response).
   - Forces QUIC/HTTP3 connections to fail, making clients fall back to TCP.

Result: clients using your server as DNS get AI service domains pointed at you; your proxy forwards that traffic to the real endpoints via TCP.

---

## Supported domains (default)

| Service | Suffixes |
|---------|----------|
| **OpenAI / ChatGPT** | `.openai.com`, `.chatgpt.com`, `.oaistatic.com`, `.oaiusercontent.com` |
| **Google Gemini** | `.gemini.google.com`, `.aistudio.google.com`, `.ai.google.dev`, `.generativelanguage.googleapis.com`, `.makersuite.google.com` |
| **Cursor IDE** | `.cursor.sh`, `.cursor.com`, `.cursorapi.com`, `.cursor-cdn.com` |
| **Microsoft Copilot** | `.copilot.microsoft.com`, `.bing.com`, `.bingapis.com`, `.edgeservices.bing.com`, `.edgecopilot.microsoft.com` |

---

## QUIC / HTTP3 Handling

Modern browsers (Chrome, Edge, Firefox) and some services use **QUIC (HTTP/3)** over UDP port 443. This bypasses traditional TCP proxies. DnsSpoofer handles this with a multi-layer approach:

### How we force TCP fallback

1. **DNS level:** Block HTTPS/SVCB records (type 65/64) for spoofed domains. These records advertise HTTP/3 support and ECH keys. Without them, clients don't know QUIC is available.

2. **Network level:** UDP sink on port 443 reads and drops all UDP packets. QUIC handshakes fail, forcing clients to fall back to TCP.

3. **Result:** All traffic goes through our TCP proxy where we can read SNI and route correctly.

### Potential issues

| Issue | Description | Solution |
|-------|-------------|----------|
| **DNS over HTTPS (DoH)** | Browsers may use encrypted DNS (8.8.8.8, 1.1.1.1) bypassing your DNS server | Disable DoH in browser settings, or set system DNS to your server |
| **Alt-Svc header** | After first TCP visit, server may advertise QUIC via HTTP header | UDP sink ensures QUIC attempts fail anyway |
| **ECH (Encrypted Client Hello)** | Hides real SNI from proxy | We block HTTPS RR in DNS, so clients don't get ECH keys for our domains |
| **Cached QUIC** | Browser may remember QUIC worked before | UDP sink forces failure; browser falls back to TCP |

### Disabling QUIC in browsers (optional)

If you want to completely disable QUIC on client side:

- **Chrome/Edge:** `chrome://flags` → search "QUIC" → set "Experimental QUIC protocol" to **Disabled**
- **Firefox:** `about:config` → `network.http.http3.enable` → **false**

---

## Requirements

- Go 1.21+
- Linux: ports 53, 80, 443 (or use `CAP_NET_BIND_SERVICE` / run as root for <1024)

---

## Build

```bash
# Local
go build -o dnsspoofer .

# Linux (e.g. Ubuntu server)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dnsspoofer-linux-amd64 .

# Or use Makefile
make build        # local
make build-linux  # linux/amd64
make build-ubuntu # linux/amd64 (alias)

# Or use build script
./scripts/build-ubuntu.sh
```

---

## Usage

```bash
# Default: spoof IP 95.164.123.192, listen on :53, :80, :443
sudo ./dnsspoofer

# Custom spoof IP and ports
./dnsspoofer \
  -spoof-ip=YOUR_SERVER_IP \
  -dns-port=:53 \
  -http-port=:80 \
  -https-port=:443

# Custom domain list (comma-separated suffixes)
./dnsspoofer -spoof-suffixes=".openai.com,.chatgpt.com,.cursor.sh"

# Full flags
./dnsspoofer -h
```

| Flag | Default | Description |
|------|---------|-------------|
| `-spoof-ip` | `95.164.123.192` | IP returned for spoofed domains |
| `-dns-port` | `:53` | DNS listen address |
| `-http-port` | `:80` | HTTP proxy listen address |
| `-https-port` | `:443` | HTTPS proxy listen address (TCP) |
| `-udp-sink-port` | `:443` | UDP sink listen address (drops QUIC/HTTP3 traffic) |
| `-spoof-suffixes` | (see above) | Comma-separated domain suffixes to spoof |
| `-upstream-dns` | `8.8.8.8:53,1.1.1.1:53` | Upstream DNS for non-spoofed + failover |
| `-resolver-dns` | `8.8.8.8:53` | DNS used by proxy to resolve backends (avoids loop) |

---

## Systemd (Linux)

Copy the binary and unit file:

```bash
sudo cp dnsspoofer-linux-amd64 /usr/local/bin/dnsspoofer
sudo chmod +x /usr/local/bin/dnsspoofer
sudo cp dnsspoofer.service /etc/systemd/system/
```

Edit `/etc/systemd/system/dnsspoofer.service` if you need another `-spoof-ip` or ports, then:

```bash
sudo systemctl daemon-reload
sudo systemctl enable dnsspoofer
sudo systemctl start dnsspoofer
sudo systemctl status dnsspoofer
```

Logs: `journalctl -u dnsspoofer -f`

---

## Deployment

### First-time installation (bootstrap)

For a fresh server where DnsSpoofer has never been installed:

```bash
./scripts/bootstrap-server.sh [SERVER]
# Or with force flag to reinstall
./scripts/bootstrap-server.sh --force [SERVER]
```

The bootstrap script will:
- Check for systemd
- Build Linux binary
- Install binary and systemd unit
- Stop and disable systemd-resolved (free port 53)
- Enable and start the service

### Updating existing installation

For servers where DnsSpoofer is already installed:

```bash
./scripts/deploy.sh [SERVER]
# Or use Makefile
make deploy
```

The deploy script will:
- Build Linux binary
- Upload and install binary and unit file
- Restart the service

Both scripts support:
- Command-line argument: `./scripts/deploy.sh root@192.168.1.1`
- Environment variables: `DNS_SPOOFER_SERVER=192.168.1.1 ./scripts/deploy.sh`
- Local config file: `scripts/deploy.local` (gitignored, put credentials there)

See script headers for full usage details.

---

## Debug Tools

The project includes debug utilities located in `scripts/debug/` for troubleshooting and monitoring:

### Fetch Server Debug Log

Fetches detailed debug logs from the server when `DEBUG_LOG_PATH` environment variable is set.

**Setup on server:**
1. Edit `/etc/systemd/system/dnsspoofer.service` and add to `[Service]` section:
   ```ini
   Environment=DEBUG_LOG_PATH=/tmp/dnsspoofer_debug.log
   ```
2. Restart the service: `sudo systemctl restart dnsspoofer`

**Usage:**
```bash
./scripts/debug/fetch-server-debug.sh [output_file]
# Default output: .cursor/debug_server.log
```

The script connects to the server via SSH and downloads the debug log file. Requires `scripts/deploy.local` configuration or SSH key access.

### View Error Logs

Filters and displays errors, connection issues, and tunnel closures from server logs.

**Usage:**
```bash
./scripts/debug/logs-errors.sh [lines]
# Default: last 500 lines
```

Shows filtered output for:
- Errors
- SNI peek timeouts
- Backend dial failures
- Resolve errors
- Tunnel closures
- I/O timeouts

### Check DNS Resolver

Tests DNS resolver behavior for repeated lookups to verify IP rotation.

**Usage:**
```bash
go run scripts/debug/check_resolver.go
```

Checks what IP addresses are returned for multiple DNS queries of the same hostname. Useful for debugging load balancing and IP selection issues.

**Note:** All debug tools require server access via `scripts/deploy.local` configuration or SSH keys.

---

## How it works

- **DNS:** [miekg/dns](https://github.com/miekg/dns) for UDP server and upstream `Exchange()`. Suffix match is case-insensitive; A records are spoofed, AAAA returns empty (force IPv4), HTTPS/SVCB return NODATA (block QUIC hints).
- **SNI:** Peek TLS ClientHello via `crypto/tls` + fake read-only `net.Conn` and `GetConfigForClient`; bytes replayed to backend with `io.TeeReader` / `io.MultiReader`.
- **Proxy:** Resolves backend host with a dedicated resolver pointing at `-resolver-dns` so the host is never resolved via your own DNS (no loop). Then raw `io.Copy` client ↔ backend.
- **UDP Sink:** Simple `net.ListenUDP` that reads and discards all packets. Forces QUIC to fail, triggering TCP fallback.

---

**[Kakadu Secure Technologies](https://github.com/Matrena-VPN)**
