// Copyright (c) 2025 Darren Soothill. All rights reserved.

package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/grandcat/zeroconf"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

type Config struct {
	TapoEmail       string   `json:"tapo_email"`
	TapoPassword    string   `json:"tapo_password"`
	PlugIPs         []string `json:"plug_ips"`
	AutoDiscover    bool     `json:"auto_discover"`
	DiscoveryMethod string   `json:"discovery_method"` // "mdns", "scan", or "both"
	ScanSubnet      string   `json:"scan_subnet"`      // e.g., "192.168.1.0/24"
	InfluxURL       string   `json:"influx_url"`
	InfluxToken     string   `json:"influx_token"`
	InfluxOrg       string   `json:"influx_org"`
	InfluxBucket    string   `json:"influx_bucket"`
	PollInterval    int      `json:"poll_interval_seconds"`
	LogLevel        string   `json:"log_level"`        // "debug", "info", "warn", "error"
	RequestTimeout  int      `json:"request_timeout_seconds"`
	MaxRetries      int      `json:"max_retries"`
}

// LogLevel constants
const (
	LogLevelDebug = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// Global logger
var (
	logger    *Logger
	logPrefix = ""
)

// Logger provides structured logging with levels
type Logger struct {
	level int
	mu    sync.Mutex
}

func NewLogger(levelStr string) *Logger {
	level := LogLevelInfo
	switch strings.ToLower(levelStr) {
	case "debug":
		level = LogLevelDebug
	case "info":
		level = LogLevelInfo
	case "warn", "warning":
		level = LogLevelWarn
	case "error":
		level = LogLevelError
	}
	return &Logger{level: level}
}

func (l *Logger) Debug(format string, v ...interface{}) {
	if l.level <= LogLevelDebug {
		l.log("DEBUG", format, v...)
	}
}

func (l *Logger) Info(format string, v ...interface{}) {
	if l.level <= LogLevelInfo {
		l.log("INFO", format, v...)
	}
}

func (l *Logger) Warn(format string, v ...interface{}) {
	if l.level <= LogLevelWarn {
		l.log("WARN", format, v...)
	}
}

func (l *Logger) Error(format string, v ...interface{}) {
	if l.level <= LogLevelError {
		l.log("ERROR", format, v...)
	}
}

func (l *Logger) log(level, format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, v...)
	log.Printf("[%s] %s: %s", timestamp, level, msg)
}

// ValidateConfig validates the configuration
func ValidateConfig(config *Config) error {
	if config.TapoEmail == "" {
		return fmt.Errorf("tapo_email is required")
	}
	if config.TapoPassword == "" {
		return fmt.Errorf("tapo_password is required")
	}
	if config.InfluxURL == "" {
		return fmt.Errorf("influx_url is required")
	}
	if config.InfluxToken == "" {
		return fmt.Errorf("influx_token is required")
	}
	if config.InfluxOrg == "" {
		return fmt.Errorf("influx_org is required")
	}
	if config.InfluxBucket == "" {
		return fmt.Errorf("influx_bucket is required")
	}
	if config.PollInterval <= 0 {
		config.PollInterval = 60 // Default to 60 seconds
	}
	if config.LogLevel == "" {
		config.LogLevel = "info" // Default to info
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 10 // Default to 10 seconds
	}
	if config.MaxRetries < 0 {
		config.MaxRetries = 3 // Default to 3 retries
	}
	if config.AutoDiscover {
		if config.DiscoveryMethod == "" {
			config.DiscoveryMethod = "both"
		}
		validMethods := map[string]bool{"mdns": true, "scan": true, "both": true}
		if !validMethods[config.DiscoveryMethod] {
			return fmt.Errorf("invalid discovery_method: %s (must be 'mdns', 'scan', or 'both')", config.DiscoveryMethod)
		}
		if config.DiscoveryMethod == "scan" && config.ScanSubnet == "" {
			return fmt.Errorf("scan_subnet is required when discovery_method is 'scan'")
		}
	}
	if !config.AutoDiscover && len(config.PlugIPs) == 0 {
		return fmt.Errorf("either auto_discover must be true or plug_ips must be provided")
	}
	return nil
}

type TapoClient struct {
	ip           string
	email        string
	password     string
	token        string
	tokenExpiry  time.Time
	cookies      []*http.Cookie
	client       *http.Client
	maxRetries   int
	mu           sync.Mutex
	privateKey   *rsa.PrivateKey
	serverPubKey *rsa.PublicKey
}

