// Copyright (c) 2025 Darren Soothill. All rights reserved.

package main

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"
)

// TestPkcs7Pad tests the PKCS7 padding function
func TestPkcs7Pad(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		blockSize int
		wantLen   int
	}{
		{
			name:      "empty data with blocksize 16",
			data:      []byte{},
			blockSize: 16,
			wantLen:   16,
		},
		{
			name:      "data length equals blocksize",
			data:      []byte("1234567890123456"),
			blockSize: 16,
			wantLen:   32,
		},
		{
			name:      "data needs padding",
			data:      []byte("hello"),
			blockSize: 16,
			wantLen:   16,
		},
		{
			name:      "data length is 1 less than blocksize",
			data:      []byte("123456789012345"),
			blockSize: 16,
			wantLen:   16,
		},
		{
			name:      "blocksize 8",
			data:      []byte("test"),
			blockSize: 8,
			wantLen:   8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pkcs7Pad(tt.data, tt.blockSize)

			// Check length
			if len(result) != tt.wantLen {
				t.Errorf("pkcs7Pad() length = %d, want %d", len(result), tt.wantLen)
			}

			// Check that result is a multiple of blockSize
			if len(result)%tt.blockSize != 0 {
				t.Errorf("pkcs7Pad() result length %d is not a multiple of blockSize %d", len(result), tt.blockSize)
			}

			// Check that original data is preserved
			if len(tt.data) > 0 && string(result[:len(tt.data)]) != string(tt.data) {
				t.Errorf("pkcs7Pad() did not preserve original data")
			}

			// Check padding bytes are correct
			padding := result[len(result)-1]
			for i := len(result) - int(padding); i < len(result); i++ {
				if result[i] != padding {
					t.Errorf("pkcs7Pad() padding byte at position %d = %d, want %d", i, result[i], padding)
				}
			}
		})
	}
}

// TestPkcs7Unpad tests the PKCS7 unpadding function
func TestPkcs7Unpad(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []byte
	}{
		{
			name: "valid padding of 5",
			data: []byte("hello\x05\x05\x05\x05\x05"),
			want: []byte("hello"),
		},
		{
			name: "valid padding of 1",
			data: []byte("123456789012345\x01"),
			want: []byte("123456789012345"),
		},
		{
			name: "full block of padding",
			data: []byte("\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10"),
			want: []byte{},
		},
		{
			name: "empty data",
			data: []byte{},
			want: []byte{},
		},
		{
			name: "invalid padding - too large",
			data: []byte("hello\x20"),
			want: []byte("hello\x20"), // Should return original data
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pkcs7Unpad(tt.data)
			if string(result) != string(tt.want) {
				t.Errorf("pkcs7Unpad() = %v, want %v", result, tt.want)
			}
		})
	}
}

// TestPkcs7PadUnpadRoundTrip tests that padding and unpadding are inverse operations
func TestPkcs7PadUnpadRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		blockSize int
	}{
		{"empty", []byte{}, 16},
		{"short string", []byte("hello"), 16},
		{"exact block", []byte("1234567890123456"), 16},
		{"long string", []byte("this is a longer string to test padding"), 16},
		{"blocksize 8", []byte("test data"), 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			padded := pkcs7Pad(tt.data, tt.blockSize)
			unpadded := pkcs7Unpad(padded)

			if string(unpadded) != string(tt.data) {
				t.Errorf("Round trip failed: original = %v, after pad/unpad = %v", tt.data, unpadded)
			}
		})
	}
}

