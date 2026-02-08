package udpsink

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// Config holds UDP sink configuration
type Config struct {
	ListenAddr string // Address to listen on (e.g., ":443")
}

// Sink is a UDP listener that drops all incoming packets
// Used to explicitly "sink" QUIC traffic on port 443, forcing clients to fall back to TCP
// Optionally sends ICMP Port Unreachable to speed up TCP fallback
type Sink struct {
	config     Config
	conn       *net.UDPConn
	icmpConn   *icmp.PacketConn // For sending ICMP Port Unreachable
	shutdownCh chan struct{}
	wg         sync.WaitGroup
	dropped    atomic.Uint64 // Counter for dropped packets
	icmpSent   atomic.Uint64 // Counter for ICMP responses sent
}

// New creates a new UDP sink
func New(cfg Config) *Sink {
	return &Sink{
		config:     cfg,
		shutdownCh: make(chan struct{}),
	}
}

// Start starts the UDP sink listener (IPv4 only)
func (s *Sink) Start() error {
	// Bind IPv4 only so we don't listen on IPv6
	host, port, err := net.SplitHostPort(s.config.ListenAddr)
	if err != nil || port == "" {
		port = "443"
		host = ""
	}
	if host == "" || host == "::" {
		host = "0.0.0.0"
	}
	addr, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(host, port))
	if err != nil {
		return fmt.Errorf("resolve UDP address: %w", err)
	}

	s.conn, err = net.ListenUDP("udp4", addr)
	if err != nil {
		return fmt.Errorf("listen UDP: %w", err)
	}

	// Try to create ICMP connection for sending Port Unreachable (requires CAP_NET_RAW or root)
	// If this fails, we'll just drop packets silently (slower fallback but still works)
	s.icmpConn, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		log.Printf("[UDPSink] Warning: Cannot create ICMP socket (need CAP_NET_RAW or root): %v. Will drop packets silently (slower TCP fallback)", err)
		s.icmpConn = nil
	} else {
		log.Printf("[UDPSink] ICMP Port Unreachable enabled (fast TCP fallback)")
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.readLoop()
	}()

	log.Printf("[UDPSink] Started on %s (dropping all QUIC/UDP traffic)", s.config.ListenAddr)
	return nil
}

// readLoop reads and discards all incoming UDP packets
func (s *Sink) readLoop() {
	// Buffer for reading packets - QUIC Initial packets can be up to ~1200 bytes
	// but we don't care about the content, just need to read and discard
	buf := make([]byte, 2048)

	for {
		select {
		case <-s.shutdownCh:
			return
		default:
		}

		// Read packet (blocking, but conn.Close() will unblock it)
		n, remoteAddr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-s.shutdownCh:
				// Expected shutdown
				return
			default:
				// Check if it's a "use of closed network connection" error
				if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
					return
				}
				log.Printf("[UDPSink] Read error: %v", err)
				continue
			}
		}

		// Send ICMP Port Unreachable if ICMP socket is available (faster TCP fallback)
		// Otherwise just drop silently (slower fallback but still works)
		if s.icmpConn != nil {
			if err := s.sendICMPPortUnreachable(remoteAddr, buf[:n]); err != nil {
				// Log error but continue - ICMP is best-effort
				if s.icmpSent.Load() < 5 {
					log.Printf("[UDPSink] ICMP send error: %v", err)
				}
			} else {
				s.icmpSent.Add(1)
			}
		}

		count := s.dropped.Add(1)
		if count%1000 == 0 || count <= 10 {
			// Log first 10 packets and then every 1000th to avoid log spam
			if s.icmpConn != nil {
				log.Printf("[UDPSink] Dropped packet #%d from %s (%d bytes), ICMP sent: %d", count, remoteAddr, n, s.icmpSent.Load())
			} else {
				log.Printf("[UDPSink] Dropped packet #%d from %s (%d bytes)", count, remoteAddr, n)
			}
		}
	}
}