type HandshakeResponse struct {
	ErrorCode int `json:"error_code"`
	Result    struct {
		Key string `json:"key"`
	} `json:"result"`
}

type SecurePassthroughResponse struct {
	ErrorCode int `json:"error_code"`
	Result    struct {
		Response string `json:"response"`
	} `json:"result"`
}

type EnergyUsageResponse struct {
	ErrorCode int `json:"error_code"`
	Result    struct {
		CurrentPower int `json:"current_power"` // in milliwatts
		TodayEnergy  int `json:"today_energy"`  // in watt-hours
		MonthEnergy  int `json:"month_energy"`  // in watt-hours
		TodayRuntime int `json:"today_runtime"` // in minutes
		MonthRuntime int `json:"month_runtime"` // in minutes
	} `json:"result"`
}

func NewTapoClient(ip, email, password string, timeout time.Duration, maxRetries int) *TapoClient {
	return &TapoClient{
		ip:         ip,
		email:      email,
		password:   password,
		client:     &http.Client{Timeout: timeout},
		maxRetries: maxRetries,
	}
}

// retryWithBackoff retries a function with exponential backoff
func (t *TapoClient) retryWithBackoff(ctx context.Context, operation func() error, operationName string) error {
	var lastErr error
	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			logger.Debug("[%s] Retry attempt %d/%d after %v for %s", t.ip, attempt, t.maxRetries, backoff, operationName)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		err := operation()
		if err == nil {
			if attempt > 0 {
				logger.Info("[%s] %s succeeded after %d retries", t.ip, operationName, attempt)
			}
			return nil
		}

		lastErr = err
		logger.Debug("[%s] %s failed (attempt %d/%d): %v", t.ip, operationName, attempt+1, t.maxRetries+1, err)
	}

	return fmt.Errorf("%s failed after %d retries: %w", operationName, t.maxRetries+1, lastErr)
}

// needsTokenRefresh checks if the token needs to be refreshed
func (t *TapoClient) needsTokenRefresh() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.token == "" || time.Now().After(t.tokenExpiry)
}

// ensureAuthenticated ensures the client is authenticated, refreshing if needed
func (t *TapoClient) ensureAuthenticated(ctx context.Context) error {
	if !t.needsTokenRefresh() {
		return nil
	}

	logger.Debug("[%s] Token needs refresh, re-authenticating", t.ip)
	return t.HandshakeWithContext(ctx)
}

func (t *TapoClient) Handshake() error {
	return t.HandshakeWithContext(context.Background())
}

func (t *TapoClient) HandshakeWithContext(ctx context.Context) error {
	return t.retryWithBackoff(ctx, func() error {
		return t.doHandshake(ctx)
	}, "handshake")
}

func (t *TapoClient) doHandshake(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Export public key
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to marshal public key: %w", err)
	}

	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	// Send handshake request
	handshakePayload := map[string]interface{}{
		"method": "handshake",
		"params": map[string]string{
			"key": string(pubKeyPEM),
		},
	}

	body, err := json.Marshal(handshakePayload)
	if err != nil {
		return fmt.Errorf("failed to marshal handshake payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("http://%s/app", t.ip), bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("handshake request failed: %w", err)
	}
	defer resp.Body.Close()

	var handshakeResp HandshakeResponse
	if err := json.NewDecoder(resp.Body).Decode(&handshakeResp); err != nil {
		return fmt.Errorf("failed to decode handshake response: %w", err)
	}

	if handshakeResp.ErrorCode != 0 {
		return fmt.Errorf("handshake failed with error code: %d", handshakeResp.ErrorCode)
	}

	// Decode the server's key
	keyBlock, _ := pem.Decode([]byte(handshakeResp.Result.Key))
	if keyBlock == nil {
		return fmt.Errorf("failed to decode server key")
	}

	serverKey, err := x509.ParsePKIXPublicKey(keyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse server key: %w", err)
	}

	serverPubKey, ok := serverKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("server key is not RSA public key")
	}

	// Store cookies for session
	t.cookies = resp.Cookies()
	t.privateKey = privateKey
	t.serverPubKey = serverPubKey

	// Now login with encrypted credentials
	return t.doLogin(ctx)
}

