// Copyright (c) 2025 Darren Soothill. All rights reserved.

package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

// DeviceData represents the current state and energy data for a device
type DeviceData struct {
	IP                 string  `json:"ip"`
	Name               string  `json:"name"`
	Model              string  `json:"model"`
	FirmwareVersion    string  `json:"firmware_version"`
	MAC                string  `json:"mac_address"`
	Type               string  `json:"type"`
	CurrentPowerWatts  float64 `json:"current_power_watts"`
	TodayEnergyKWh     float64 `json:"today_energy_kwh"`
	MonthEnergyKWh     float64 `json:"month_energy_kwh"`
	TodayRuntimeHours  float64 `json:"today_runtime_hours"`
	MonthRuntimeHours  float64 `json:"month_runtime_hours"`
	IsOnline           bool    `json:"is_online"`
	LastSeen           string  `json:"last_seen"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
}

// WebUIServer manages the web UI HTTP server
type WebUIServer struct {
	server             *http.Server
	deviceStates       map[string]*DeviceState
	deviceStatesMu     *sync.Mutex
	deviceMetadata     map[string]*DeviceMetadata
	deviceMetadataMu   *sync.RWMutex
	deviceCache        *DeviceCache
	config             *Config
}

// NewWebUIServer creates a new web UI server
func NewWebUIServer(
	host string,
	port int,
	deviceStates map[string]*DeviceState,
	deviceStatesMu *sync.Mutex,
	deviceMetadata map[string]*DeviceMetadata,
	deviceMetadataMu *sync.RWMutex,
	deviceCache *DeviceCache,
	config *Config,
) *WebUIServer {
	mux := http.NewServeMux()

	server := &WebUIServer{
		server: &http.Server{
			Addr:         fmt.Sprintf("%s:%d", host, port),
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		deviceStates:     deviceStates,
		deviceStatesMu:   deviceStatesMu,
		deviceMetadata:   deviceMetadata,
		deviceMetadataMu: deviceMetadataMu,
		deviceCache:      deviceCache,
		config:           config,
	}

	// Register routes
	mux.HandleFunc("/api/devices", server.handleGetDevices)
	mux.HandleFunc("/api/device/", server.handleGetDevice)
	mux.HandleFunc("/", server.handleIndex)

	return server
}

// Start starts the web UI server
func (s *WebUIServer) Start() error {
	logger.Info("Starting Web UI server on http://%s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the web UI server
func (s *WebUIServer) Shutdown() error {
	logger.Info("Shutting down Web UI server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// handleGetDevices returns all devices with their current data
func (s *WebUIServer) handleGetDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	devices := s.getDevicesData()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if err := json.NewEncoder(w).Encode(devices); err != nil {
		logger.Error("Failed to encode devices data: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleGetDevice returns data for a specific device
func (s *WebUIServer) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract IP from URL path
	ip := r.URL.Path[len("/api/device/"):]
	if ip == "" {
		http.Error(w, "Device IP required", http.StatusBadRequest)
		return
	}

	devices := s.getDevicesData()

	// Find the device
	for _, device := range devices {
		if device.IP == ip {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")

			if err := json.NewEncoder(w).Encode(device); err != nil {
				logger.Error("Failed to encode device data: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			return
		}
	}

	http.Error(w, "Device not found", http.StatusNotFound)
}

// handleIndex serves the main HTML page
func (s *WebUIServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		// Serve static files
		http.FileServer(http.FS(staticFiles)).ServeHTTP(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Serve the index.html file
	content, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		logger.Error("Failed to read index.html: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Write(content)
}

// getDevicesData aggregates device state, metadata, and cached energy data
func (s *WebUIServer) getDevicesData() []DeviceData {
	var devices []DeviceData

	s.deviceStatesMu.Lock()
	defer s.deviceStatesMu.Unlock()

	for ip, state := range s.deviceStates {
		state.mu.Lock()

		device := DeviceData{
			IP:                  ip,
			IsOnline:            state.IsOnline,
			LastSeen:            state.LastSeen.Format("2006-01-02 15:04:05"),
			ConsecutiveFailures: state.ConsecutiveFailures,
		}

		// Get metadata if available
		s.deviceMetadataMu.RLock()
		if metadata, exists := s.deviceMetadata[ip]; exists {
			metadata.mu.RLock()
			device.Name = metadata.Name
			device.Model = metadata.Model
			device.FirmwareVersion = metadata.FirmwareVer
			device.MAC = metadata.MAC
			device.Type = metadata.Type
			metadata.mu.RUnlock()
		}
		s.deviceMetadataMu.RUnlock()

		// Get cached energy data if available
		if s.deviceCache != nil {
			if energyData, found := s.deviceCache.Get(ip); found {
				device.CurrentPowerWatts = float64(energyData.Result.CurrentPower) / 1000.0
				device.TodayEnergyKWh = float64(energyData.Result.TodayEnergy) / 1000.0
				device.MonthEnergyKWh = float64(energyData.Result.MonthEnergy) / 1000.0
				device.TodayRuntimeHours = float64(energyData.Result.TodayRuntime) / 60.0
				device.MonthRuntimeHours = float64(energyData.Result.MonthRuntime) / 60.0
			}
		}

		devices = append(devices, device)
		state.mu.Unlock()
	}

	return devices
}
