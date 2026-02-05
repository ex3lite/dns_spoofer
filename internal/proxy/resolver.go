package proxy

import (
	"context"
	"net"
	"time"
)

// NewResolver creates a custom DNS resolver that queries the specified DNS server.
// This is crucial to avoid loops when the system DNS is pointing to our own DNS server.
func NewResolver(dnsServer string, timeout time.Duration) *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: timeout,
			}
			return d.DialContext(ctx, "udp", dnsServer)
		},
	}
}

// ResolveHost resolves a hostname to IP addresses using the provided resolver.
// Returns the first resolved IP address.
func ResolveHost(ctx context.Context, resolver *net.Resolver, host string) (string, error) {
	ips, err := resolver.LookupHost(ctx, host)
	if err != nil {
		return "", err
	}
	if len(ips) == 0 {
		return "", &net.DNSError{Err: "no addresses found", Name: host}
	}
	return ips[0], nil
}