func (t *TapoClient) doLogin(ctx context.Context) error {
	// This method assumes mu is already locked by the caller
	loginPayload := map[string]string{
		"method":          "login_device",
		"params":          fmt.Sprintf(`{"username":"%s","password":"%s"}`, base64.StdEncoding.EncodeToString([]byte(t.email)), base64.StdEncoding.EncodeToString([]byte(t.password))),
		"requestTimeMils": fmt.Sprintf("%d", time.Now().UnixMilli()),
	}

	jsonPayload, _ := json.Marshal(loginPayload)

	// Encrypt with server's public key
	encryptedPayload, err := rsa.EncryptPKCS1v15(rand.Reader, t.serverPubKey, jsonPayload)
	if err != nil {
		return fmt.Errorf("failed to encrypt login payload: %w", err)
	}

	securePayload := map[string]interface{}{
		"method": "securePassthrough",
		"params": map[string]string{
			"request": base64.StdEncoding.EncodeToString(encryptedPayload),
		},
	}

	body, _ := json.Marshal(securePayload)

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("http://%s/app", t.ip), bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range t.cookies {
		req.AddCookie(cookie)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	var secureResp SecurePassthroughResponse
	if err := json.NewDecoder(resp.Body).Decode(&secureResp); err != nil {
		return fmt.Errorf("failed to decode login response: %w", err)
	}

	if secureResp.ErrorCode != 0 {
		return fmt.Errorf("login failed with error code: %d", secureResp.ErrorCode)
	}

	// Decrypt response
	encryptedResponse, _ := base64.StdEncoding.DecodeString(secureResp.Result.Response)
	decryptedResponse, err := rsa.DecryptPKCS1v15(rand.Reader, t.privateKey, encryptedResponse)
	if err != nil {
		return fmt.Errorf("failed to decrypt login response: %w", err)
	}

	var loginResult map[string]interface{}
	if err := json.Unmarshal(decryptedResponse, &loginResult); err != nil {
		return fmt.Errorf("failed to unmarshal login result: %w", err)
	}

	if token, ok := loginResult["result"].(map[string]interface{})["token"].(string); ok {
		t.token = token
		// Tokens typically expire after 24 hours, set expiry to 23 hours to be safe
		t.tokenExpiry = time.Now().Add(23 * time.Hour)
	} else {
		return fmt.Errorf("failed to extract token from login response")
	}

	t.cookies = resp.Cookies()

	return nil
}

func (t *TapoClient) GetEnergyUsage() (*EnergyUsageResponse, error) {
	return t.GetEnergyUsageWithContext(context.Background())
}

func (t *TapoClient) GetEnergyUsageWithContext(ctx context.Context) (*EnergyUsageResponse, error) {
	// Ensure we have a valid token
	if err := t.ensureAuthenticated(ctx); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	var result *EnergyUsageResponse
	err := t.retryWithBackoff(ctx, func() error {
		var err error
		result, err = t.doGetEnergyUsage(ctx)

		// If we get an authentication error, try refreshing the token once
		if err != nil && strings.Contains(err.Error(), "error code: -1001") {
			logger.Debug("[%s] Authentication error detected, refreshing token", t.ip)
			if refreshErr := t.HandshakeWithContext(ctx); refreshErr != nil {
				return fmt.Errorf("token refresh failed: %w", refreshErr)
			}
			result, err = t.doGetEnergyUsage(ctx)
		}

		return err
	}, "get_energy_usage")

	return result, err
}

func (t *TapoClient) doGetEnergyUsage(ctx context.Context) (*EnergyUsageResponse, error) {
	// Create symmetric key for AES encryption
	key := make([]byte, 16)
	iv := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	// Create request
	request := map[string]interface{}{
		"method":          "get_energy_usage",
		"requestTimeMils": time.Now().UnixMilli(),
	}

	jsonRequest, _ := json.Marshal(request)

	// Encrypt request
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	padded := pkcs7Pad(jsonRequest, aes.BlockSize)
	encrypted := make([]byte, len(padded))
	mode.CryptBlocks(encrypted, padded)

	securePayload := map[string]interface{}{
		"method": "securePassthrough",
		"params": map[string]string{
			"request": base64.StdEncoding.EncodeToString(encrypted),
		},
	}

	body, _ := json.Marshal(securePayload)

	t.mu.Lock()
	token := t.token
	cookies := t.cookies
	t.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("http://%s/app?token=%s", t.ip, token), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var secureResp SecurePassthroughResponse
	if err := json.NewDecoder(resp.Body).Decode(&secureResp); err != nil {
		return nil, err
	}

	if secureResp.ErrorCode != 0 {
		return nil, fmt.Errorf("get energy usage failed with error code: %d", secureResp.ErrorCode)
	}

	// Decrypt response
	encryptedResponse, _ := base64.StdEncoding.DecodeString(secureResp.Result.Response)

	block, _ = aes.NewCipher(key)
	mode2 := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(encryptedResponse))
	mode2.CryptBlocks(decrypted, encryptedResponse)

	decrypted = pkcs7Unpad(decrypted)

	var energyResp EnergyUsageResponse
	if err := json.Unmarshal(decrypted, &energyResp); err != nil {
		return nil, err
	}

	return &energyResp, nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padtext...)
}