// TestInc tests the IP address incrementer
func TestInc(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "increment last octet",
			input: "192.168.1.1",
			want:  "192.168.1.2",
		},
		{
			name:  "increment with overflow to third octet",
			input: "192.168.1.255",
			want:  "192.168.2.0",
		},
		{
			name:  "increment with overflow to second octet",
			input: "192.168.255.255",
			want:  "192.169.0.0",
		},
		{
			name:  "increment with overflow to first octet",
			input: "192.255.255.255",
			want:  "193.0.0.0",
		},
		{
			name:  "increment zero address",
			input: "0.0.0.0",
			want:  "0.0.0.1",
		},
		{
			name:  "increment max address (wraps around)",
			input: "255.255.255.255",
			want:  "0.0.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.input).To4()
			if ip == nil {
				t.Fatalf("Failed to parse input IP: %s", tt.input)
			}

			inc(ip)

			got := ip.String()
			if got != tt.want {
				t.Errorf("inc(%s) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

// TestNewTapoClient tests the TapoClient constructor
func TestNewTapoClient(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		email    string
		password string
	}{
		{
			name:     "basic client creation",
			ip:       "192.168.1.100",
			email:    "test@example.com",
			password: "password123",
		},
		{
			name:     "empty credentials",
			ip:       "10.0.0.1",
			email:    "",
			password: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewTapoClient(tt.ip, tt.email, tt.password, 10*time.Second, 3)

			if client == nil {
				t.Fatal("NewTapoClient() returned nil")
			}

			if client.ip != tt.ip {
				t.Errorf("client.ip = %s, want %s", client.ip, tt.ip)
			}

			if client.email != tt.email {
				t.Errorf("client.email = %s, want %s", client.email, tt.email)
			}

			if client.password != tt.password {
				t.Errorf("client.password = %s, want %s", client.password, tt.password)
			}

			if client.client == nil {
				t.Error("client.client (http.Client) is nil")
			}

			if client.client.Timeout != 10*time.Second {
				t.Errorf("client.client.Timeout = %v, want %v", client.client.Timeout, 10*time.Second)
			}

			if client.token != "" {
				t.Errorf("client.token should be empty initially, got %s", client.token)
			}
		})
	}
}

// TestConfigUnmarshal tests unmarshaling of configuration JSON
func TestConfigUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    Config
		wantErr bool
	}{
		{
			name: "full config",
			json: `{
				"tapo_email": "test@example.com",
				"tapo_password": "password123",
				"plug_ips": ["192.168.1.100", "192.168.1.101"],
				"auto_discover": true,
				"discovery_method": "mdns",
				"scan_subnet": "192.168.1.0/24",
				"influx_url": "http://localhost:8086",
				"influx_token": "my-token",
				"influx_org": "my-org",
				"influx_bucket": "tapo",
				"poll_interval_seconds": 60
			}`,
			want: Config{
				TapoEmail:       "test@example.com",
				TapoPassword:    "password123",
				PlugIPs:         []string{"192.168.1.100", "192.168.1.101"},
				AutoDiscover:    true,
				DiscoveryMethod: "mdns",
				ScanSubnet:      "192.168.1.0/24",
				InfluxURL:       "http://localhost:8086",
				InfluxToken:     "my-token",
				InfluxOrg:       "my-org",
				InfluxBucket:    "tapo",
				PollInterval:    60,
			},
			wantErr: false,
		},
		{
			name: "minimal config",
			json: `{
				"tapo_email": "test@example.com",
				"tapo_password": "pass",
				"influx_url": "http://localhost:8086",
				"influx_token": "token",
				"influx_org": "org",
				"influx_bucket": "bucket"
			}`,
			want: Config{
				TapoEmail:    "test@example.com",
				TapoPassword: "pass",
				PlugIPs:      nil,
				InfluxURL:    "http://localhost:8086",
				InfluxToken:  "token",
				InfluxOrg:    "org",
				InfluxBucket: "bucket",
			},
			wantErr: false,
		},
		{
			name:    "invalid json",
			json:    `{invalid json}`,
			wantErr: true,
		},
		{
			name: "scan discovery method",
			json: `{
				"tapo_email": "test@example.com",
				"tapo_password": "pass",
				"discovery_method": "scan",
				"scan_subnet": "10.0.0.0/24",
				"influx_url": "http://localhost:8086",
				"influx_token": "token",
				"influx_org": "org",
				"influx_bucket": "bucket"
			}`,
			want: Config{
				TapoEmail:       "test@example.com",
				TapoPassword:    "pass",
				DiscoveryMethod: "scan",
				ScanSubnet:      "10.0.0.0/24",
				InfluxURL:       "http://localhost:8086",
				InfluxToken:     "token",
				InfluxOrg:       "org",
				InfluxBucket:    "bucket",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config Config
			err := json.Unmarshal([]byte(tt.json), &config)

			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if config.TapoEmail != tt.want.TapoEmail {
				t.Errorf("config.TapoEmail = %s, want %s", config.TapoEmail, tt.want.TapoEmail)
			}

			if config.TapoPassword != tt.want.TapoPassword {
				t.Errorf("config.TapoPassword = %s, want %s", config.TapoPassword, tt.want.TapoPassword)
			}

			if config.DiscoveryMethod != tt.want.DiscoveryMethod {
				t.Errorf("config.DiscoveryMethod = %s, want %s", config.DiscoveryMethod, tt.want.DiscoveryMethod)
			}

			if config.ScanSubnet != tt.want.ScanSubnet {
				t.Errorf("config.ScanSubnet = %s, want %s", config.ScanSubnet, tt.want.ScanSubnet)
			}

			if config.AutoDiscover != tt.want.AutoDiscover {
				t.Errorf("config.AutoDiscover = %v, want %v", config.AutoDiscover, tt.want.AutoDiscover)
			}

			if config.PollInterval != tt.want.PollInterval {
				t.Errorf("config.PollInterval = %d, want %d", config.PollInterval, tt.want.PollInterval)
			}
		})
	}
}

