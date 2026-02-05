# DnsSpoofer

**DNS relay with selective spoofing + transparent TCP proxy** for redirecting AI service traffic (OpenAI, ChatGPT, Google Gemini, Cursor) through your own server. One binary: DNS answers with your IP for chosen domains, then proxies raw TCP (HTTP/HTTPS) to the real backends.

**Author:** [Kakadu Secure Technologies](https://github.com/kakadu-secure)

---

## What it does

1. **DNS server (UDP :53)**  
   - For configured domain suffixes → returns your server’s IP (spoof).  
   - For everything else → forwards to upstream DNS (8.8.8.8, 1.1.1.1 with failover).

2. **TCP proxy (:80, :443)**  
   - Accepts connections to your IP.  
   - Reads SNI (TLS) or `Host` (HTTP), resolves the host via upstream DNS (to avoid loops), then tunnels raw bytes to the real server. No TLS decryption.

Result: clients using your server as DNS get AI domains pointed at you; your proxy forwards that traffic to the real OpenAI/Gemini/Cursor endpoints.

---

## Supported domains (default)

| Service   | Suffixes |
|----------|----------|
| **OpenAI / ChatGPT** | `.openai.com`, `.chatgpt.com`, `.oaistatic.com`, `.oaiusercontent.com` |
| **Google Gemini**    | `.gemini.google.com`, `.aistudio.google.com`, `.ai.google.dev`, `.generativelanguage.googleapis.com`, `.makersuite.google.com` |
| **Cursor IDE**       | `.cursor.sh`, `.cursor.com`, `.cursorapi.com`, `.cursor-cdn.com` |

---

## Requirements

- Go 1.21+
- Linux: ports 53, 80, 443 (or use `CAP_NET_BIND_SERVICE` / run as root for &lt;1024)

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
| `-https-port` | `:443` | HTTPS proxy listen address |
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

- **DNS:** [miekg/dns](https://github.com/miekg/dns) for UDP server and upstream `Exchange()`. Suffix match is case-insensitive; A (and optionally AAAA) are spoofed.
- **SNI:** Peek TLS ClientHello via `crypto/tls` + fake read-only `net.Conn` and `GetConfigForClient`; bytes replayed to backend with `io.TeeReader` / `io.MultiReader`.
- **Proxy:** Resolves backend host with a dedicated resolver pointing at `-resolver-dns` so the host is never resolved via your own DNS (no loop). Then raw `io.Copy` client ↔ backend.

---

**Kakadu Secure Technologies**