func pkcs7Unpad(data []byte) []byte {
	length := len(data)
	if length == 0 {
		return data
	}
	padding := int(data[length-1])
	if padding > length {
		return data
	}
	return data[:length-padding]
}

// DiscoverPlugsMDNS discovers Tapo plugs using mDNS/Bonjour
func DiscoverPlugsMDNS(ctx context.Context, timeout time.Duration) ([]string, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize mDNS resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	var plugIPs []string
	var mu sync.Mutex

	go func() {
		for entry := range entries {
			// Tapo devices typically advertise with _hap._tcp or custom service types
			// Check for TP-Link/Tapo in the hostname or service info
			if strings.Contains(strings.ToLower(entry.HostName), "tapo") ||
				strings.Contains(strings.ToLower(entry.Instance), "tapo") {

				mu.Lock()
				for _, ip := range entry.AddrIPv4 {
					ipStr := ip.String()
					logger.Info("Discovered Tapo device via mDNS: %s (%s)", entry.Instance, ipStr)
					plugIPs = append(plugIPs, ipStr)
				}
				mu.Unlock()
			}
		}
	}()

	discCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Browse for common service types
	serviceTypes := []string{"_hap._tcp", "_http._tcp", "_tapo._tcp"}

	for _, serviceType := range serviceTypes {
		if err := resolver.Browse(discCtx, serviceType, "local.", entries); err != nil {
			logger.Warn("Failed to browse for %s: %v", serviceType, err)
		}
	}

	<-discCtx.Done()
	close(entries)

	return plugIPs, nil
}

// DiscoverPlugsScan discovers Tapo plugs by scanning a subnet
func DiscoverPlugsScan(ctx context.Context, subnet string, timeout time.Duration) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return nil, fmt.Errorf("invalid subnet: %w", err)
	}

	var plugIPs []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Scan all IPs in the subnet
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		ipStr := ip.String()

		select {
		case <-ctx.Done():
			wg.Wait()
			return plugIPs, ctx.Err()
		default:
		}

		wg.Add(1)
		go func(ip string) {
			defer wg.Done()

			if isTapoDevice(ip, timeout) {
				mu.Lock()
				logger.Info("Discovered Tapo device via scan: %s", ip)
				plugIPs = append(plugIPs, ip)
				mu.Unlock()
			}
		}(ipStr)
	}

	wg.Wait()
	return plugIPs, nil
}

// inc increments an IP address
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// isTapoDevice checks if an IP is a Tapo device by attempting a connection
func isTapoDevice(ip string, timeout time.Duration) bool {
	// Try to connect to the Tapo HTTP endpoint
	client := &http.Client{Timeout: timeout}
	
	// Send a simple handshake request to see if it's a Tapo device
	testPayload := map[string]interface{}{
		"method": "handshake",
		"params": map[string]string{
			"key": "test",
		},
	}
	
	body, _ := json.Marshal(testPayload)
	
	resp, err := client.Post(
		fmt.Sprintf("http://%s/app", ip),
		"application/json",
		bytes.NewBuffer(body),
	)
	
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	// Check if the response looks like a Tapo device
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}
	
	// Tapo devices will respond with an error_code field
	_, hasErrorCode := result["error_code"]
	return hasErrorCode
}