// TestDiscoverPlugsMethodSelection tests the discovery method selection logic
func TestDiscoverPlugsMethodSelection(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		subnet    string
		wantErr   bool
		errString string
	}{
		{
			name:    "scan method without subnet",
			method:  "scan",
			subnet:  "",
			wantErr: true,
			errString: "scan method requires subnet",
		},
		{
			name:      "invalid method",
			method:    "invalid",
			subnet:    "",
			wantErr:   true,
			errString: "unknown discovery method",
		},
		{
			name:    "scan method with invalid subnet",
			method:  "scan",
			subnet:  "invalid-subnet",
			wantErr: true,
			errString: "invalid subnet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DiscoverPlugs(context.Background(), tt.method, tt.subnet, 1*time.Second)

			if (err != nil) != tt.wantErr {
				t.Errorf("DiscoverPlugs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				errMsg := err.Error()
				if tt.errString != "" && !contains(errMsg, tt.errString) {
					t.Errorf("DiscoverPlugs() error = %v, want error containing %q", err, tt.errString)
				}
			}
		})
	}
}

// TestEnergyUsageResponseStructure tests the structure of energy usage response
func TestEnergyUsageResponseStructure(t *testing.T) {
	jsonData := `{
		"error_code": 0,
		"result": {
			"current_power": 5000,
			"today_energy": 1500,
			"month_energy": 45000,
			"today_runtime": 120,
			"month_runtime": 3600
		}
	}`

	var response EnergyUsageResponse
	err := json.Unmarshal([]byte(jsonData), &response)

	if err != nil {
		t.Fatalf("Failed to unmarshal EnergyUsageResponse: %v", err)
	}

	if response.ErrorCode != 0 {
		t.Errorf("ErrorCode = %d, want 0", response.ErrorCode)
	}

	if response.Result.CurrentPower != 5000 {
		t.Errorf("CurrentPower = %d, want 5000", response.Result.CurrentPower)
	}

	if response.Result.TodayEnergy != 1500 {
		t.Errorf("TodayEnergy = %d, want 1500", response.Result.TodayEnergy)
	}

	if response.Result.MonthEnergy != 45000 {
		t.Errorf("MonthEnergy = %d, want 45000", response.Result.MonthEnergy)
	}

	if response.Result.TodayRuntime != 120 {
		t.Errorf("TodayRuntime = %d, want 120", response.Result.TodayRuntime)
	}

	if response.Result.MonthRuntime != 3600 {
		t.Errorf("MonthRuntime = %d, want 3600", response.Result.MonthRuntime)
	}
}

// TestHandshakeResponseStructure tests the structure of handshake response
func TestHandshakeResponseStructure(t *testing.T) {
	jsonData := `{
		"error_code": 0,
		"result": {
			"key": "test-key-data"
		}
	}`

	var response HandshakeResponse
	err := json.Unmarshal([]byte(jsonData), &response)

	if err != nil {
		t.Fatalf("Failed to unmarshal HandshakeResponse: %v", err)
	}

	if response.ErrorCode != 0 {
		t.Errorf("ErrorCode = %d, want 0", response.ErrorCode)
	}

	if response.Result.Key != "test-key-data" {
		t.Errorf("Key = %s, want test-key-data", response.Result.Key)
	}
}

