# Tapo Data Logger - TODO List

This document tracks suggested improvements and enhancements for the tapo-data-logger project.

## Status Legend
- ⏳ Pending
- 🚧 In Progress
- ✅ Completed

---

## Monitoring & Observability

### ✅ Add device metadata tracking
Track device name, model, firmware version, and other metadata in InfluxDB alongside energy data.

**Benefits:**
- Better identification of devices in dashboards
- Track firmware versions for debugging
- Enhanced data analysis capabilities

**Estimated effort:** Medium

**Implemented:** Device metadata (name, model, firmware version, MAC address, device type) is tracked and stored as tags in InfluxDB. Metadata is refreshed hourly and published to MQTT when enabled.

---

### ⏳ Implement Prometheus metrics endpoint
Add a `/metrics` HTTP endpoint exposing Prometheus-compatible metrics.

**Metrics to expose:**
- Device connection status
- API request counts and latencies
- Error rates
- Data collection success/failure rates

**Benefits:**
- Industry-standard monitoring integration
- Better observability in production
- Easy integration with Grafana

**Estimated effort:** Medium

---

### ✅ Create Grafana dashboard template
Provide a ready-to-use Grafana dashboard JSON template.

**Panels to include:**
- Real-time power consumption ✅
- Daily/monthly energy usage ✅
- Device availability ✅
- Cost estimation (with configurable rates) ⏳

**Benefits:**
- Quick setup for new users
- Best practices for visualization
- Professional-looking dashboards out of the box

**Estimated effort:** Small

**Implemented:** Comprehensive Grafana dashboard template with power consumption, energy usage, and device tracking panels.

---

### ✅ Implement alerting system
Add alerting for device offline detection and anomalous power consumption.

**Alert types:**
- Device offline for > X minutes ✅
- Unusual power consumption patterns ⏳
- Failed authentication attempts ⏳
- InfluxDB connection issues ⏳

**Benefits:**
- Proactive issue detection
- Reduce downtime
- Better operational awareness

**Estimated effort:** Medium

**Implemented:** Slack webhook integration with configurable thresholds for device offline/online alerts.

---

## Performance & Reliability

### ✅ Add device-specific polling intervals
Allow different polling intervals per device based on importance/usage.

**Configuration:**
```json
"device_poll_intervals": {
  "192.168.1.10": 30,
  "192.168.1.11": 300
}
```

**Benefits:**
- Optimize network traffic
- Reduce API calls to less critical devices
- Better resource utilization

**Estimated effort:** Medium

**Implemented:** Device-specific polling intervals via `device_poll_intervals` configuration map.

---

### ✅ Implement connection pooling for InfluxDB writes
Reuse InfluxDB connections instead of creating new ones for each write.

**Benefits:**
- Reduced connection overhead
- Better performance
- Lower memory usage

**Estimated effort:** Small

**Implemented:** Connection pooling with configurable settings: `influx_max_idle_conns`, `influx_max_idle_conns_per_host`, `influx_max_conns_per_host`, and `influx_idle_conn_timeout`.

---

### ✅ Add data buffering with batch writes
Buffer data points and write to InfluxDB in batches instead of individual writes.

**Features:**
- Configurable batch size and flush interval ✅
- Automatic flush on shutdown ✅
- Error handling with retry buffer ✅

**Benefits:**
- Significantly reduced InfluxDB load
- Better write performance
- Network efficiency

**Estimated effort:** Medium

**Implemented:** PointBuffer system with configurable `batch_write_size` and `batch_write_interval` settings.

---

### ✅ Add support for multiple InfluxDB instances
Support multiple InfluxDB instances for high availability and failover.

**Features:**
- Primary/secondary InfluxDB configuration ✅
- Automatic failover on connection loss ✅
- Health checking ✅

**Benefits:**
- High availability
- Zero data loss during InfluxDB maintenance
- Production-ready reliability

**Estimated effort:** Large

**Implemented:** Multiple InfluxDB support via `influx_urls` array with priority-based failover and health checking.

---

### ✅ Add device state caching
Cache device state to reduce unnecessary API calls when data hasn't changed.

**Features:**
- Configurable cache TTL ✅
- Smart invalidation ✅
- Memory-efficient storage ✅

**Benefits:**
- Reduced API calls to devices
- Lower network traffic
- Faster response times

**Estimated effort:** Medium

**Implemented:** DeviceCache with configurable `cache_ttl_seconds` setting (0 = disabled).

---

### ✅ Implement rate limiting for device requests
Protect devices from excessive API calls and respect manufacturer limits.

**Features:**
- Per-device rate limiting ✅
- Configurable limits ✅
- Queue-based request handling ✅

**Benefits:**
- Prevent device overload
- Comply with API rate limits
- More reliable operations

**Estimated effort:** Medium

**Implemented:** RateLimiter with semaphore-based concurrency control via `max_concurrent` setting.

---

## Operational Improvements

### ✅ Add configuration reload without restart
Support SIGHUP signal to reload configuration without stopping data collection.

**Features:**
- Hot reload of non-critical settings ✅
- Add/remove devices dynamically ✅
- Update polling intervals on the fly ✅

**Benefits:**
- Zero downtime configuration updates
- Better operational experience
- Production-friendly

**Estimated effort:** Medium

**Implemented:** SIGHUP signal handler reloads configuration and updates polling intervals, device lists, and other settings without restarting the application.

---

