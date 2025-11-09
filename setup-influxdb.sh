#!/bin/bash
# Copyright (c) 2025 Darren Soothill. All rights reserved.

# InfluxDB Setup Script for Tapo Energy Logger
# This script helps set up InfluxDB for the Tapo energy monitoring application

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
DEFAULT_BUCKET="tapo_energy"
DEFAULT_ORG="home"
DEFAULT_URL="http://localhost:8086"

echo -e "${BLUE}=== InfluxDB Setup for Tapo Energy Logger ===${NC}"
echo ""

# Function to check if influx CLI is installed
check_influx_cli() {
    if ! command -v influx &> /dev/null; then
        echo -e "${RED}Error: InfluxDB CLI (influx) not found${NC}"
        echo ""
        echo "Please install InfluxDB 2.x first:"
        echo ""
        echo "Ubuntu/Debian:"
        echo "  wget -q https://repos.influxdata.com/influxdata-archive_compat.key"
        echo "  echo '393e8779c89ac8d958f81f942f9ad7fb82a25e133faddaf92e15b16e6ac9ce4c influxdata-archive_compat.key' | sha256sum -c"
        echo "  cat influxdata-archive_compat.key | gpg --dearmor | sudo tee /etc/apt/trusted.gpg.d/influxdata-archive_compat.gpg > /dev/null"
        echo "  echo 'deb [signed-by=/etc/apt/trusted.gpg.d/influxdata-archive_compat.gpg] https://repos.influxdata.com/debian stable main' | sudo tee /etc/apt/sources.list.d/influxdata.list"
        echo "  sudo apt-get update && sudo apt-get install influxdb2 influxdb2-cli"
        echo ""
        echo "macOS:"
        echo "  brew install influxdb influxdb-cli"
        echo ""
        echo "Or download from: https://portal.influxdata.com/downloads/"
        echo ""
        exit 1
    fi
}

# Function to check if InfluxDB is running
check_influxdb_running() {
    local url=$1
    if curl -s "${url}/health" > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Function to check if InfluxDB is already set up
check_influxdb_setup() {
    local url=$1
    local response=$(curl -s "${url}/api/v2/setup")
    if echo "$response" | grep -q '"allowed":false'; then
        return 0  # Already set up
    else
        return 1  # Not set up
    fi
}

# Function to perform initial InfluxDB setup
initial_setup() {
    local url=$1
    local org=$2
    local bucket=$3

    echo -e "${YELLOW}Performing initial InfluxDB setup...${NC}"
    echo ""

    # Prompt for username and password
    read -p "Enter admin username [admin]: " username
    username=${username:-admin}

    read -sp "Enter admin password (min 8 characters): " password
    echo ""

    if [ ${#password} -lt 8 ]; then
        echo -e "${RED}Error: Password must be at least 8 characters${NC}"
        exit 1
    fi

    read -sp "Confirm password: " password_confirm
    echo ""

    if [ "$password" != "$password_confirm" ]; then
        echo -e "${RED}Error: Passwords don't match${NC}"
        exit 1
    fi

    # Retention period
    echo ""
    echo "Data retention period:"
    echo "  1) 7 days"
    echo "  2) 30 days"
    echo "  3) 90 days"
    echo "  4) 1 year"
    echo "  5) Infinite"
    read -p "Select retention period [5]: " retention_choice
    retention_choice=${retention_choice:-5}

    case $retention_choice in
        1) retention="604800" ;;      # 7 days in seconds
        2) retention="2592000" ;;     # 30 days
        3) retention="7776000" ;;     # 90 days
        4) retention="31536000" ;;    # 1 year
        5) retention="0" ;;           # Infinite
        *) retention="0" ;;
    esac

    # Create setup payload
    local setup_payload=$(cat <<EOF
{
  "username": "$username",
  "password": "$password",
  "org": "$org",
  "bucket": "$bucket",
  "retentionPeriodSeconds": $retention
}
EOF
)

    # Execute setup
    local response=$(curl -s -X POST "${url}/api/v2/setup" \
        -H "Content-Type: application/json" \
        -d "$setup_payload")

    if echo "$response" | grep -q '"auth"'; then
        echo -e "${GREEN}✓ InfluxDB initial setup complete${NC}"

        # Extract token from response
        local token=$(echo "$response" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

        echo ""
        echo -e "${GREEN}Setup successful!${NC}"
        echo ""
        echo "Credentials:"
        echo "  Username: $username"
        echo "  Organization: $org"
        echo "  Bucket: $bucket"
        echo "  Token: $token"

        # Save config
        influx config create \
            --config-name tapo-logger \
            --host-url "$url" \
            --org "$org" \
            --token "$token" \
            --active > /dev/null 2>&1 || true

        echo "$token"
    else
        echo -e "${RED}Error during setup:${NC}"
        echo "$response"
        exit 1
    fi
}