// TestSecurePassthroughResponseStructure tests the structure of secure passthrough response
func TestSecurePassthroughResponseStructure(t *testing.T) {
	jsonData := `{
		"error_code": 0,
		"result": {
			"response": "encrypted-response-data"
		}
	}`

	var response SecurePassthroughResponse
	err := json.Unmarshal([]byte(jsonData), &response)

	if err != nil {
		t.Fatalf("Failed to unmarshal SecurePassthroughResponse: %v", err)
	}

	if response.ErrorCode != 0 {
		t.Errorf("ErrorCode = %d, want 0", response.ErrorCode)
	}

	if response.Result.Response != "encrypted-response-data" {
		t.Errorf("Response = %s, want encrypted-response-data", response.Result.Response)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestSlackMessageStructure tests the Slack message JSON structure
func TestSlackMessageStructure(t *testing.T) {
	msg := SlackMessage{
		Attachments: []Attachment{
			{
				Color: "#ff0000",
				Title: "Test Alert",
				Fields: []Field{
					{
						Title: "Device IP",
						Value: "192.168.1.100",
						Short: true,
					},
					{
						Title: "Status",
						Value: "OFFLINE",
						Short: true,
					},
				},
			},
		},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal SlackMessage: %v", err)
	}

	// Unmarshal back to verify structure
	var decoded SlackMessage
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal SlackMessage: %v", err)
	}

	// Verify structure
	if len(decoded.Attachments) != 1 {
		t.Errorf("Attachments count = %d, want 1", len(decoded.Attachments))
	}

	if decoded.Attachments[0].Color != "#ff0000" {
		t.Errorf("Color = %s, want #ff0000", decoded.Attachments[0].Color)
	}

	if decoded.Attachments[0].Title != "Test Alert" {
		t.Errorf("Title = %s, want Test Alert", decoded.Attachments[0].Title)
	}

	if len(decoded.Attachments[0].Fields) != 2 {
		t.Errorf("Fields count = %d, want 2", len(decoded.Attachments[0].Fields))
	}
}

// TestDeviceStateTracking tests device state updates
func TestDeviceStateTracking(t *testing.T) {
	state := &DeviceState{
		IP:       "192.168.1.100",
		IsOnline: true,
		LastSeen: time.Now(),
	}

	// Simulate a failure
	state.mu.Lock()
	state.ConsecutiveFailures++
	state.IsOnline = false
	state.mu.Unlock()

	if state.ConsecutiveFailures != 1 {
		t.Errorf("ConsecutiveFailures = %d, want 1", state.ConsecutiveFailures)
	}

	if state.IsOnline {
		t.Error("IsOnline should be false after failure")
	}

	// Simulate recovery
	state.mu.Lock()
	state.ConsecutiveFailures = 0
	state.IsOnline = true
	state.LastSeen = time.Now()
	state.AlertSent = false
	state.mu.Unlock()

	if state.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures = %d, want 0 after recovery", state.ConsecutiveFailures)
	}

	if !state.IsOnline {
		t.Error("IsOnline should be true after recovery")
	}
}

