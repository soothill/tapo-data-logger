// Copyright (c) 2025 Darren Soothill. All rights reserved.

package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

// BenchmarkEncryption benchmarks the AES encryption performance
func BenchmarkEncryption(b *testing.B) {
	// Generate test data
	key := make([]byte, 16)
	iv := make([]byte, 16)
	rand.Read(key)
	rand.Read(iv)

	// Sample request data
	request := map[string]interface{}{
		"method": "get_energy_usage",
		"params": map[string]interface{}{},
	}
	jsonRequest, _ := json.Marshal(request)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Encrypt
		block, err := aes.NewCipher(key)
		if err != nil {
			b.Fatal(err)
		}

		paddedData := pkcs7Pad(jsonRequest, aes.BlockSize)
		mode := cipher.NewCBCEncrypter(block, iv)
		encrypted := make([]byte, len(paddedData))
		mode.CryptBlocks(encrypted, paddedData)
		_ = base64.StdEncoding.EncodeToString(encrypted)
	}
}

// BenchmarkDecryption benchmarks the AES decryption performance
func BenchmarkDecryption(b *testing.B) {
	// Generate test data
	key := make([]byte, 16)
	iv := make([]byte, 16)
	rand.Read(key)
	rand.Read(iv)

	// Create encrypted data
	request := map[string]interface{}{
		"method": "get_energy_usage",
		"params": map[string]interface{}{},
	}
	jsonRequest, _ := json.Marshal(request)

	block, _ := aes.NewCipher(key)
	paddedData := pkcs7Pad(jsonRequest, aes.BlockSize)
	mode := cipher.NewCBCEncrypter(block, iv)
	encrypted := make([]byte, len(paddedData))
	mode.CryptBlocks(encrypted, paddedData)
	encodedData := base64.StdEncoding.EncodeToString(encrypted)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Decrypt
		encryptedResponse, _ := base64.StdEncoding.DecodeString(encodedData)
		block, _ := aes.NewCipher(key)
		mode2 := cipher.NewCBCDecrypter(block, iv)
		decrypted := make([]byte, len(encryptedResponse))
		mode2.CryptBlocks(decrypted, encryptedResponse)
		_ = pkcs7Unpad(decrypted)
	}
}

// BenchmarkRSAKeyGeneration benchmarks RSA key generation performance
func BenchmarkRSAKeyGeneration(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := rsa.GenerateKey(rand.Reader, 1024)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPointBufferAdd benchmarks adding points to the buffer
func BenchmarkPointBufferAdd(b *testing.B) {
	// Initialize logger for benchmarks
	if logger == nil {
		logger = NewLogger("error")
	}

	config := &Config{
		BatchWriteSize:     100,
		BatchWriteInterval: 10,
	}

	// Create a dummy InfluxDB manager (won't actually write)
	managers := []*InfluxDBManager{}

	buffer := NewPointBuffer(config.BatchWriteSize, time.Duration(config.BatchWriteInterval)*time.Second, managers)

	// Create a sample point
	point := write.NewPoint(
		"energy",
		map[string]string{
			"device_ip": "192.168.1.100",
			"device_name": "test_device",
		},
		map[string]interface{}{
			"current_power": float64(100),
			"today_energy": float64(1000),
		},
		time.Now(),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buffer.Add(point)
	}
}

// BenchmarkPointBufferConcurrent benchmarks concurrent additions to the buffer
func BenchmarkPointBufferConcurrent(b *testing.B) {
	// Initialize logger for benchmarks
	if logger == nil {
		logger = NewLogger("error")
	}

	config := &Config{
		BatchWriteSize:     1000,
		BatchWriteInterval: 60,
	}

	managers := []*InfluxDBManager{}
	buffer := NewPointBuffer(config.BatchWriteSize, time.Duration(config.BatchWriteInterval)*time.Second, managers)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			point := write.NewPoint(
				"energy",
				map[string]string{
					"device_ip": "192.168.1.100",
					"device_name": "test_device",
				},
				map[string]interface{}{
					"current_power": float64(100),
					"today_energy": float64(1000),
				},
				time.Now(),
			)
			_ = buffer.Add(point)
		}
	})
}

