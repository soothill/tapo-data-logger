# Docker Deployment Guide

This guide covers two Docker deployment options for the Tapo Data Logger:

1. **All-in-one deployment** - Includes InfluxDB and Grafana (recommended for new users)
2. **Standalone deployment** - Just the logger (for existing InfluxDB/Grafana setups)

## Prerequisites

- Docker Engine 20.10 or later
- Docker Compose v2.0 or later
- A `config.json` file (see [Configuration](#configuration))

## Option 1: All-in-One Deployment (Recommended)

This is the easiest way to get started. It includes:
- Tapo Data Logger
- InfluxDB 2.7
- Grafana 10.2.3

### Quick Start

1. **Create your configuration file:**

```bash
cp config.example.json config.json
```

Edit `config.json` with your Tapo credentials and devices:

```json
{
  "tapo_email": "your-email@example.com",
  "tapo_password": "your-password",
  "plug_ips": ["192.168.1.100"],
  "influx_url": "http://influxdb:8086",
  "influx_token": "my-super-secret-auth-token",
  "influx_org": "my-org",
  "influx_bucket": "tapo_energy",
  "poll_interval_seconds": 60,
  "webui_enabled": true,
  "webui_port": 8080
}
```

**Important:** Use `http://influxdb:8086` as the InfluxDB URL (Docker service name).

2. **Review and customize docker-compose.yml:**

Edit the environment variables in `docker-compose.yml`:

```yaml
# InfluxDB credentials (match these in your config.json)
- DOCKER_INFLUXDB_INIT_USERNAME=admin
- DOCKER_INFLUXDB_INIT_PASSWORD=adminpassword123  # CHANGE THIS!
- DOCKER_INFLUXDB_INIT_ORG=my-org
- DOCKER_INFLUXDB_INIT_BUCKET=tapo_energy
- DOCKER_INFLUXDB_INIT_ADMIN_TOKEN=my-super-secret-auth-token  # CHANGE THIS!

# Grafana credentials
- GF_SECURITY_ADMIN_USER=admin
- GF_SECURITY_ADMIN_PASSWORD=admin  # CHANGE THIS!
```

3. **Start the stack:**

```bash
docker-compose up -d
```

4. **Verify everything is running:**

```bash
docker-compose ps
```

All services should show as "healthy" after a minute or two.

### Access the Services

- **Tapo Data Logger Web UI:** http://localhost:8080
- **Grafana Dashboard:** http://localhost:3000 (admin/admin)
- **InfluxDB:** http://localhost:8086

### Setting Up Grafana Dashboard

1. Log in to Grafana at http://localhost:3000
2. Go to **Configuration → Data Sources → Add data source**
3. Select **InfluxDB**
4. Configure:
   - **Query Language:** Flux
   - **URL:** `http://influxdb:8086`
   - **Organization:** `my-org` (or whatever you set)
   - **Token:** `my-super-secret-auth-token` (from docker-compose.yml)
   - **Default Bucket:** `tapo_energy`
5. Click **Save & Test**
6. Import the dashboard from `grafana-dashboard.json`:
   - Go to **Dashboards → Import**
   - Upload `grafana-dashboard.json`
   - Select your InfluxDB data source
   - Click **Import**

### Managing the All-in-One Stack

```bash
# Start all services
docker-compose up -d

# Stop all services
docker-compose down

# Stop and remove all data (DESTRUCTIVE!)
docker-compose down -v

# View logs
docker-compose logs -f

# View logs for specific service
docker-compose logs -f tapo-logger

# Restart a service
docker-compose restart tapo-logger

# Rebuild and restart after code changes
docker-compose up -d --build
```

### Data Persistence

Data is stored in Docker volumes:
- `influxdb-data` - InfluxDB time-series data
- `influxdb-config` - InfluxDB configuration
- `grafana-data` - Grafana dashboards and settings

To backup your data:

```bash
# Backup InfluxDB
docker run --rm -v tapo-data-logger_influxdb-data:/data -v $(pwd):/backup \
  alpine tar czf /backup/influxdb-backup-$(date +%Y%m%d).tar.gz /data

# Backup Grafana
docker run --rm -v tapo-data-logger_grafana-data:/data -v $(pwd):/backup \
  alpine tar czf /backup/grafana-backup-$(date +%Y%m%d).tar.gz /data
```

---

## Option 2: Standalone Deployment

Use this if you already have InfluxDB and Grafana running.

### Quick Start

1. **Create your configuration file:**

```bash
cp config.example.json config.json
```

Edit `config.json` with your existing InfluxDB details:

```json
{
  "tapo_email": "your-email@example.com",
  "tapo_password": "your-password",
  "plug_ips": ["192.168.1.100"],
  "influx_url": "http://your-influxdb-host:8086",
  "influx_token": "your-existing-token",
  "influx_org": "your-org",
  "influx_bucket": "tapo_energy",
  "poll_interval_seconds": 60,
  "webui_enabled": true,
  "webui_port": 8080
}
```

2. **Start the logger:**

```bash
docker-compose -f docker-compose-standalone.yml up -d
```

3. **Verify it's running:**

```bash
docker-compose -f docker-compose-standalone.yml ps
```

### Network Configuration

If your InfluxDB is running on localhost, you have two options:

**Option A: Use host networking (Linux only)**

Uncomment the `network_mode: "host"` line in `docker-compose-standalone.yml`:

```yaml
network_mode: "host"
```

Then use `http://localhost:8086` in your config.json.

**Option B: Use Docker bridge network**

If InfluxDB is in another container, connect to the same network or use the host IP address.

### Managing the Standalone Deployment

```bash
# Start
docker-compose -f docker-compose-standalone.yml up -d

# Stop
docker-compose -f docker-compose-standalone.yml down

# View logs
docker-compose -f docker-compose-standalone.yml logs -f

# Restart
docker-compose -f docker-compose-standalone.yml restart

# Rebuild after code changes
docker-compose -f docker-compose-standalone.yml up -d --build
```

---

## Advanced Configuration

### Environment Variables

You can override configuration using environment variables:

```bash
docker run -d \
  -e CONFIG_FILE=/app/config.json \
  -e TZ=America/New_York \
  -v $(pwd)/config.json:/app/config.json:ro \
  -p 8080:8080 \
  tapo-data-logger
```

### Resource Limits

Both docker-compose files include resource limits:

```yaml
deploy:
  resources:
    limits:
      cpus: '1'
      memory: 512M
    reservations:
      cpus: '0.25'
      memory: 128M
```

Adjust these based on your system and number of devices.

### Security

The containers run as non-root users and have minimal capabilities:

```yaml
cap_drop:
  - ALL
cap_add:
  - NET_BIND_SERVICE
```

### Using Docker Secrets (Swarm/Kubernetes)

For production deployments, consider using Docker secrets for sensitive data:

```yaml
secrets:
  tapo_password:
    external: true
  influx_token:
    external: true
```

---

## Troubleshooting

### Container won't start

```bash
# Check logs
docker-compose logs tapo-logger

# Common issues:
# 1. Missing config.json - ensure it exists
# 2. Invalid JSON in config.json - validate with a JSON linter
# 3. Wrong file permissions - ensure config.json is readable
```

### Can't connect to InfluxDB

```bash
# Test connectivity from the container
docker-compose exec tapo-logger wget -O- http://influxdb:8086/health

# Check InfluxDB logs
docker-compose logs influxdb

# Ensure InfluxDB is healthy
docker-compose ps
```

### Web UI not accessible

```bash
# Check if port is bound
docker-compose ps

# Check if service is listening
docker-compose exec tapo-logger netstat -ln | grep 8080

# Check firewall rules on host
sudo iptables -L | grep 8080
```

### Data not appearing in InfluxDB

```bash
# Check logger logs for errors
docker-compose logs -f tapo-logger

# Verify InfluxDB credentials match
# Check config.json matches docker-compose.yml environment variables

# Test manual write to InfluxDB
docker-compose exec influxdb influx write \
  -b tapo_energy \
  -o my-org \
  -t my-super-secret-auth-token \
  'test,host=server01 value=1.0'
```

### Performance issues

```bash
# Check resource usage
docker stats

# Adjust resource limits in docker-compose.yml
# Increase batch_write_size in config.json
# Increase poll_interval_seconds in config.json
```

---

## Using Pre-built Images from GitHub Container Registry

The project automatically builds and publishes Docker images to GitHub Container Registry (GHCR) for easy deployment.

### Available Tags

- `latest` - Latest build from the main branch
- `main` - Latest build from the main branch
- `v*.*.*` - Specific version tags (e.g., v1.0.0)
- `main-<sha>` - Specific commit builds

### Pull and Run from GHCR

```bash
# Pull the latest image
docker pull ghcr.io/soothill/tapo-data-logger:latest

# Run the image
docker run -d \
  --name tapo-logger \
  -v $(pwd)/config.json:/app/config.json:ro \
  -p 8080:8080 \
  ghcr.io/soothill/tapo-data-logger:latest
```

### Using GHCR Images with Docker Compose

Update your `docker-compose.yml` or `docker-compose-standalone.yml` to use the pre-built image:

```yaml
services:
  tapo-logger:
    image: ghcr.io/soothill/tapo-data-logger:latest
    # Comment out or remove the 'build' section
    container_name: tapo-data-logger
    # ... rest of your configuration
```

## Building the Image Manually

If you prefer to build locally instead of using GHCR:

```bash
# Build the image
docker build -t tapo-data-logger:latest .

# Run manually
docker run -d \
  --name tapo-logger \
  -v $(pwd)/config.json:/app/config.json:ro \
  -p 8080:8080 \
  tapo-data-logger:latest
```

---

## Health Checks

All services include health checks:

```bash
# Check health status
docker-compose ps

# Detailed health check
docker inspect tapo-data-logger | grep -A 20 Health
```

Health check endpoints:
- **Tapo Logger:** http://localhost:8080/
- **InfluxDB:** `influx ping`
- **Grafana:** http://localhost:3000/api/health

---

## Upgrading

### All-in-One Stack

```bash
# Pull latest images
docker-compose pull

# Rebuild and restart
docker-compose up -d --build

# Check logs for any issues
docker-compose logs -f
```

### Standalone Deployment

```bash
# Rebuild image
docker-compose -f docker-compose-standalone.yml build

# Restart with new image
docker-compose -f docker-compose-standalone.yml up -d

# Check logs
docker-compose -f docker-compose-standalone.yml logs -f
```

---

## Production Recommendations

1. **Change default passwords** in docker-compose.yml
2. **Use Docker secrets** for sensitive data
3. **Enable HTTPS** with a reverse proxy (nginx, Traefik, Caddy)
4. **Set up automated backups** for InfluxDB and Grafana volumes
5. **Monitor container health** with tools like Prometheus
6. **Use specific image tags** instead of `latest`
7. **Set up log rotation** to prevent disk fill
8. **Review resource limits** based on your device count
9. **Enable authentication** on all services
10. **Regular updates** of all container images

---

## Additional Resources

- [Tapo Data Logger README](README.md)
- [Grafana Dashboard Guide](GRAFANA.md)
- [InfluxDB Documentation](https://docs.influxdata.com/influxdb/v2/)
- [Grafana Documentation](https://grafana.com/docs/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