// TestValidateConfigWithAlerts tests configuration validation with alerts enabled
func TestValidateConfigWithAlerts(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "alerts enabled with valid webhook",
			config: Config{
				TapoEmail:         "test@example.com",
				TapoPassword:      "password",
				PlugIPs:           []string{"192.168.1.100"},
				InfluxURL:         "http://localhost:8086",
				InfluxToken:       "token",
				InfluxOrg:         "org",
				InfluxBucket:      "bucket",
				AlertsEnabled:     true,
				SlackWebhookURL:   "https://hooks.slack.com/services/test",
				AlertAfterFailures: 3,
			},
			wantErr: false,
		},
		{
			name: "alerts enabled without webhook",
			config: Config{
				TapoEmail:      "test@example.com",
				TapoPassword:   "password",
				PlugIPs:        []string{"192.168.1.100"},
				InfluxURL:      "http://localhost:8086",
				InfluxToken:    "token",
				InfluxOrg:      "org",
				InfluxBucket:   "bucket",
				AlertsEnabled:  true,
				SlackWebhookURL: "",
			},
			wantErr: true,
		},
		{
			name: "alerts disabled without webhook",
			config: Config{
				TapoEmail:      "test@example.com",
				TapoPassword:   "password",
				PlugIPs:        []string{"192.168.1.100"},
				InfluxURL:      "http://localhost:8086",
				InfluxToken:    "token",
				InfluxOrg:      "org",
				InfluxBucket:   "bucket",
				AlertsEnabled:  false,
				SlackWebhookURL: "",
			},
			wantErr: false,
		},
		{
			name: "alerts enabled with zero threshold (should default to 3)",
			config: Config{
				TapoEmail:         "test@example.com",
				TapoPassword:      "password",
				PlugIPs:           []string{"192.168.1.100"},
				InfluxURL:         "http://localhost:8086",
				InfluxToken:       "token",
				InfluxOrg:         "org",
				InfluxBucket:      "bucket",
				AlertsEnabled:     true,
				SlackWebhookURL:   "https://hooks.slack.com/services/test",
				AlertAfterFailures: 0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(&tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check that alert_after_failures defaults to 3 if not set
			if tt.config.AlertsEnabled && tt.config.AlertAfterFailures == 0 && err == nil {
				if tt.config.AlertAfterFailures != 3 {
					t.Errorf("AlertAfterFailures should default to 3, got %d", tt.config.AlertAfterFailures)
				}
			}
		})
	}
}

// TestSendSlackNotificationFormat tests Slack notification message formatting
func TestSendSlackNotificationFormat(t *testing.T) {
	// This test verifies the message structure without actually sending
	// We'll marshal a message and verify the JSON structure

	tests := []struct {
		name     string
		deviceIP string
		status   string
		message  string
		wantColor string
	}{
		{
			name:     "offline notification",
			deviceIP: "192.168.1.100",
			status:   "offline",
			message:  "Device has been offline",
			wantColor: "#ff0000",
		},
		{
			name:     "online notification",
			deviceIP: "192.168.1.101",
			status:   "online",
			message:  "Device is back online",
			wantColor: "#36a64f",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create message structure similar to sendSlackNotification
			color := "#36a64f" // Green for online
			if tt.status == "offline" {
				color = "#ff0000" // Red for offline
			}

			slackMsg := SlackMessage{
				Attachments: []Attachment{
					{
						Color: color,
						Title: "Tapo Device " + tt.status + " Alert",
						Fields: []Field{
							{Title: "Device IP", Value: tt.deviceIP, Short: true},
							{Title: "Status", Value: tt.status, Short: true},
							{Title: "Message", Value: tt.message, Short: false},
						},
					},
				},
			}

			// Marshal to verify structure
			jsonData, err := json.Marshal(slackMsg)
			if err != nil {
				t.Fatalf("Failed to marshal message: %v", err)
			}

			// Verify JSON contains expected fields
			jsonStr := string(jsonData)
			if !contains(jsonStr, tt.deviceIP) {
				t.Errorf("JSON doesn't contain device IP %s", tt.deviceIP)
			}

			if !contains(jsonStr, tt.wantColor) {
				t.Errorf("JSON doesn't contain expected color %s", tt.wantColor)
			}

			if !contains(jsonStr, tt.status) {
				t.Errorf("JSON doesn't contain status %s", tt.status)
			}
		})
	}
}

// TestLogger tests the Logger functionality
func TestLogger(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{"debug level", "debug"},
		{"info level", "info"},
		{"warn level", "warn"},
		{"error level", "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := NewLogger(tt.level)
			if log == nil {
				t.Fatal("NewLogger returned nil")
			}

			// Test all log methods
			log.Debug("debug message")
			log.Info("info message")
			log.Warn("warn message")
			log.Error("error message")
		})
	}
}

// TestDeviceCache tests the DeviceCache functionality
func TestDeviceCache(t *testing.T) {
	cache := NewDeviceCache(5 * time.Minute)
	if cache == nil {
		t.Fatal("NewDeviceCache returned nil")
	}

	ip := "192.168.1.100"

	// Test Get on non-existent key
	data, exists := cache.Get(ip)
	if exists {
		t.Errorf("Get on non-existent key should return false, got true")
	}
	if data != nil {
		t.Errorf("Get on non-existent key should return nil data, got %v", data)
	}

	// Test Set
	energyData := &EnergyUsageResponse{
		ErrorCode: 0,
	}
	energyData.Result.CurrentPower = 5000
	energyData.Result.TodayEnergy = 2500
	energyData.Result.MonthEnergy = 75000
	cache.Set(ip, energyData)

	// Test Get on existing key
	retrieved, exists := cache.Get(ip)
	if !exists {
		t.Fatal("Get on existing key returned false")
	}
	if retrieved == nil {
		t.Fatal("Get on existing key returned nil data")
	}

	if retrieved.Result.CurrentPower != 5000 {
		t.Errorf("Retrieved CurrentPower = %d, want 5000", retrieved.Result.CurrentPower)
	}
}