// DiscoverPlugs discovers Tapo plugs using the specified method(s)
func DiscoverPlugs(ctx context.Context, method string, subnet string, timeout time.Duration) ([]string, error) {
	var allPlugs []string
	seen := make(map[string]bool)

	switch method {
	case "mdns":
		plugs, err := DiscoverPlugsMDNS(ctx, timeout)
		if err != nil {
			return nil, fmt.Errorf("mDNS discovery failed: %w", err)
		}
		for _, ip := range plugs {
			if !seen[ip] {
				allPlugs = append(allPlugs, ip)
				seen[ip] = true
			}
		}

	case "scan":
		if subnet == "" {
			return nil, fmt.Errorf("scan method requires subnet to be specified")
		}
		plugs, err := DiscoverPlugsScan(ctx, subnet, timeout)
		if err != nil {
			return nil, fmt.Errorf("network scan failed: %w", err)
		}
		for _, ip := range plugs {
			if !seen[ip] {
				allPlugs = append(allPlugs, ip)
				seen[ip] = true
			}
		}

	case "both":
		// Try mDNS first
		plugs, err := DiscoverPlugsMDNS(ctx, timeout)
		if err != nil {
			logger.Warn("mDNS discovery failed: %v", err)
		} else {
			for _, ip := range plugs {
				if !seen[ip] {
					allPlugs = append(allPlugs, ip)
					seen[ip] = true
				}
			}
		}

		// Then try scanning if subnet is provided
		if subnet != "" {
			plugs, err := DiscoverPlugsScan(ctx, subnet, timeout)
			if err != nil {
				logger.Warn("Network scan failed: %v", err)
			} else {
				for _, ip := range plugs {
					if !seen[ip] {
						allPlugs = append(allPlugs, ip)
						seen[ip] = true
					}
				}
			}
		}

	default:
		return nil, fmt.Errorf("unknown discovery method: %s (use 'mdns', 'scan', or 'both')", method)
	}

	return allPlugs, nil
}

// checkInfluxDBHealth verifies InfluxDB connectivity
func checkInfluxDBHealth(ctx context.Context, client influxdb2.Client) error {
	health, err := client.Health(ctx)
	if err != nil {
		return fmt.Errorf("failed to check InfluxDB health: %w", err)
	}

	if health.Status != "pass" {
		return fmt.Errorf("InfluxDB health check failed: status=%s, message=%s", health.Status, *health.Message)
	}

	logger.Info("InfluxDB health check passed")
	return nil
}

func writeToInflux(writeAPI api.WriteAPIBlocking, plugIP string, energy *EnergyUsageResponse) error {
	point := influxdb2.NewPoint(
		"tapo_energy",
		map[string]string{
			"plug_ip": plugIP,
		},
		map[string]interface{}{
			"current_power_watts": float64(energy.Result.CurrentPower) / 1000.0,
			"today_energy_kwh":    float64(energy.Result.TodayEnergy) / 1000.0,
			"month_energy_kwh":    float64(energy.Result.MonthEnergy) / 1000.0,
			"today_runtime_hours": float64(energy.Result.TodayRuntime) / 60.0,
			"month_runtime_hours": float64(energy.Result.MonthRuntime) / 60.0,
		},
		time.Now(),
	)

	return writeAPI.WritePoint(nil, point)
}