# Function to create bucket in existing setup
create_bucket() {
    local url=$1
    local org=$2
    local bucket=$3
    local token=$4

    echo -e "${YELLOW}Creating bucket '$bucket'...${NC}"

    # Check if bucket already exists
    local existing=$(curl -s "${url}/api/v2/buckets?name=${bucket}" \
        -H "Authorization: Token ${token}")

    if echo "$existing" | grep -q "\"name\":\"${bucket}\""; then
        echo -e "${YELLOW}Warning: Bucket '$bucket' already exists${NC}"
        return 0
    fi

    # Get org ID
    local org_id=$(curl -s "${url}/api/v2/orgs?org=${org}" \
        -H "Authorization: Token ${token}" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

    if [ -z "$org_id" ]; then
        echo -e "${RED}Error: Organization '$org' not found${NC}"
        exit 1
    fi

    # Ask for retention
    echo ""
    echo "Data retention period:"
    echo "  1) 7 days"
    echo "  2) 30 days"
    echo "  3) 90 days"
    echo "  4) 1 year"
    echo "  5) Infinite"
    read -p "Select retention period [5]: " retention_choice
    retention_choice=${retention_choice:-5}

    case $retention_choice in
        1) retention="604800" ;;
        2) retention="2592000" ;;
        3) retention="7776000" ;;
        4) retention="31536000" ;;
        5) retention="0" ;;
        *) retention="0" ;;
    esac

    # Create bucket
    local bucket_payload=$(cat <<EOF
{
  "orgID": "$org_id",
  "name": "$bucket",
  "retentionRules": [
    {
      "type": "expire",
      "everySeconds": $retention
    }
  ]
}
EOF
)

    local response=$(curl -s -X POST "${url}/api/v2/buckets" \
        -H "Authorization: Token ${token}" \
        -H "Content-Type: application/json" \
        -d "$bucket_payload")

    if echo "$response" | grep -q "\"name\":\"${bucket}\""; then
        echo -e "${GREEN}✓ Bucket created successfully${NC}"
    else
        echo -e "${RED}Error creating bucket:${NC}"
        echo "$response"
        exit 1
    fi
}