// TestDeviceCacheExpiry tests cache expiration
func TestDeviceCacheExpiry(t *testing.T) {
	cache := NewDeviceCache(100 * time.Millisecond)

	ip := "192.168.1.100"
	energyData := &EnergyUsageResponse{
		ErrorCode: 0,
	}
	energyData.Result.CurrentPower = 5000
	cache.Set(ip, energyData)

	// Should exist immediately
	_, exists := cache.Get(ip)
	if !exists {
		t.Error("Data should exist immediately after set")
	}

	// Wait for expiry
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	_, exists = cache.Get(ip)
	if exists {
		t.Error("Data should be expired after TTL")
	}
}

// TestPointBuffer tests the PointBuffer functionality
// Note: PointBuffer requires InfluxDBManager instances, so we test creation only
func TestPointBuffer(t *testing.T) {
	// Test with empty managers list
	buffer := NewPointBuffer(100, 5*time.Second, []*InfluxDBManager{})
	if buffer == nil {
		t.Fatal("NewPointBuffer returned nil")
	}

	// Flush should not panic with empty managers
	buffer.Flush()
}

// TestRateLimiter tests the RateLimiter functionality
func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(2)
	if limiter == nil {
		t.Fatal("NewRateLimiter returned nil")
	}

	// Acquire should not panic
	limiter.Acquire()
	limiter.Acquire()

	// Release should not panic
	limiter.Release()
	limiter.Release()
}

// TestRateLimiterNil tests rate limiter with no limit
func TestRateLimiterNil(t *testing.T) {
	limiter := NewRateLimiter(0)
	if limiter != nil {
		t.Error("NewRateLimiter(0) should return nil")
	}

	limiter = NewRateLimiter(-1)
	if limiter != nil {
		t.Error("NewRateLimiter(-1) should return nil")
	}
}

// TestCreateInfluxPoint tests InfluxDB point creation
func TestCreateInfluxPoint(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		energy   *EnergyUsageResponse
		metadata *DeviceMetadata
	}{
		{
			name: "basic point with metadata",
			ip:   "192.168.1.100",
			energy: &EnergyUsageResponse{
				ErrorCode: 0,
			},
			metadata: &DeviceMetadata{
				Name:     "Living Room",
				Model:    "P110",
				DeviceID: "device-123",
			},
		},
		{
			name: "point without metadata",
			ip:   "10.0.0.1",
			energy: &EnergyUsageResponse{
				ErrorCode: 0,
			},
			metadata: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set energy values
			tt.energy.Result.CurrentPower = 5000
			tt.energy.Result.TodayEnergy = 2500
			tt.energy.Result.MonthEnergy = 75000
			tt.energy.Result.TodayRuntime = 180
			tt.energy.Result.MonthRuntime = 5400

			point := createInfluxPoint(tt.ip, tt.energy, tt.metadata)

			if point == nil {
				t.Fatal("createInfluxPoint returned nil")
			}

			// Point should have been created successfully
			// In a real test with InfluxDB client, we'd verify the fields
		})
	}
}

