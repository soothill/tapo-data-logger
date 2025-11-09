// Copyright (c) 2025 soothill. All rights reserved.

package main

import (
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
			client := NewTapoClient(tt.ip, tt.email, tt.password)

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
			_, err := DiscoverPlugs(tt.method, tt.subnet, 1*time.Second)

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