# Function to create API token
create_token() {
    local url=$1
    local org=$2
    local bucket=$3
    local token=$4

    echo -e "${YELLOW}Creating API token for bucket '$bucket'...${NC}"

    # Get org ID
    local org_id=$(curl -s "${url}/api/v2/orgs?org=${org}" \
        -H "Authorization: Token ${token}" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

    # Get bucket ID
    local bucket_id=$(curl -s "${url}/api/v2/buckets?name=${bucket}" \
        -H "Authorization: Token ${token}" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

    if [ -z "$bucket_id" ]; then
        echo -e "${RED}Error: Bucket '$bucket' not found${NC}"
        exit 1
    fi

    # Create token with write permissions
    local token_payload=$(cat <<EOF
{
  "description": "Tapo Energy Logger Token",
  "orgID": "$org_id",
  "permissions": [
    {
      "action": "write",
      "resource": {
        "type": "buckets",
        "id": "$bucket_id",
        "orgID": "$org_id"
      }
    },
    {
      "action": "read",
      "resource": {
        "type": "buckets",
        "id": "$bucket_id",
        "orgID": "$org_id"
      }
    }
  ]
}
EOF
)

    local response=$(curl -s -X POST "${url}/api/v2/authorizations" \
        -H "Authorization: Token ${token}" \
        -H "Content-Type: application/json" \
        -d "$token_payload")

    if echo "$response" | grep -q '"token"'; then
        local new_token=$(echo "$response" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
        echo -e "${GREEN}✓ API token created successfully${NC}"
        echo "$new_token"
    else
        echo -e "${RED}Error creating token:${NC}"
        echo "$response"
        exit 1
    fi
}

# Function to update config.json
update_config() {
    local url=$1
    local org=$2
    local bucket=$3
    local token=$4

    if [ ! -f "config.json" ]; then
        echo -e "${YELLOW}config.json not found. Creating from example...${NC}"
        if [ -f "config.example.json" ]; then
            cp config.example.json config.json
        else
            echo -e "${RED}Error: config.example.json not found${NC}"
            return 1
        fi
    fi

    # Update config.json using sed (works on both Linux and macOS)
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        sed -i '' "s|\"influx_url\":.*|\"influx_url\": \"$url\",|" config.json
        sed -i '' "s|\"influx_token\":.*|\"influx_token\": \"$token\",|" config.json
        sed -i '' "s|\"influx_org\":.*|\"influx_org\": \"$org\",|" config.json
        sed -i '' "s|\"influx_bucket\":.*|\"influx_bucket\": \"$bucket\",|" config.json
    else
        # Linux
        sed -i "s|\"influx_url\":.*|\"influx_url\": \"$url\",|" config.json
        sed -i "s|\"influx_token\":.*|\"influx_token\": \"$token\",|" config.json
        sed -i "s|\"influx_org\":.*|\"influx_org\": \"$org\",|" config.json
        sed -i "s|\"influx_bucket\":.*|\"influx_bucket\": \"$bucket\",|" config.json
    fi

    echo -e "${GREEN}✓ config.json updated${NC}"
}

# Main script logic
main() {
    check_influx_cli

    # Get InfluxDB URL
    read -p "Enter InfluxDB URL [$DEFAULT_URL]: " INFLUX_URL
    INFLUX_URL=${INFLUX_URL:-$DEFAULT_URL}

    # Check if InfluxDB is running
    echo ""
    echo -e "${YELLOW}Checking InfluxDB connection...${NC}"
    if ! check_influxdb_running "$INFLUX_URL"; then
        echo -e "${RED}Error: Cannot connect to InfluxDB at $INFLUX_URL${NC}"
        echo ""
        echo "Make sure InfluxDB is running:"
        echo "  sudo systemctl start influxdb  # Linux"
        echo "  brew services start influxdb   # macOS"
        echo ""
        exit 1
    fi
    echo -e "${GREEN}✓ Connected to InfluxDB${NC}"

    # Check if initial setup is needed
    echo ""
    if ! check_influxdb_setup "$INFLUX_URL"; then
        echo -e "${YELLOW}InfluxDB has not been set up yet.${NC}"
        echo ""

        read -p "Enter organization name [$DEFAULT_ORG]: " ORG
        ORG=${ORG:-$DEFAULT_ORG}

        read -p "Enter bucket name [$DEFAULT_BUCKET]: " BUCKET
        BUCKET=${BUCKET:-$DEFAULT_BUCKET}

        TOKEN=$(initial_setup "$INFLUX_URL" "$ORG" "$BUCKET")
    else
        echo -e "${GREEN}✓ InfluxDB is already set up${NC}"
        echo ""

        # Ask for existing credentials or create new
        echo "Options:"
        echo "  1) Use existing token"
        echo "  2) Create new bucket and token"
        read -p "Select option [1]: " option
        option=${option:-1}

        if [ "$option" = "1" ]; then
            read -p "Enter organization name [$DEFAULT_ORG]: " ORG
            ORG=${ORG:-$DEFAULT_ORG}

            read -p "Enter bucket name [$DEFAULT_BUCKET]: " BUCKET
            BUCKET=${BUCKET:-$DEFAULT_BUCKET}

            read -p "Enter existing API token: " TOKEN

            # Verify token works
            if ! curl -s "${INFLUX_URL}/api/v2/buckets" -H "Authorization: Token ${TOKEN}" > /dev/null; then
                echo -e "${RED}Error: Invalid token or connection failed${NC}"
                exit 1
            fi

            echo -e "${GREEN}✓ Token verified${NC}"
        else
            read -p "Enter your admin API token: " ADMIN_TOKEN

            read -p "Enter organization name [$DEFAULT_ORG]: " ORG
            ORG=${ORG:-$DEFAULT_ORG}

            read -p "Enter bucket name [$DEFAULT_BUCKET]: " BUCKET
            BUCKET=${BUCKET:-$DEFAULT_BUCKET}

            # Create bucket if it doesn't exist
            create_bucket "$INFLUX_URL" "$ORG" "$BUCKET" "$ADMIN_TOKEN"

            # Create new token
            TOKEN=$(create_token "$INFLUX_URL" "$ORG" "$BUCKET" "$ADMIN_TOKEN")
        fi
    fi

    # Display configuration
    echo ""
    echo -e "${GREEN}=== Configuration ===${NC}"
    echo ""
    echo "InfluxDB URL: $INFLUX_URL"
    echo "Organization: $ORG"
    echo "Bucket: $BUCKET"
    echo "API Token: $TOKEN"
    echo ""

    # Ask to update config.json
    read -p "Update config.json with these values? (y/n) [y]: " update
    update=${update:-y}

    if [ "$update" = "y" ] || [ "$update" = "Y" ]; then
        update_config "$INFLUX_URL" "$ORG" "$BUCKET" "$TOKEN"
        echo ""
        echo -e "${GREEN}Setup complete!${NC}"
        echo ""
        echo "Next steps:"
        echo "  1. Update your Tapo credentials in config.json"
        echo "  2. Run: go run main.go"
    else
        echo ""
        echo "Add these values to your config.json manually:"
        echo ""
        echo "  \"influx_url\": \"$INFLUX_URL\","
        echo "  \"influx_token\": \"$TOKEN\","
        echo "  \"influx_org\": \"$ORG\","
        echo "  \"influx_bucket\": \"$BUCKET\""
    fi

    echo ""
}

main "$@"
