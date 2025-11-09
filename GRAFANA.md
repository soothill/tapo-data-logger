# Grafana Dashboard Setup

This document explains how to set up and use the Grafana dashboard for visualizing Tapo energy monitoring data.

## Dashboard Features

The Tapo Energy Monitor dashboard provides comprehensive visualization of your smart plug data:

### Real-Time Monitoring
- **Current Power Consumption**: Gauge showing live power usage with color-coded thresholds
- **Power Consumption Over Time**: Time series graph showing power usage trends

### Energy Metrics
- **Today's Energy Usage**: Current day's energy consumption in kWh
- **This Month's Energy Usage**: Current month's total energy consumption
- **Daily Energy Usage (Last 30 Days)**: Historical bar chart of daily consumption

### Runtime Tracking
- **Today's Runtime**: Hours the device has been running today
- **This Month's Runtime**: Total runtime hours for the current month
- **Daily Runtime (Last 30 Days)**: Historical bar chart of daily runtime

### Multi-Plug Support
- **Energy Distribution by Plug**: Pie chart showing energy usage breakdown across all plugs
- **All Plugs Summary**: Table view with all metrics for each plug

## Prerequisites

- Grafana 9.0 or later
- InfluxDB 2.x data source configured in Grafana
- Tapo data logger running and writing data to InfluxDB

## Installation

### 1. Configure InfluxDB Data Source

First, add your InfluxDB instance as a data source in Grafana:

1. Navigate to **Configuration → Data Sources** in Grafana
2. Click **Add data source**
3. Select **InfluxDB**
4. Configure the connection:
   - **Name**: `InfluxDB` (or your preferred name)
   - **Query Language**: `Flux`
   - **URL**: Your InfluxDB URL (e.g., `http://localhost:8086`)
   - **Auth**: Toggle "Basic auth" if needed
   - **Organization**: Your InfluxDB organization name
   - **Token**: Your InfluxDB API token
   - **Default Bucket**: `tapo_energy` (or your configured bucket)
5. Click **Save & Test** to verify the connection

### 2. Import the Dashboard

1. In Grafana, navigate to **Dashboards → Import**
2. Click **Upload JSON file**
3. Select the `grafana-dashboard.json` file from this repository
4. Configure the import options:
   - **Name**: Keep as "Tapo Energy Monitor" or customize
   - **Folder**: Select a folder or create a new one
   - **InfluxDB**: Select your InfluxDB data source
5. Click **Import**

## Configuration

### Dashboard Variables

The dashboard includes several template variables that allow you to customize the view:

#### Data Source
- **Variable**: `DS_INFLUXDB`
- **Description**: Select the InfluxDB data source to use
- **Configuration**: Automatically populated with available InfluxDB sources

#### Bucket
- **Variable**: `bucket`
- **Description**: The InfluxDB bucket name containing your Tapo data
- **Default**: `tapo_energy`
- **Configuration**: Change this if your bucket has a different name

#### Plug IP Filter
- **Variable**: `plug_ip`
- **Description**: Filter data by specific plug IP address(es)
- **Default**: All plugs
- **Configuration**:
  - Select "All" to view data from all plugs
  - Select one or more specific IPs to filter the view
  - Multi-select is supported

#### Time Interval
- **Variable**: `interval`
- **Description**: Aggregation interval for time series data
- **Default**: Auto (automatically adjusts based on time range)
- **Options**: 10s, 30s, 1m, 5m, 15m, 30m, 1h, 6h, 12h, 1d

### Customizing the Dashboard

#### Adjusting Thresholds

You can customize color thresholds for the power gauge:

1. Click the panel title and select **Edit**
2. Navigate to the **Field** tab on the right
3. Expand **Thresholds**
4. Modify the values:
   - Green: 0-100W (normal usage)
   - Yellow: 100-500W (moderate usage)
   - Orange: 500-1000W (high usage)
   - Red: >1000W (very high usage)

#### Modifying Time Ranges

Default time range is last 24 hours. To change:

1. Use the time picker in the top-right corner
2. Select a preset range (e.g., Last 7 days, Last 30 days)
3. Or set a custom absolute or relative time range

#### Auto-Refresh

The dashboard auto-refreshes every 30 seconds by default. To change:

