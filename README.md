# Tapo Energy Monitor to InfluxDB

A Go application that collects power consumption data from TP-Link Tapo smart plugs (P110, P115, etc.) and logs it to InfluxDB for monitoring and analysis.

## Features

- **Automatic plug discovery** via mDNS/Bonjour or network scanning
- Real-time power consumption monitoring (watts)
- Daily and monthly energy usage tracking (kWh)
- Runtime statistics (hours)
- Multiple plug support with automatic re-discovery
- Configurable polling interval
- Concurrent data collection from multiple plugs
- **Device offline detection** with Slack notifications
- Automatic recovery notifications when devices come back online
- **Mobile-responsive Web UI** for real-time monitoring on iPhone/Android
- REST API for device data access

## Prerequisites

- Go 1.24 or later
- InfluxDB 2.x instance
- TP-Link Tapo smart plugs with energy monitoring (P110, P115, etc.)
- Tapo account credentials

## Installation

### Option 1: Docker (Recommended)

The easiest way to get started is with Docker. Pre-built images are available on GitHub Container Registry:

```bash
# Pull the latest image
docker pull ghcr.io/soothill/tapo-data-logger:latest

# Run with your config
docker run -d \
  --name tapo-logger \
  -v $(pwd)/config.json:/app/config.json:ro \
  -p 8080:8080 \
  ghcr.io/soothill/tapo-data-logger:latest
```

For full Docker deployment options including Docker Compose with InfluxDB and Grafana, see [DOCKER.md](DOCKER.md).

### Option 2: Build from Source

1. Clone or download this repository

2. Install dependencies:
```bash
go mod download
```

3. Create your configuration file:
```bash
cp config.example.json config.json
```

4. Edit `config.json` with your settings:
```json
{
  "tapo_email": "your-tapo-account@example.com",
  "tapo_password": "your-tapo-password",
  "plug_ips": [],
  "auto_discover": true,
  "discovery_method": "both",
  "scan_subnet": "192.168.1.0/24",
  "influx_url": "http://localhost:8086",
  "influx_token": "your-influxdb-token",
  "influx_org": "your-org",
  "influx_bucket": "tapo_energy",
  "poll_interval_seconds": 60,
  "slack_webhook_url": "",
  "alerts_enabled": false,
  "alert_after_failures": 3,
  "webui_enabled": true,
  "webui_port": 8080,
  "webui_host": "0.0.0.0"
}
```

## Configuration

### Auto-Discovery Options

The application can automatically discover Tapo plugs on your network using three methods:

**1. mDNS/Bonjour Discovery** (`"discovery_method": "mdns"`)
- Discovers plugs advertising via mDNS/Bonjour
- Fast and efficient
- Works best if your network supports multicast DNS
- No additional configuration needed

**2. Network Scanning** (`"discovery_method": "scan"`)
- Scans your subnet for Tapo devices
- More reliable on networks with mDNS issues
- Requires `scan_subnet` to be configured (e.g., "192.168.1.0/24")
- Takes longer but finds all accessible plugs

**3. Both Methods** (`"discovery_method": "both"`)
- Tries mDNS first, then falls back to scanning
- Recommended for most setups
- Provides the best chance of finding all devices

### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `auto_discover` | Enable automatic plug discovery | `false` |
| `discovery_method` | Discovery method: "mdns", "scan", or "both" | `"both"` |
| `scan_subnet` | Subnet to scan (CIDR notation) | `""` |
| `plug_ips` | Manually specify plug IPs (optional if auto-discovery is enabled) | `[]` |
| `alerts_enabled` | Enable Slack alerts for offline devices | `false` |
| `slack_webhook_url` | Slack webhook URL for notifications | `""` |
| `alert_after_failures` | Number of consecutive failures before alerting | `3` |
| `webui_enabled` | Enable web UI for real-time monitoring | `false` |
| `webui_port` | Web UI HTTP port | `8080` |
| `webui_host` | Web UI bind address | `"0.0.0.0"` |

**Notes:**
- You can combine auto-discovery with manual IPs - both will be monitored
- The app re-discovers plugs every hour to find new devices
- If a plug becomes unreachable, it will be tracked and can trigger alerts if enabled

### Finding Your Subnet

To determine your subnet for scanning:

```bash
# Linux/Mac
ip addr show | grep inet

# Or use your router's subnet, typically:
# 192.168.1.0/24 or 192.168.0.0/24 or 10.0.0.0/24
```

### Manual IP Configuration (Alternative to Auto-Discovery)

You can find the IP addresses of your Tapo plugs in several ways:
- Check your router's DHCP client list
- Use the Tapo app (Device Settings → Device Info)
- Use network scanning tools like `nmap` or `arp-scan`

It's recommended to assign static IP addresses to your plugs in your router's DHCP settings.

### Slack Alerts Configuration

The application can send Slack notifications when devices go offline or come back online.

**Setting up Slack alerts:**

1. Create a Slack webhook:
   - Go to https://api.slack.com/apps
   - Create a new app or select an existing one
   - Navigate to "Incoming Webhooks"
   - Activate incoming webhooks and create a new webhook URL
   - Copy the webhook URL

2. Configure alerts in `config.json`:
```json
{
  "alerts_enabled": true,
  "slack_webhook_url": "https://hooks.slack.com/services/YOUR/WEBHOOK/URL",
  "alert_after_failures": 3
}
```

**How it works:**
- The application tracks consecutive polling failures for each device
- After N consecutive failures (default: 3), an **offline alert** is sent to Slack
- When the device comes back online, a **recovery notification** is sent
- This prevents false alerts from temporary network glitches

