//go:build ignore
// +build ignore

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/grandcat/zeroconf"
)

func main() {
	method := flag.String("method", "mdns", "Discovery method: mdns, scan, or both")
	subnet := flag.String("subnet", "192.168.1.0/24", "Subnet to scan (for scan method)")
	timeout := flag.Duration("timeout", 5*time.Second, "Discovery timeout")
	flag.Parse()

	fmt.Printf("Discovering Tapo plugs using method: %s\n", *method)
	fmt.Printf("Timeout: %v\n\n", *timeout)

	var plugs []string
	var err error

	switch *method {
	case "mdns":
		plugs, err = discoverMDNS(*timeout)
	case "scan":
		fmt.Printf("Scanning subnet: %s\n", *subnet)
		plugs, err = discoverScan(*subnet, *timeout)
	case "both":
		plugs, err = discoverBoth(*subnet, *timeout)
	default:
		log.Fatalf("Unknown method: %s", *method)
	}

	if err != nil {
		log.Fatalf("Discovery failed: %v", err)
	}

	if len(plugs) == 0 {
		fmt.Println("No Tapo plugs found")
		return
	}

	fmt.Printf("\nFound %d plug(s):\n", len(plugs))
	for i, ip := range plugs {
		fmt.Printf("  %d. %s\n", i+1, ip)
	}

	// Output as JSON for easy config generation
	jsonOutput := map[string]interface{}{
		"plug_ips": plugs,
	}
	jsonBytes, _ := json.MarshalIndent(jsonOutput, "", "  ")
	fmt.Printf("\nAdd to config.json:\n%s\n", string(jsonBytes))
}

func discoverMDNS(timeout time.Duration) ([]string, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	var plugIPs []string

	go func() {
		for entry := range entries {
			fmt.Printf("Found mDNS service: %s\n", entry.Instance)
			if len(entry.AddrIPv4) > 0 {
				ip := entry.AddrIPv4[0].String()
				plugIPs = append(plugIPs, ip)
				fmt.Printf("  IP: %s\n", ip)
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	serviceTypes := []string{"_hap._tcp", "_http._tcp"}
	for _, serviceType := range serviceTypes {
		if err := resolver.Browse(ctx, serviceType, "local.", entries); err != nil {
			log.Printf("Warning: Failed to browse %s: %v", serviceType, err)
		}
	}

	<-ctx.Done()
	close(entries)

	return plugIPs, nil
}

func discoverScan(subnet string, timeout time.Duration) ([]string, error) {
	// This is a simplified version - the full implementation is in main.go
	fmt.Println("Network scanning not implemented in this test tool")
	fmt.Println("Use the main application with discovery_method: 'scan'")
	return []string{}, nil
}

func discoverBoth(subnet string, timeout time.Duration) ([]string, error) {
	fmt.Println("Trying mDNS first...")
	plugs, err := discoverMDNS(timeout)
	if err != nil {
		return nil, err
	}
	fmt.Printf("mDNS found %d plug(s)\n", len(plugs))
	return plugs, nil
}
