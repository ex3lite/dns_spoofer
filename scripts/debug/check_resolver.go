// One-off script to check what ResolveHost (ips[0]) returns for repeated lookups.
// Run: go run scripts/debug/check_resolver.go
package main

import (
	"context"
	"fmt"
	"net"
	"time"
)

func main() {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, "udp", "8.8.8.8:53")
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hosts := []string{"i.ytimg.com", "googlevideo.com", "discord.com"}
	for _, host := range hosts {
		fmt.Printf("\n--- %s ---\n", host)
		ips, err := resolver.LookupHost(ctx, host)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}
		fmt.Printf("total IPs: %d\n", len(ips))
		fmt.Printf("ips[0] (current proxy): %s\n", ips[0])
		// Simulate 5 connections: each would get ips[0] with current code
		for i := 0; i < 5; i++ {
			ips2, _ := resolver.LookupHost(ctx, host)
			if len(ips2) > 0 {
				fmt.Printf("  connection %d -> %s\n", i+1, ips2[0])
			}
		}
	}
}