func main() {
	// Load configuration
	configFile := "config.json"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	configData, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	var config Config
	if err := json.Unmarshal(configData, &config); err != nil {
		log.Fatalf("Failed to parse config file: %v", err)
	}

	// Validate configuration
	if err := ValidateConfig(&config); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Initialize logger
	logger = NewLogger(config.LogLevel)
	logger.Info("Starting tapo-data-logger")
	logger.Info("Log level: %s", config.LogLevel)

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Received signal %v, initiating graceful shutdown...", sig)
		cancel()
	}()

	// Setup InfluxDB client
	influxClient := influxdb2.NewClient(config.InfluxURL, config.InfluxToken)
	defer influxClient.Close()

	// Check InfluxDB health
	logger.Info("Checking InfluxDB connectivity...")
	if err := checkInfluxDBHealth(ctx, influxClient); err != nil {
		logger.Error("InfluxDB health check failed: %v", err)
		log.Fatalf("InfluxDB health check failed: %v", err)
	}

	writeAPI := influxClient.WriteAPIBlocking(config.InfluxOrg, config.InfluxBucket)

	// Discover plugs if auto-discovery is enabled
	var plugIPs []string
	var plugIPsMu sync.Mutex

	if config.AutoDiscover {
		logger.Info("Starting plug discovery...")

		discoveryMethod := config.DiscoveryMethod
		if discoveryMethod == "" {
			discoveryMethod = "both"
		}

		discovered, err := DiscoverPlugs(ctx, discoveryMethod, config.ScanSubnet, 2*time.Second)
		if err != nil {
			logger.Warn("Discovery failed: %v", err)
		} else {
			logger.Info("Discovered %d plug(s) via %s", len(discovered), discoveryMethod)
			plugIPs = discovered
		}
	}

	// Add manually configured IPs
	for _, ip := range config.PlugIPs {
		// Check if already in discovered list
		found := false
		for _, dip := range plugIPs {
			if dip == ip {
				found = true
				break
			}
		}
		if !found {
			plugIPs = append(plugIPs, ip)
		}
	}

	if len(plugIPs) == 0 {
		logger.Error("No plugs found or configured. Enable auto_discover or add plug_ips to config.")
		log.Fatalf("No plugs found or configured")
	}

	logger.Info("Monitoring %d plug(s): %v", len(plugIPs), plugIPs)
	logger.Info("Polling interval: %d seconds", config.PollInterval)
	logger.Info("Request timeout: %d seconds", config.RequestTimeout)
	logger.Info("Max retries: %d", config.MaxRetries)

	// Rediscover plugs periodically (every hour)
	var wg sync.WaitGroup
	if config.AutoDiscover {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rediscoveryTicker := time.NewTicker(1 * time.Hour)
			defer rediscoveryTicker.Stop()

			for {
				select {
				case <-ctx.Done():
					logger.Info("Stopping plug re-discovery...")
					return
				case <-rediscoveryTicker.C:
					logger.Debug("Re-discovering plugs...")
					discovered, err := DiscoverPlugs(ctx, config.DiscoveryMethod, config.ScanSubnet, 2*time.Second)
					if err != nil {
						logger.Warn("Re-discovery failed: %v", err)
						continue
					}

					// Update plug list
					plugIPsMu.Lock()
					newPlugs := []string{}
					for _, ip := range discovered {
						found := false
						for _, existing := range plugIPs {
							if ip == existing {
								found = true
								break
							}
						}
						if !found {
							logger.Info("Found new plug: %s", ip)
							newPlugs = append(newPlugs, ip)
						}
					}

					if len(newPlugs) > 0 {
						plugIPs = append(plugIPs, newPlugs...)
						logger.Info("Now monitoring %d plug(s)", len(plugIPs))
					}
					plugIPsMu.Unlock()
				}
			}
		}()
	}

	ticker := time.NewTicker(time.Duration(config.PollInterval) * time.Second)
	defer ticker.Stop()

	timeout := time.Duration(config.RequestTimeout) * time.Second

	// Initial collection
	plugIPsMu.Lock()
	for _, plugIP := range plugIPs {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			collectAndLog(ctx, ip, config.TapoEmail, config.TapoPassword, writeAPI, timeout, config.MaxRetries)
		}(plugIP)
	}
	plugIPsMu.Unlock()

	// Periodic collection
	for {
		select {
		case <-ctx.Done():
			logger.Info("Shutdown signal received, stopping periodic collection...")
			ticker.Stop()
			logger.Info("Waiting for all goroutines to finish...")
			wg.Wait()
			logger.Info("Graceful shutdown complete")
			return
		case <-ticker.C:
			plugIPsMu.Lock()
			currentPlugs := make([]string, len(plugIPs))
			copy(currentPlugs, plugIPs)
			plugIPsMu.Unlock()

			for _, plugIP := range currentPlugs {
				wg.Add(1)
				go func(ip string) {
					defer wg.Done()
					collectAndLog(ctx, ip, config.TapoEmail, config.TapoPassword, writeAPI, timeout, config.MaxRetries)
				}(plugIP)
			}
		}
	}
}

func collectAndLog(ctx context.Context, plugIP, email, password string, writeAPI api.WriteAPIBlocking, timeout time.Duration, maxRetries int) {
	client := NewTapoClient(plugIP, email, password, timeout, maxRetries)

	energy, err := client.GetEnergyUsageWithContext(ctx)
	if err != nil {
		logger.Error("[%s] Failed to get energy usage: %v", plugIP, err)
		return
	}

	if err := writeToInflux(writeAPI, plugIP, energy); err != nil {
		logger.Error("[%s] Failed to write to InfluxDB: %v", plugIP, err)
		return
	}

	logger.Debug("[%s] Current power: %.2fW, Today: %.3fkWh",
		plugIP,
		float64(energy.Result.CurrentPower)/1000.0,
		float64(energy.Result.TodayEnergy)/1000.0,
	)
}

