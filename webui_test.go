// Copyright (c) 2025 Darren Soothill. All rights reserved.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func setupTestWebUIServer() *WebUIServer {
	// Create test device states
	deviceStates := map[string]*DeviceState{
		"192.168.1.100": {
			IP:                  "192.168.1.100",
			ConsecutiveFailures: 0,
			IsOnline:            true,
			LastSeen:            time.Now(),
			AlertSent:           false,
		},
		"192.168.1.101": {
			IP:                  "192.168.1.101",
			ConsecutiveFailures: 5,
			IsOnline:            false,
			LastSeen:            time.Now().Add(-10 * time.Minute),
			AlertSent:           true,
		},
	}
	deviceStatesMu := &sync.Mutex{}

	// Create test device metadata
	deviceMetadata := map[string]*DeviceMetadata{
		"192.168.1.100": {
			IP:          "192.168.1.100",
			Name:        "Living Room Plug",
			Model:       "P110",
			FirmwareVer: "1.0.5",
			MAC:         "AA:BB:CC:DD:EE:FF",
			Type:        "SMART.TAPOPLUG",
			LastUpdated: time.Now(),
		},
	}
	deviceMetadataMu := &sync.RWMutex{}

	// Create test device cache with energy data
	deviceCache := NewDeviceCache(60 * time.Second)
	deviceCache.Set("192.168.1.100", &EnergyUsageResponse{
		Result: struct {
			CurrentPower int `json:"current_power"`
			TodayEnergy  int `json:"today_energy"`
			MonthEnergy  int `json:"month_energy"`
			TodayRuntime int `json:"today_runtime"`
			MonthRuntime int `json:"month_runtime"`
		}{
			CurrentPower: 1500,  // 1.5W
			TodayEnergy:  250,   // 0.25kWh
			MonthEnergy:  5000,  // 5kWh
			TodayRuntime: 120,   // 2 hours
			MonthRuntime: 3600,  // 60 hours
		},
	})

	// Create test config
	config := &Config{
		WebUIEnabled: true,
		WebUIPort:    8080,
		WebUIHost:    "localhost",
	}

	return NewWebUIServer(
		config.WebUIHost,
		config.WebUIPort,
		deviceStates,
		deviceStatesMu,
		deviceMetadata,
		deviceMetadataMu,
		deviceCache,
		config,
	)
}

