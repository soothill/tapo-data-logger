#!/bin/bash
# Copyright (c) 2025 soothill. All rights reserved.

# Helper script to detect your local subnet for Tapo plug discovery

echo "=== Detecting Your Network Subnet ==="
echo ""

if command -v ip &> /dev/null; then
    # Linux with ip command
    echo "Local Network Interfaces:"
    ip -4 addr show | grep -E "inet " | grep -v "127.0.0.1" | while read -r line; do
        interface=$(echo "$line" | awk '{print $NF}')
        ip_cidr=$(echo "$line" | awk '{print $2}')
        
        echo "  $interface: $ip_cidr"
    done
    
elif command -v ifconfig &> /dev/null; then
    # macOS or Linux with ifconfig
    echo "Local Network Interfaces:"
    ifconfig | grep -E "inet " | grep -v "127.0.0.1" | while read -r line; do
        ip=$(echo "$line" | awk '{print $2}')
        echo "  IP: $ip"
    done
    echo ""
    echo "Common subnets to try:"
    echo "  - 192.168.1.0/24"
    echo "  - 192.168.0.0/24"
    echo "  - 10.0.0.0/24"
fi

echo ""
echo "Add the subnet to your config.json like this:"
echo '  "scan_subnet": "192.168.1.0/24"'
echo ""
echo "Or use the discovery test tool:"
echo "  go run discovery-test.go -method scan -subnet 192.168.1.0/24"