1. Click the refresh interval dropdown (top-right)
2. Select a different interval (10s, 1m, 5m, etc.)
3. Or disable auto-refresh

## Panel Descriptions

### 1. Current Power Consumption (Gauge)
- Shows the most recent power reading
- Updates automatically every 30 seconds
- Color-coded based on power usage thresholds

### 2. Power Consumption Over Time (Time Series)
- Displays power usage trends over the selected time range
- Aggregates data based on the selected interval
- Shows mean, max, and last values in the legend
- Supports zooming by dragging on the graph

### 3-6. Stat Panels (Energy and Runtime)
- Display single-value metrics with sparkline graphs
- Show the latest values from the last hour
- Color-coded based on configured thresholds

### 7-8. Daily Charts (Bar Charts)
- Show daily maximum values for the last 30 days
- Useful for identifying usage patterns and trends
- Display sum, mean, and max in the legend

### 9. Energy Distribution (Pie Chart)
- Shows proportional energy usage across all plugs
- Useful when monitoring multiple devices
- Displays both values and percentages

### 10. All Plugs Summary (Table)
- Comprehensive view of all metrics for each plug
- Color-coded current power column
- Sortable by any column (click column header)
- Auto-updates with live data

## Troubleshooting

### No Data Displayed

1. **Check InfluxDB connection**:
   - Verify the data source is properly configured
   - Test the connection in Data Sources settings

2. **Verify bucket name**:
   - Ensure the `bucket` variable matches your InfluxDB bucket
   - Default is `tapo_energy`

3. **Check time range**:
   - Ensure you have data in the selected time range
   - Try expanding the time range to "Last 7 days"

4. **Verify data is being written**:
   ```bash
   # Query InfluxDB directly
   influx query 'from(bucket: "tapo_energy") |> range(start: -1h) |> limit(n: 10)'
   ```

### Queries Timing Out

1. **Reduce time range**: Try a shorter time range first
2. **Increase interval**: Use a larger aggregation interval for long time ranges
3. **Check InfluxDB performance**: Ensure InfluxDB has sufficient resources

### Plug IPs Not Showing in Filter

1. **Wait for data**: The variable refreshes every time you open the dashboard
2. **Check data**: Ensure data exists with the `plug_ip` tag
3. **Manually refresh**: Click the refresh icon next to the variable dropdown

## Data Schema Reference

The dashboard expects data in the following InfluxDB schema:

**Measurement**: `tapo_energy`

**Tags**:
- `plug_ip`: IP address of the smart plug

**Fields**:
- `current_power_watts`: Current power consumption (float)
- `today_energy_kwh`: Energy consumed today (float)
- `month_energy_kwh`: Energy consumed this month (float)
- `today_runtime_hours`: Runtime today (float)
- `month_runtime_hours`: Runtime this month (float)

## Tips and Best Practices

1. **Create Multiple Dashboards**: Create separate dashboards for different areas (e.g., kitchen, office)
2. **Use Variables**: Leverage the plug_ip variable to create focused views
3. **Set Alerts**: Configure Grafana alerts for unusual power consumption
4. **Export Data**: Use Grafana's export feature to generate reports
5. **Customize Panels**: Modify panel queries to create custom visualizations
6. **Naming Plugs**: Consider setting up a naming service to map IPs to device names

## Advanced Customization

### Adding Device Names

To display friendly device names instead of IP addresses, you can:

1. Edit the Tapo logger to include device names as tags
2. Modify the Flux queries to map IPs to names
3. Use Grafana's transformation features to replace values

### Calculating Costs

To add energy cost calculations:

1. Add a new variable for cost per kWh
2. Create new panels with calculated fields
3. Use transformation to multiply energy by cost

Example query modification:
```flux
from(bucket: "${bucket}")
  |> range(start: -1h)
  |> filter(fn: (r) => r["_field"] == "month_energy_kwh")
  |> map(fn: (r) => ({ r with _value: r._value * ${cost_per_kwh} }))
```

### Integration with Other Systems

The dashboard can be:
- Embedded in other web applications
- Accessed via Grafana's API
- Exported as PDF reports
- Shared with team members via links

## Support

For issues with:
- **Dashboard**: Check this documentation and Grafana logs
- **Data collection**: See the main README.md
- **InfluxDB**: Consult InfluxDB documentation

## License

This dashboard template is part of the tapo-data-logger project.