**Alert messages include:**
- Device IP address
- Status (OFFLINE or ONLINE)
- Duration of downtime (for recovery notifications)
- Number of failed poll attempts
- Timestamp

### Web UI Configuration

The application includes a built-in mobile-responsive web interface for real-time monitoring of your Tapo devices.

**Enable Web UI:**
```json
{
  "webui_enabled": true,
  "webui_port": 8080,
  "webui_host": "0.0.0.0"
}
```

**Configuration Options:**
- `webui_enabled`: Enable/disable the web UI (default: `false`)
- `webui_port`: HTTP port for the web server (default: `8080`)
- `webui_host`: Bind address for the web server (default: `"0.0.0.0"` - all interfaces)

**Access the Web UI:**

Once enabled, access the web interface at:
- From the same machine: `http://localhost:8080`
- From mobile/other devices: `http://YOUR_SERVER_IP:8080`

**Features:**
- Real-time power consumption display
- Device status indicators (online/offline)
- Total energy usage summaries
- Auto-refresh every 5 seconds
- Mobile-optimized layout for iPhone and Android
- Works great as a home screen webapp

**REST API Endpoints:**
- `GET /api/devices` - Get all devices with current data
- `GET /api/device/{ip}` - Get specific device data

### InfluxDB Setup

1. Create a bucket for your energy data (or use an existing one)
2. Generate an API token with write permissions to your bucket
3. Note your organization name

## Usage

### Testing Discovery First

Before running the main application, you can test discovery:

```bash
# Test mDNS discovery
go run discovery-test.go -method mdns -timeout 10s

# Test network scanning
go run discovery-test.go -method scan -subnet 192.168.1.0/24 -timeout 30s
```

This will show you which plugs are found and output the IPs in JSON format for easy config generation.

### Running the Main Application

Run the application:
```bash
go run main.go
```

Or specify a custom config file:
```bash
go run main.go /path/to/config.json
```

Build and run as a binary:
```bash
go build -o tapo-logger
./tapo-logger
```

## Data Schema

The application writes data to InfluxDB with the following schema:

**Measurement**: `tapo_energy`

**Tags**:
- `plug_ip`: IP address of the plug

**Fields**:
- `current_power_watts`: Current power consumption in watts
- `today_energy_kwh`: Energy consumed today in kilowatt-hours
- `month_energy_kwh`: Energy consumed this month in kilowatt-hours
- `today_runtime_hours`: Runtime today in hours
- `month_runtime_hours`: Runtime this month in hours

## Visualization with Grafana

A pre-built Grafana dashboard is included for visualizing your energy data. The dashboard provides:

- Real-time power consumption monitoring
- Daily and monthly energy usage tracking
- Runtime statistics and trends
- Multi-plug comparison and distribution
- Historical data visualization

### Quick Setup

1. Import `grafana-dashboard.json` into your Grafana instance
2. Configure your InfluxDB data source
3. Update the bucket name if needed

For detailed setup instructions, see [GRAFANA.md](GRAFANA.md).

## Example Flux Queries

### Current power consumption:
```flux
from(bucket: "tapo_energy")
  |> range(start: -1h)
  |> filter(fn: (r) => r["_measurement"] == "tapo_energy")
  |> filter(fn: (r) => r["_field"] == "current_power_watts")
```

### Daily energy usage:
```flux
from(bucket: "tapo_energy")
  |> range(start: -7d)
  |> filter(fn: (r) => r["_measurement"] == "tapo_energy")
  |> filter(fn: (r) => r["_field"] == "today_energy_kwh")
  |> aggregateWindow(every: 1d, fn: last)
```

## Running as a Service

### systemd (Linux)

Create `/etc/systemd/system/tapo-logger.service`:

```ini
[Unit]
Description=Tapo Energy Monitor
After=network.target

[Service]
Type=simple
User=youruser
WorkingDirectory=/path/to/tapo-influx-logger
ExecStart=/path/to/tapo-influx-logger/tapo-logger
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable tapo-logger
sudo systemctl start tapo-logger
sudo systemctl status tapo-logger
```

## Troubleshooting

### Auto-Discovery Issues

**No plugs discovered:**
1. Check that your plugs are powered on and connected to WiFi
2. Verify you're on the same network as the plugs
3. Try the "scan" method instead of "mdns" if you have multicast issues
4. Ensure your subnet is correct in the configuration
5. Check firewall rules aren't blocking port 9999 or multicast traffic

**mDNS not working:**
- Some routers/networks block multicast DNS traffic
- Use `discovery_method: "scan"` instead
- Check if Avahi/Bonjour services are running (Linux/Mac)

**Network scan is slow:**
- Scanning large subnets can take time
- Consider using a smaller subnet (e.g., /26 instead of /24)
- Use "mdns" method if it works on your network

### Connection Issues

If you're having trouble connecting to your plugs:
1. Verify the plug IP addresses are correct and reachable
2. Ensure your machine can reach the plugs on the network
3. Check that the Tapo credentials are correct
4. Try accessing the plug through the Tapo app to ensure it's online

### Authentication Failures

- Make sure you're using your Tapo account credentials (not local device password)
- Some accounts may have 2FA enabled - this may cause issues
- Try logging out and back into the Tapo app to verify credentials

### InfluxDB Write Errors

- Verify your InfluxDB token has write permissions
- Check that the bucket exists
- Ensure the InfluxDB URL is correct and reachable

## Limitations

- This uses an unofficial/reverse-engineered API that could change
- The Tapo protocol requires authentication for each session
- Some plug models may report different fields or formats

## License

This project is provided as-is for personal use.

## Contributing

Feel free to submit issues or pull requests for improvements!

