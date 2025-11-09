// Copyright (c) 2025 Darren Soothill. All rights reserved.

package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

// ExportFormat represents the export file format
type ExportFormat string

const (
	FormatJSON ExportFormat = "json"
	FormatCSV  ExportFormat = "csv"
)

// ExportOptions contains options for data export
type ExportOptions struct {
	InfluxURL    string
	InfluxToken  string
	InfluxOrg    string
	InfluxBucket string
	StartTime    time.Time
	EndTime      time.Time
	OutputFile   string
	Format       ExportFormat
	PlugIP       string // Optional: filter by specific plug IP
}

// DataPoint represents a single data point for export
type DataPoint struct {
	Time               time.Time `json:"time"`
	PlugIP             string    `json:"plug_ip"`
	CurrentPowerWatts  float64   `json:"current_power_watts"`
	TodayEnergyKWh     float64   `json:"today_energy_kwh"`
	MonthEnergyKWh     float64   `json:"month_energy_kwh"`
	TodayRuntimeHours  float64   `json:"today_runtime_hours"`
	MonthRuntimeHours  float64   `json:"month_runtime_hours"`
}

// ExportData exports historical data from InfluxDB
func ExportData(ctx context.Context, opts ExportOptions) error {
	// Create InfluxDB client
	client := influxdb2.NewClient(opts.InfluxURL, opts.InfluxToken)
	defer client.Close()

	// Get query API
	queryAPI := client.QueryAPI(opts.InfluxOrg)

	// Build Flux query
	query := buildExportQuery(opts)

	// Execute query
	result, err := queryAPI.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query InfluxDB: %w", err)
	}
	defer result.Close()

	// Collect data points
	var dataPoints []DataPoint
	for result.Next() {
		record := result.Record()

		// Get plug IP from tags
		plugIP, ok := record.ValueByKey("plug_ip").(string)
		if !ok {
			continue
		}

		// Get field name
		field := record.Field()

		// Get value
		value, ok := record.Value().(float64)
		if !ok {
			continue
		}

		// Find or create data point for this timestamp and plug
		timestamp := record.Time()
		var dp *DataPoint
		for i := range dataPoints {
			if dataPoints[i].Time.Equal(timestamp) && dataPoints[i].PlugIP == plugIP {
				dp = &dataPoints[i]
				break
			}
		}

		if dp == nil {
			dataPoints = append(dataPoints, DataPoint{
				Time:   timestamp,
				PlugIP: plugIP,
			})
			dp = &dataPoints[len(dataPoints)-1]
		}

		// Set field value
		switch field {
		case "current_power_watts":
			dp.CurrentPowerWatts = value
		case "today_energy_kwh":
			dp.TodayEnergyKWh = value
		case "month_energy_kwh":
			dp.MonthEnergyKWh = value
		case "today_runtime_hours":
			dp.TodayRuntimeHours = value
		case "month_runtime_hours":
			dp.MonthRuntimeHours = value
		}
	}

	if result.Err() != nil {
		return fmt.Errorf("query error: %w", result.Err())
	}

	// Export data in requested format
	switch opts.Format {
	case FormatJSON:
		return exportJSON(dataPoints, opts.OutputFile)
	case FormatCSV:
		return exportCSV(dataPoints, opts.OutputFile)
	default:
		return fmt.Errorf("unsupported export format: %s", opts.Format)
	}
}

// buildExportQuery builds a Flux query for data export
func buildExportQuery(opts ExportOptions) string {
	query := fmt.Sprintf(`from(bucket: "%s")
  |> range(start: %s, stop: %s)
  |> filter(fn: (r) => r["_measurement"] == "tapo_energy")`,
		opts.InfluxBucket,
		opts.StartTime.Format(time.RFC3339),
		opts.EndTime.Format(time.RFC3339),
	)

	// Add plug IP filter if specified
	if opts.PlugIP != "" {
		query += fmt.Sprintf(`
  |> filter(fn: (r) => r["plug_ip"] == "%s")`, opts.PlugIP)
	}

	return query
}

// exportJSON exports data points to JSON file
func exportJSON(dataPoints []DataPoint, outputFile string) error {
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(dataPoints); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// exportCSV exports data points to CSV file
func exportCSV(dataPoints []DataPoint, outputFile string) error {
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"timestamp",
		"plug_ip",
		"current_power_watts",
		"today_energy_kwh",
		"month_energy_kwh",
		"today_runtime_hours",
		"month_runtime_hours",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data rows
	for _, dp := range dataPoints {
		row := []string{
			dp.Time.Format(time.RFC3339),
			dp.PlugIP,
			fmt.Sprintf("%.2f", dp.CurrentPowerWatts),
			fmt.Sprintf("%.3f", dp.TodayEnergyKWh),
			fmt.Sprintf("%.3f", dp.MonthEnergyKWh),
			fmt.Sprintf("%.2f", dp.TodayRuntimeHours),
			fmt.Sprintf("%.2f", dp.MonthRuntimeHours),
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}

// BackupData is a convenience function that exports all data for backup
func BackupData(ctx context.Context, config *Config, outputFile string, format ExportFormat, days int) error {
	// Determine which InfluxDB instance to use
	var influxURL, influxToken, influxOrg, influxBucket string

	if config.InfluxURL != "" {
		influxURL = config.InfluxURL
		influxToken = config.InfluxToken
		influxOrg = config.InfluxOrg
		influxBucket = config.InfluxBucket
	} else if len(config.InfluxURLs) > 0 {
		// Use first (highest priority) instance
		influxURL = config.InfluxURLs[0].URL
		influxToken = config.InfluxURLs[0].Token
		influxOrg = config.InfluxURLs[0].Org
		influxBucket = config.InfluxURLs[0].Bucket
	} else {
		return fmt.Errorf("no InfluxDB configuration found")
	}

	opts := ExportOptions{
		InfluxURL:    influxURL,
		InfluxToken:  influxToken,
		InfluxOrg:    influxOrg,
		InfluxBucket: influxBucket,
		StartTime:    time.Now().AddDate(0, 0, -days),
		EndTime:      time.Now(),
		OutputFile:   outputFile,
		Format:       format,
	}

	return ExportData(ctx, opts)
}
