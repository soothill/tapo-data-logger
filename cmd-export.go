// Copyright (c) 2025 Darren Soothill. All rights reserved.
// +build ignore

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	// Command-line flags
	configFile := flag.String("config", "config.json", "Path to configuration file")
	outputFile := flag.String("output", "", "Output file path (required)")
	format := flag.String("format", "json", "Export format: json or csv")
	days := flag.Int("days", 30, "Number of days to export (from now backwards)")
	startTime := flag.String("start", "", "Start time in RFC3339 format (optional, overrides -days)")
	endTime := flag.String("end", "", "End time in RFC3339 format (optional, defaults to now)")
	plugIP := flag.String("plug", "", "Filter by specific plug IP (optional)")

	flag.Parse()

	// Validate required flags
	if *outputFile == "" {
		fmt.Fprintf(os.Stderr, "Error: -output flag is required\n")
		flag.Usage()
		os.Exit(1)
	}

	// Load configuration
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Parse times
	var start, end time.Time
	if *startTime != "" {
		start, err = time.Parse(time.RFC3339, *startTime)
		if err != nil {
			log.Fatalf("Failed to parse start time: %v", err)
		}
	} else {
		start = time.Now().AddDate(0, 0, -*days)
	}

	if *endTime != "" {
		end, err = time.Parse(time.RFC3339, *endTime)
		if err != nil {
			log.Fatalf("Failed to parse end time: %v", err)
		}
	} else {
		end = time.Now()
	}

	// Determine export format
	var exportFormat ExportFormat
	switch *format {
	case "json":
		exportFormat = FormatJSON
	case "csv":
		exportFormat = FormatCSV
	default:
		log.Fatalf("Invalid format: %s (must be 'json' or 'csv')", *format)
	}

	// Determine which InfluxDB instance to use
	var influxURL, influxToken, influxOrg, influxBucket string
	if config.InfluxURL != "" {
		influxURL = config.InfluxURL
		influxToken = config.InfluxToken
		influxOrg = config.InfluxOrg
		influxBucket = config.InfluxBucket
	} else if len(config.InfluxURLs) > 0 {
		influxURL = config.InfluxURLs[0].URL
		influxToken = config.InfluxURLs[0].Token
		influxOrg = config.InfluxURLs[0].Org
		influxBucket = config.InfluxURLs[0].Bucket
	} else {
		log.Fatalf("No InfluxDB configuration found")
	}

	// Create export options
	opts := ExportOptions{
		InfluxURL:    influxURL,
		InfluxToken:  influxToken,
		InfluxOrg:    influxOrg,
		InfluxBucket: influxBucket,
		StartTime:    start,
		EndTime:      end,
		OutputFile:   *outputFile,
		Format:       exportFormat,
		PlugIP:       *plugIP,
	}

	// Export data
	ctx := context.Background()
	fmt.Printf("Exporting data from %s to %s...\n", start.Format(time.RFC3339), end.Format(time.RFC3339))
	if *plugIP != "" {
		fmt.Printf("Filtering by plug IP: %s\n", *plugIP)
	}

	if err := ExportData(ctx, opts); err != nil {
		log.Fatalf("Export failed: %v", err)
	}

	fmt.Printf("Data exported successfully to: %s\n", *outputFile)
}
