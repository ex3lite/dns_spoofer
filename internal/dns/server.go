package dns

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// Config holds DNS server configuration
type Config struct {
	ListenAddr      string        // Address to listen on (e.g., ":53")
	SpoofIP         net.IP        // IP to return for spoofed domains
	SpoofSuffixes   []string      // Domain suffixes to spoof (e.g., ".openai.com")
	UpstreamDNS     []string      // Upstream DNS servers (e.g., ["8.8.8.8:53", "1.1.1.1:53"])
	UpstreamTimeout time.Duration // Timeout for upstream queries
}

// Server is a DNS server that spoofs specific domains
type Server struct {
	config     Config
	udpServer  *dns.Server
	client     *dns.Client
	shutdownCh chan struct{}
	wg         sync.WaitGroup
}

// New creates a new DNS server
func New(cfg Config) *Server {
	// Normalize suffixes to lowercase
	suffixes := make([]string, len(cfg.SpoofSuffixes))
	for i, s := range cfg.SpoofSuffixes {
		suffixes[i] = strings.ToLower(s)
	}
	cfg.SpoofSuffixes = suffixes

	if cfg.UpstreamTimeout == 0 {
		cfg.UpstreamTimeout = 5 * time.Second
	}

	return &Server{
		config:     cfg,
		client:     &dns.Client{Timeout: cfg.UpstreamTimeout},
		shutdownCh: make(chan struct{}),
	}
}

// shouldSpoof checks if the domain should be spoofed
func (s *Server) shouldSpoof(name string) bool {
	// Normalize: lowercase and remove trailing dot
	name = strings.ToLower(strings.TrimSuffix(name, "."))

	for _, suffix := range s.config.SpoofSuffixes {
		// Remove leading dot from suffix for comparison
		cleanSuffix := strings.TrimPrefix(suffix, ".")

		// Match exact domain or subdomain
		if name == cleanSuffix || strings.HasSuffix(name, "."+cleanSuffix) {
			return true
		}
	}
	return false
}

// handleRequest handles incoming DNS requests
func (s *Server) handleRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = false

	for _, q := range r.Question {
		log.Printf("[DNS] Query: %s (type %s)", q.Name, dns.TypeToString[q.Qtype])

		if s.shouldSpoof(q.Name) {
			// Spoof A and AAAA records for our domains
			switch q.Qtype {
			case dns.TypeA:
				log.Printf("[DNS] Spoofing %s -> %s", q.Name, s.config.SpoofIP)
				rr := &dns.A{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    60,
					},
					A: s.config.SpoofIP.To4(),
				}
				m.Answer = append(m.Answer, rr)

			case dns.TypeAAAA:
				// Return empty response for AAAA to force IPv4
				// Or return IPv6 if SpoofIP is IPv6
				if ip6 := s.config.SpoofIP.To16(); ip6 != nil && s.config.SpoofIP.To4() == nil {
					rr := &dns.AAAA{
						Hdr: dns.RR_Header{
							Name:   q.Name,
							Rrtype: dns.TypeAAAA,
							Class:  dns.ClassINET,
							Ttl:    60,
						},
						AAAA: ip6,
					}
					m.Answer = append(m.Answer, rr)
				}
				// If SpoofIP is IPv4, just return empty AAAA (no error, just no answer)
				log.Printf("[DNS] Spoofing AAAA %s -> (empty, forcing IPv4)", q.Name)

			default:
				// For other record types, forward to upstream
				s.forwardToUpstream(w, r)
				return
			}
		} else {
			// Forward non-spoofed domains to upstream
			s.forwardToUpstream(w, r)
			return
		}
	}

	if err := w.WriteMsg(m); err != nil {
		log.Printf("[DNS] Error writing response: %v", err)
	}
}

// forwardToUpstream forwards the request to upstream DNS servers
func (s *Server) forwardToUpstream(w dns.ResponseWriter, r *dns.Msg) {
	var lastErr error

	for _, upstream := range s.config.UpstreamDNS {
		log.Printf("[DNS] Forwarding to upstream %s", upstream)

		resp, _, err := s.client.Exchange(r, upstream)
		if err != nil {
			log.Printf("[DNS] Upstream %s error: %v", upstream, err)
			lastErr = err
			continue
		}

		if err := w.WriteMsg(resp); err != nil {
			log.Printf("[DNS] Error writing upstream response: %v", err)
		}
		return
	}

	// All upstreams failed
	log.Printf("[DNS] All upstreams failed, last error: %v", lastErr)
	m := new(dns.Msg)
	m.SetRcode(r, dns.RcodeServerFailure)
	if err := w.WriteMsg(m); err != nil {
		log.Printf("[DNS] Error writing SERVFAIL: %v", err)
	}
}

// Start starts the DNS server
func (s *Server) Start() error {
	s.udpServer = &dns.Server{
		Addr:    s.config.ListenAddr,
		Net:     "udp",
		Handler: dns.HandlerFunc(s.handleRequest),
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log.Printf("[DNS] Starting UDP server on %s", s.config.ListenAddr)
		if err := s.udpServer.ListenAndServe(); err != nil {
			select {
			case <-s.shutdownCh:
				// Expected shutdown
			default:
				log.Printf("[DNS] Server error: %v", err)
			}
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the DNS server
func (s *Server) Shutdown(ctx context.Context) error {
	close(s.shutdownCh)

	if s.udpServer != nil {
		if err := s.udpServer.ShutdownContext(ctx); err != nil {
			return fmt.Errorf("DNS server shutdown: %w", err)
		}
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