### ✅ Add historical data export/backup
Provide tools to export historical data from InfluxDB for backup or migration.

**Features:**
- Export to CSV/JSON formats ✅
- Date range selection ✅
- Per-device export ✅

**Benefits:**
- Data portability
- Backup capabilities
- Analysis in external tools

**Estimated effort:** Medium

**Implemented:** `cmd-export.go` provides a CLI tool for exporting data to JSON or CSV formats with date range and device filtering options.

---

### ⏳ Create Docker container and docker-compose setup
Package application as Docker container with docker-compose for easy deployment.

**Include:**
- Multi-stage build for small image size
- docker-compose.yml with InfluxDB and Grafana
- Environment variable configuration
- Health checks

**Benefits:**
- Easy deployment
- Consistent environment
- Perfect for home servers

**Estimated effort:** Medium

---

### ✅ Add configuration validation CLI command
Add `--validate-config` flag to check configuration without running the application.

**Checks:**
- JSON syntax ✅
- Required fields ✅
- Network connectivity tests ⏳
- Credential validation ⏳

**Benefits:**
- Catch errors before deployment
- Better troubleshooting
- CI/CD integration

**Estimated effort:** Small

**Implemented:** ValidateConfig() function validates JSON syntax, required fields, and configuration consistency on startup.

---

### ⏳ Implement secure credential storage
Integrate with system keyring or Vault for secure credential storage.

**Options:**
- System keyring integration (Linux: Secret Service, macOS: Keychain)
- HashiCorp Vault support
- Environment variable support

**Benefits:**
- No plaintext passwords in config
- Better security posture
- Enterprise-ready

**Estimated effort:** Large

---

## Features & Extensibility

### ⏳ Create web UI for configuration and monitoring
Build a simple web interface for managing the application.

**Features:**
- View current device status
- Configure devices and settings
- View real-time power consumption
- Restart/reload operations

**Benefits:**
- User-friendly management
- No command-line required
- Better accessibility

**Estimated effort:** Large

---

### ✅ Implement MQTT support
Publish device data to MQTT for integration with home automation systems.

**Topics:**
```
tapo/{device_id}/energy   (combined energy data with metadata)
tapo/{device_id}/metadata (device information)
```

**Benefits:**
- Home Assistant integration
- Real-time updates
- IoT ecosystem compatibility

**Estimated effort:** Medium

**Implemented:** Full MQTT support with configurable broker URL, authentication, client ID, topic prefix, and QoS levels. Publishes energy data and device metadata to separate topics.

---

### ⏳ Add support for other Tapo device types
Extend beyond energy monitoring plugs to other Tapo devices.

**Device types:**
- Smart bulbs (L530, L510, etc.)
- Security cameras
- Sensors
- Other smart home devices

**Benefits:**
- Comprehensive Tapo ecosystem monitoring
- Wider applicability
- More user adoption

**Estimated effort:** Large

---

## Testing & Quality

### ✅ Add integration tests with mock Tapo devices
Create integration tests using mock Tapo device servers.

**Coverage:**
- Authentication flows ✅
- Energy data retrieval ✅
- Error handling ✅
- Retry logic ✅

**Benefits:**
- Better test coverage
- Catch regressions
- Confidence in releases

**Estimated effort:** Medium

**Implemented:** `integration_test.go` includes comprehensive integration tests with a mock Tapo device server that simulates authentication, encryption, and device responses.

---

### ⏳ Create benchmarks for performance testing
Add benchmark tests to track performance over time.

**Benchmarks:**
- Encryption/decryption performance
- Concurrent device handling
- Memory usage patterns
- InfluxDB write performance

**Benefits:**
- Performance regression detection
- Optimization guidance
- Scalability insights

**Estimated effort:** Small

---

## Priority Recommendations

### High Priority (Quick Wins)
1. ✅ Create Grafana dashboard template
2. ✅ Add configuration validation CLI command
3. ✅ Implement connection pooling for InfluxDB writes
4. ⏳ Create Docker container and docker-compose setup

### Medium Priority (High Impact)
1. ⏳ Implement Prometheus metrics endpoint
2. ✅ Add data buffering with batch writes
3. ✅ Add device metadata tracking
4. ✅ Implement MQTT support
5. ✅ Implement alerting system (Slack)

### Lower Priority (Nice to Have)
1. ⏳ Create web UI for configuration and monitoring
2. ⏳ Add support for other Tapo device types
3. ⏳ Implement secure credential storage

### Recently Completed
1. ✅ Add device-specific polling intervals
2. ✅ Add support for multiple InfluxDB instances (HA/failover)
3. ✅ Add device state caching
4. ✅ Implement rate limiting for device requests
5. ✅ Add device metadata tracking
6. ✅ Implement MQTT support
7. ✅ Add configuration reload without restart (SIGHUP)
8. ✅ Add historical data export/backup
9. ✅ Implement connection pooling for InfluxDB writes
10. ✅ Add integration tests with mock Tapo devices

---

## Contributing

When working on items from this list:
1. Move the item to "🚧 In Progress" status
2. Create a feature branch: `feature/description-of-task`
3. Implement the feature with tests
4. Update this file to mark as "✅ Completed"
5. Submit a pull request

---

## Summary

**Total Tasks:** 22
**Completed:** 14 ✅
**In Progress:** 0 🚧
**Pending:** 8 ⏳
**Completion Rate:** 64%

---

*Last updated: 2025-11-09*
