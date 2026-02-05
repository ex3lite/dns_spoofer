# DnsSpoofer

**DNS relay with selective spoofing + transparent TCP proxy** for redirecting AI service traffic (OpenAI, ChatGPT, Google Gemini, Cursor, YouTube) through your own server. One binary: DNS answers with your IP for chosen domains, then proxies raw TCP (HTTP/HTTPS) to the real backends.

**Author:** [Kakadu Secure Technologies](https://github.com/kakadu-secure)

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

Result: clients using your server as DNS get AI/YouTube domains pointed at you; your proxy forwards that traffic to the real endpoints via TCP.

---

## Supported domains (default)

| Service | Suffixes |
|---------|----------|
| **OpenAI / ChatGPT** | `.openai.com`, `.chatgpt.com`, `.oaistatic.com`, `.oaiusercontent.com` |
| **Google Gemini** | `.gemini.google.com`, `.aistudio.google.com`, `.ai.google.dev`, `.generativelanguage.googleapis.com`, `.makersuite.google.com` |
| **Cursor IDE** | `.cursor.sh`, `.cursor.com`, `.cursorapi.com`, `.cursor-cdn.com` |
| **YouTube** | `.youtube.com`, `.ytimg.com`, `.googlevideo.com`, `.youtube-nocookie.com`, `.youtu.be` |

---

## QUIC / HTTP3 Handling

Modern browsers (Chrome, Edge, Firefox) and services like YouTube use **QUIC (HTTP/3)** over UDP port 443. This bypasses traditional TCP proxies. DnsSpoofer handles this with a multi-layer approach:

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

## How it works

- **DNS:** [miekg/dns](https://github.com/miekg/dns) for UDP server and upstream `Exchange()`. Suffix match is case-insensitive; A records are spoofed, AAAA returns empty (force IPv4), HTTPS/SVCB return NODATA (block QUIC hints).
- **SNI:** Peek TLS ClientHello via `crypto/tls` + fake read-only `net.Conn` and `GetConfigForClient`; bytes replayed to backend with `io.TeeReader` / `io.MultiReader`.
- **Proxy:** Resolves backend host with a dedicated resolver pointing at `-resolver-dns` so the host is never resolved via your own DNS (no loop). Then raw `io.Copy` client ↔ backend.
- **UDP Sink:** Simple `net.ListenUDP` that reads and discards all packets. Forces QUIC to fail, triggering TCP fallback.

---

**Kakadu Secure Technologies**