// BenchmarkCreateInfluxPoint benchmarks creating InfluxDB points
func BenchmarkCreateInfluxPoint(b *testing.B) {
	energy := &EnergyUsageResponse{
		ErrorCode: 0,
		Result: struct {
			CurrentPower int `json:"current_power"`
			TodayEnergy  int `json:"today_energy"`
			MonthEnergy  int `json:"month_energy"`
			TodayRuntime int `json:"today_runtime"`
			MonthRuntime int `json:"month_runtime"`
		}{
			CurrentPower: 100,
			TodayEnergy:  1000,
			MonthEnergy:  30000,
			TodayRuntime: 120,
			MonthRuntime: 3600,
		},
	}

	metadata := &DeviceMetadata{
		Name:        "Test Device",
		Model:       "P110",
		FirmwareVer: "1.0.0",
		HardwareVer: "1.0",
		MAC:         "AA:BB:CC:DD:EE:FF",
		Type:        "SMART.TAPOPLUG",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = createInfluxPoint("192.168.1.100", energy, metadata)
	}
}

// BenchmarkDeviceCacheOperations benchmarks device cache operations
func BenchmarkDeviceCacheOperations(b *testing.B) {
	cache := NewDeviceCache(60) // 60 second TTL

	energy := &EnergyUsageResponse{
		ErrorCode: 0,
		Result: struct {
			CurrentPower int `json:"current_power"`
			TodayEnergy  int `json:"today_energy"`
			MonthEnergy  int `json:"month_energy"`
			TodayRuntime int `json:"today_runtime"`
			MonthRuntime int `json:"month_runtime"`
		}{
			CurrentPower: 100,
			TodayEnergy:  1000,
			MonthEnergy:  30000,
			TodayRuntime: 120,
			MonthRuntime: 3600,
		},
	}

	b.Run("Set", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cache.Set("192.168.1.100", energy)
		}
	})

	b.Run("Get", func(b *testing.B) {
		cache.Set("192.168.1.100", energy)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = cache.Get("192.168.1.100")
		}
	})
}

// BenchmarkRateLimiterAcquire benchmarks rate limiter acquire/release
func BenchmarkRateLimiterAcquire(b *testing.B) {
	limiter := NewRateLimiter(10) // 10 concurrent requests

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Acquire()
		limiter.Release()
	}
}

// BenchmarkRateLimiterConcurrent benchmarks concurrent rate limiter usage
func BenchmarkRateLimiterConcurrent(b *testing.B) {
	limiter := NewRateLimiter(100)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Acquire()
			limiter.Release()
		}
	})
}

// BenchmarkPKCS7Pad benchmarks PKCS7 padding
func BenchmarkPKCS7Pad(b *testing.B) {
	data := []byte("test data that needs padding for AES encryption")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pkcs7Pad(data, aes.BlockSize)
	}
}

// BenchmarkPKCS7Unpad benchmarks PKCS7 unpadding
func BenchmarkPKCS7Unpad(b *testing.B) {
	data := []byte("test data that needs padding for AES encryption")
	padded := pkcs7Pad(data, aes.BlockSize)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pkcs7Unpad(padded)
	}
}

// BenchmarkJSONMarshal benchmarks JSON marshaling of device data
func BenchmarkJSONMarshal(b *testing.B) {
	request := map[string]interface{}{
		"method": "get_energy_usage",
		"params": map[string]interface{}{
			"start_timestamp": 0,
			"end_timestamp":   time.Now().Unix(),
			"interval":        30,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(request)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJSONUnmarshal benchmarks JSON unmarshaling of device responses
func BenchmarkJSONUnmarshal(b *testing.B) {
	jsonData := `{
		"error_code": 0,
		"result": {
			"current_power": 100,
			"today_runtime": 120,
			"today_energy": 1000,
			"month_runtime": 3600,
			"month_energy": 30000
		}
	}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var resp struct {
			ErrorCode int `json:"error_code"`
			Result    EnergyUsageResponse `json:"result"`
		}
		err := json.Unmarshal([]byte(jsonData), &resp)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMemoryAllocation benchmarks memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	b.Run("SmallBuffers", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = make([]byte, 1024) // 1KB
		}
	})

	b.Run("MediumBuffers", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = make([]byte, 65536) // 64KB
		}
	})

	b.Run("PointAllocation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = write.NewPoint(
				"energy",
				map[string]string{
					"device_ip": "192.168.1.100",
					"device_name": "test_device",
				},
				map[string]interface{}{
					"current_power": float64(100),
					"today_energy": float64(1000),
				},
				time.Now(),
			)
		}
	})
}