// TestValidateConfigComprehensive tests ValidateConfig with more scenarios
func TestValidateConfigComprehensive(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "missing tapo email",
			config: Config{
				TapoPassword: "password",
				InfluxURL:    "http://localhost:8086",
				InfluxToken:  "token",
				InfluxOrg:    "org",
				InfluxBucket: "bucket",
			},
			wantErr: true,
			errMsg:  "tapo_email",
		},
		{
			name: "missing tapo password",
			config: Config{
				TapoEmail:    "test@example.com",
				InfluxURL:    "http://localhost:8086",
				InfluxToken:  "token",
				InfluxOrg:    "org",
				InfluxBucket: "bucket",
			},
			wantErr: true,
			errMsg:  "tapo_password",
		},
		{
			name: "missing influx url",
			config: Config{
				TapoEmail:    "test@example.com",
				TapoPassword: "password",
				InfluxToken:  "token",
				InfluxOrg:    "org",
				InfluxBucket: "bucket",
			},
			wantErr: true,
			errMsg:  "influx_url",
		},
		{
			name: "missing influx token",
			config: Config{
				TapoEmail:    "test@example.com",
				TapoPassword: "password",
				InfluxURL:    "http://localhost:8086",
				InfluxOrg:    "org",
				InfluxBucket: "bucket",
			},
			wantErr: true,
			errMsg:  "influx_token",
		},
		{
			name: "missing influx org",
			config: Config{
				TapoEmail:    "test@example.com",
				TapoPassword: "password",
				InfluxURL:    "http://localhost:8086",
				InfluxToken:  "token",
				InfluxBucket: "bucket",
			},
			wantErr: true,
			errMsg:  "influx_org",
		},
		{
			name: "missing influx bucket",
			config: Config{
				TapoEmail:    "test@example.com",
				TapoPassword: "password",
				InfluxURL:    "http://localhost:8086",
				InfluxToken:  "token",
				InfluxOrg:    "org",
			},
			wantErr: true,
			errMsg:  "influx_bucket",
		},
		{
			name: "no device sources",
			config: Config{
				TapoEmail:    "test@example.com",
				TapoPassword: "password",
				InfluxURL:    "http://localhost:8086",
				InfluxToken:  "token",
				InfluxOrg:    "org",
				InfluxBucket: "bucket",
				AutoDiscover: false,
				PlugIPs:      []string{},
			},
			wantErr: true,
			errMsg:  "auto_discover",
		},
		{
			name: "valid with autodiscover",
			config: Config{
				TapoEmail:    "test@example.com",
				TapoPassword: "password",
				InfluxURL:    "http://localhost:8086",
				InfluxToken:  "token",
				InfluxOrg:    "org",
				InfluxBucket: "bucket",
				AutoDiscover: true,
			},
			wantErr: false,
		},
		{
			name: "valid with plug IPs",
			config: Config{
				TapoEmail:    "test@example.com",
				TapoPassword: "password",
				InfluxURL:    "http://localhost:8086",
				InfluxToken:  "token",
				InfluxOrg:    "org",
				InfluxBucket: "bucket",
				PlugIPs:      []string{"192.168.1.100"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(&tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateConfig() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

// TestIsTapoDevice tests the isTapoDevice function
func TestIsTapoDevice(t *testing.T) {
	// Start a mock server
	mock := NewMockTapoDevice()
	defer mock.Close()

	// Test with valid Tapo device
	if !isTapoDevice(mock.GetIP(), 2*time.Second) {
		t.Error("isTapoDevice should return true for mock Tapo device")
	}

	// Test with invalid IP
	if isTapoDevice("192.0.2.1", 1*time.Second) {
		t.Error("isTapoDevice should return false for non-existent device")
	}
}

// TestRetryWithBackoff tests the retry mechanism
func TestRetryWithBackoff(t *testing.T) {
	setupTest()
	client := NewTapoClient("192.168.1.100", "test@example.com", "password", 5*time.Second, 3)

	// Test successful operation after retries
	attempts := 0
	err := client.retryWithBackoff(context.Background(), func() error {
		attempts++
		if attempts < 2 {
			return context.DeadlineExceeded
		}
		return nil
	}, "test operation")

	if err != nil {
		t.Errorf("retryWithBackoff should succeed, got error: %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}

	// Test operation that always fails
	attempts = 0
	err = client.retryWithBackoff(context.Background(), func() error {
		attempts++
		return context.DeadlineExceeded
	}, "failing operation")

	if err == nil {
		t.Error("retryWithBackoff should fail after max retries")
	}

	if attempts != 4 {
		t.Errorf("Expected 4 attempts (1 initial + 3 retries), got %d", attempts)
	}

	// Test context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = client.retryWithBackoff(ctx, func() error {
		return context.DeadlineExceeded
	}, "cancelled operation")

	if err != context.Canceled {
		t.Errorf("retryWithBackoff should return context.Canceled, got %v", err)
	}
}