// sendICMPPortUnreachable sends an ICMP Port Unreachable message to the source.
// This helps clients detect that the port is closed immediately, speeding up TCP fallback.
func (s *Sink) sendICMPPortUnreachable(remoteAddr *net.UDPAddr, originalPacket []byte) error {
	if s.icmpConn == nil {
		return fmt.Errorf("ICMP socket not available")
	}

	// Get local IP from the UDP connection
	localAddr := s.conn.LocalAddr().(*net.UDPAddr)
	localIP := localAddr.IP.To4()
	if localIP == nil {
		return fmt.Errorf("local IP is not IPv4")
	}

	// Construct IP header (20 bytes) + UDP header (8 bytes) for ICMP error payload
	// The payload should contain the original packet's IP header as it was received
	// IP header: version(4) + IHL(5) + TOS(0) + Total Length + ID + Flags + TTL + Protocol(17=UDP) + Checksum + Source + Dest
	ipHeader := make([]byte, 20)
	ipHeader[0] = 0x45 // Version 4, IHL 5
	ipHeader[1] = 0x00 // TOS
	// Total length: 20 (IP) + 8 (UDP) + len(originalPacket)
	totalLen := 28 + len(originalPacket)
	ipHeader[2] = byte(totalLen >> 8)
	ipHeader[3] = byte(totalLen)
	ipHeader[8] = 64                           // TTL
	ipHeader[9] = 17                           // Protocol: UDP
	copy(ipHeader[12:16], remoteAddr.IP.To4()) // Source IP (client IP - original packet source)
	copy(ipHeader[16:20], localIP)             // Dest IP (our IP - original packet destination)

	// UDP header: Source Port + Dest Port + Length + Checksum
	// Ports as they were in the original packet (client -> our port 443)
	udpHeader := make([]byte, 8)
	udpHeader[0] = byte(remoteAddr.Port >> 8)
	udpHeader[1] = byte(remoteAddr.Port)
	udpHeader[2] = byte(localAddr.Port >> 8)
	udpHeader[3] = byte(localAddr.Port)
	udpLen := 8 + len(originalPacket)
	udpHeader[4] = byte(udpLen >> 8)
	udpHeader[5] = byte(udpLen)
	// Checksum can be 0 for ICMP error payload

	// ICMP error payload: IP header + UDP header + first 8 bytes of original packet
	payload := append(ipHeader, udpHeader...)
	payload = append(payload, originalPacket[:min(8, len(originalPacket))]...)

	// Create ICMP Port Unreachable message
	msg := &icmp.Message{
		Type: ipv4.ICMPTypeDestinationUnreachable,
		Code: 3, // Port Unreachable
		Body: &icmp.DstUnreach{
			Data: payload,
		},
	}

	// Marshal and send
	msgBytes, err := msg.Marshal(nil)
	if err != nil {
		return fmt.Errorf("marshal ICMP: %w", err)
	}

	_, err = s.icmpConn.WriteTo(msgBytes, &net.IPAddr{IP: remoteAddr.IP})
	return err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// DroppedCount returns the number of dropped packets
func (s *Sink) DroppedCount() uint64 {
	return s.dropped.Load()
}

// ICMPCount returns the number of ICMP Port Unreachable messages sent
func (s *Sink) ICMPCount() uint64 {
	return s.icmpSent.Load()
}

// Shutdown gracefully shuts down the UDP sink
func (s *Sink) Shutdown(ctx context.Context) error {
	close(s.shutdownCh)

	if s.conn != nil {
		s.conn.Close()
	}
	if s.icmpConn != nil {
		s.icmpConn.Close()
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if s.icmpConn != nil {
			log.Printf("[UDPSink] Shutdown complete (dropped %d packets, sent %d ICMP Port Unreachable)", s.dropped.Load(), s.icmpSent.Load())
		} else {
			log.Printf("[UDPSink] Shutdown complete (dropped %d packets total)", s.dropped.Load())
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