func TestHandleGetDevices(t *testing.T) {
	server := setupTestWebUIServer()

	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	w := httptest.NewRecorder()

	server.handleGetDevices(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if contentType := resp.Header.Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var devices []DeviceData
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(devices) != 2 {
		t.Errorf("Expected 2 devices, got %d", len(devices))
	}

	// Check online device
	var onlineDevice *DeviceData
	for i := range devices {
		if devices[i].IP == "192.168.1.100" {
			onlineDevice = &devices[i]
			break
		}
	}

	if onlineDevice == nil {
		t.Fatal("Online device not found in response")
	}

	if !onlineDevice.IsOnline {
		t.Error("Expected device to be online")
	}

	if onlineDevice.Name != "Living Room Plug" {
		t.Errorf("Expected device name 'Living Room Plug', got '%s'", onlineDevice.Name)
	}

	if onlineDevice.Model != "P110" {
		t.Errorf("Expected model 'P110', got '%s'", onlineDevice.Model)
	}

	if onlineDevice.CurrentPowerWatts != 1.5 {
		t.Errorf("Expected power 1.5W, got %.2fW", onlineDevice.CurrentPowerWatts)
	}

	if onlineDevice.TodayEnergyKWh != 0.25 {
		t.Errorf("Expected today energy 0.25kWh, got %.2fkWh", onlineDevice.TodayEnergyKWh)
	}

	// Check offline device
	var offlineDevice *DeviceData
	for i := range devices {
		if devices[i].IP == "192.168.1.101" {
			offlineDevice = &devices[i]
			break
		}
	}

	if offlineDevice == nil {
		t.Fatal("Offline device not found in response")
	}

	if offlineDevice.IsOnline {
		t.Error("Expected device to be offline")
	}

	if offlineDevice.ConsecutiveFailures != 5 {
		t.Errorf("Expected 5 consecutive failures, got %d", offlineDevice.ConsecutiveFailures)
	}
}

func TestHandleGetDevice(t *testing.T) {
	server := setupTestWebUIServer()

	// Test getting existing device
	req := httptest.NewRequest(http.MethodGet, "/api/device/192.168.1.100", nil)
	w := httptest.NewRecorder()

	server.handleGetDevice(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var device DeviceData
	if err := json.NewDecoder(resp.Body).Decode(&device); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if device.IP != "192.168.1.100" {
		t.Errorf("Expected IP 192.168.1.100, got %s", device.IP)
	}

	if device.Name != "Living Room Plug" {
		t.Errorf("Expected device name 'Living Room Plug', got '%s'", device.Name)
	}
}

func TestHandleGetDeviceNotFound(t *testing.T) {
	server := setupTestWebUIServer()

	req := httptest.NewRequest(http.MethodGet, "/api/device/192.168.1.999", nil)
	w := httptest.NewRecorder()

	server.handleGetDevice(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestHandleGetDevicesMethodNotAllowed(t *testing.T) {
	server := setupTestWebUIServer()

	req := httptest.NewRequest(http.MethodPost, "/api/devices", nil)
	w := httptest.NewRecorder()

	server.handleGetDevices(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", resp.StatusCode)
	}
}

func TestHandleIndex(t *testing.T) {
	server := setupTestWebUIServer()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	server.handleIndex(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if contentType := resp.Header.Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type text/html; charset=utf-8, got %s", contentType)
	}
}

func TestGetDevicesDataWithEmptyCache(t *testing.T) {
	// Create server with no cached data
	deviceStates := map[string]*DeviceState{
		"192.168.1.100": {
			IP:       "192.168.1.100",
			IsOnline: true,
			LastSeen: time.Now(),
		},
	}
	deviceStatesMu := &sync.Mutex{}
	deviceMetadata := map[string]*DeviceMetadata{}
	deviceMetadataMu := &sync.RWMutex{}

	config := &Config{
		WebUIEnabled: true,
		WebUIPort:    8080,
		WebUIHost:    "localhost",
	}

	server := NewWebUIServer(
		config.WebUIHost,
		config.WebUIPort,
		deviceStates,
		deviceStatesMu,
		deviceMetadata,
		deviceMetadataMu,
		nil, // No cache
		config,
	)

	devices := server.getDevicesData()

	if len(devices) != 1 {
		t.Errorf("Expected 1 device, got %d", len(devices))
	}

	if devices[0].IP != "192.168.1.100" {
		t.Errorf("Expected IP 192.168.1.100, got %s", devices[0].IP)
	}

	// Energy values should be 0 when no cache is available
	if devices[0].CurrentPowerWatts != 0 {
		t.Errorf("Expected 0W power, got %.2fW", devices[0].CurrentPowerWatts)
	}
}

func TestCORSHeaders(t *testing.T) {
	server := setupTestWebUIServer()

	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	w := httptest.NewRecorder()

	server.handleGetDevices(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if cors := resp.Header.Get("Access-Control-Allow-Origin"); cors != "*" {
		t.Errorf("Expected CORS header '*', got '%s'", cors)
	}
}

func TestDeviceDataJSONSerialization(t *testing.T) {
	device := DeviceData{
		IP:                 "192.168.1.100",
		Name:               "Test Device",
		Model:              "P110",
		FirmwareVersion:    "1.0.5",
		MAC:                "AA:BB:CC:DD:EE:FF",
		Type:               "SMART.TAPOPLUG",
		CurrentPowerWatts:  1.5,
		TodayEnergyKWh:     0.25,
		MonthEnergyKWh:     5.0,
		TodayRuntimeHours:  2.0,
		MonthRuntimeHours:  60.0,
		IsOnline:           true,
		LastSeen:           "2025-01-01 12:00:00",
		ConsecutiveFailures: 0,
	}

	jsonData, err := json.Marshal(device)
	if err != nil {
		t.Fatalf("Failed to marshal device: %v", err)
	}

	var decoded DeviceData
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal device: %v", err)
	}

	if decoded.IP != device.IP {
		t.Errorf("IP mismatch: expected %s, got %s", device.IP, decoded.IP)
	}

	if decoded.CurrentPowerWatts != device.CurrentPowerWatts {
		t.Errorf("Power mismatch: expected %.2f, got %.2f", device.CurrentPowerWatts, decoded.CurrentPowerWatts)
	}
}

func TestWebUIServerCreation(t *testing.T) {
	deviceStates := make(map[string]*DeviceState)
	deviceStatesMu := &sync.Mutex{}
	deviceMetadata := make(map[string]*DeviceMetadata)
	deviceMetadataMu := &sync.RWMutex{}

	config := &Config{
		WebUIEnabled: true,
		WebUIPort:    8888,
		WebUIHost:    "127.0.0.1",
	}

	server := NewWebUIServer(
		config.WebUIHost,
		config.WebUIPort,
		deviceStates,
		deviceStatesMu,
		deviceMetadata,
		deviceMetadataMu,
		nil,
		config,
	)

	if server == nil {
		t.Fatal("Expected server to be created, got nil")
	}

	if server.server.Addr != "127.0.0.1:8888" {
		t.Errorf("Expected server address 127.0.0.1:8888, got %s", server.server.Addr)
	}
}
