package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
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
}

type TapoClient struct {
	ip       string
	email    string
	password string
	token    string
	cookies  []*http.Cookie
	client   *http.Client
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

func NewTapoClient(ip, email, password string) *TapoClient {
	return &TapoClient{
		ip:       ip,
		email:    email,
		password: password,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *TapoClient) Handshake() error {
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

	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/app", t.ip), bytes.NewBuffer(body))
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

	// Now login with encrypted credentials
	return t.login(serverPubKey, privateKey)
}

func (t *TapoClient) login(serverPubKey *rsa.PublicKey, privateKey *rsa.PrivateKey) error {
	loginPayload := map[string]string{
		"method":   "login_device",
		"params":   fmt.Sprintf(`{"username":"%s","password":"%s"}`, base64.StdEncoding.EncodeToString([]byte(t.email)), base64.StdEncoding.EncodeToString([]byte(t.password))),
		"requestTimeMils": fmt.Sprintf("%d", time.Now().UnixMilli()),
	}

	jsonPayload, _ := json.Marshal(loginPayload)

	// Encrypt with server's public key
	encryptedPayload, err := rsa.EncryptPKCS1v15(rand.Reader, serverPubKey, jsonPayload)
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

	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/app", t.ip), bytes.NewBuffer(body))
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
	decryptedResponse, err := rsa.DecryptPKCS1v15(rand.Reader, privateKey, encryptedResponse)
	if err != nil {
		return fmt.Errorf("failed to decrypt login response: %w", err)
	}

	var loginResult map[string]interface{}
	if err := json.Unmarshal(decryptedResponse, &loginResult); err != nil {
		return fmt.Errorf("failed to unmarshal login result: %w", err)
	}

	if token, ok := loginResult["result"].(map[string]interface{})["token"].(string); ok {
		t.token = token
	} else {
		return fmt.Errorf("failed to extract token from login response")
	}

	t.cookies = resp.Cookies()

	return nil
}

func (t *TapoClient) GetEnergyUsage() (*EnergyUsageResponse, error) {
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
		"method": "get_energy_usage",
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

	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/app?token=%s", t.ip, t.token), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range t.cookies {
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
func DiscoverPlugsMDNS(timeout time.Duration) ([]string, error) {
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
					log.Printf("Discovered Tapo device via mDNS: %s (%s)", entry.Instance, ipStr)
					plugIPs = append(plugIPs, ipStr)
				}
				mu.Unlock()
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Browse for common service types
	serviceTypes := []string{"_hap._tcp", "_http._tcp", "_tapo._tcp"}
	
	for _, serviceType := range serviceTypes {
		if err := resolver.Browse(ctx, serviceType, "local.", entries); err != nil {
			log.Printf("Warning: Failed to browse for %s: %v", serviceType, err)
		}
	}

	<-ctx.Done()
	close(entries)

	return plugIPs, nil
}

// DiscoverPlugsScan discovers Tapo plugs by scanning a subnet
func DiscoverPlugsScan(subnet string, timeout time.Duration) ([]string, error) {
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
		
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			
			if isTapoDevice(ip, timeout) {
				mu.Lock()
				log.Printf("Discovered Tapo device via scan: %s", ip)
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
func DiscoverPlugs(method string, subnet string, timeout time.Duration) ([]string, error) {
	var allPlugs []string
	seen := make(map[string]bool)

	switch method {
	case "mdns":
		plugs, err := DiscoverPlugsMDNS(timeout)
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
		plugs, err := DiscoverPlugsScan(subnet, timeout)
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
		plugs, err := DiscoverPlugsMDNS(timeout)
		if err != nil {
			log.Printf("Warning: mDNS discovery failed: %v", err)
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
			plugs, err := DiscoverPlugsScan(subnet, timeout)
			if err != nil {
				log.Printf("Warning: network scan failed: %v", err)
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

	// Discover plugs if auto-discovery is enabled
	var plugIPs []string
	
	if config.AutoDiscover {
		log.Println("Starting plug discovery...")
		
		discoveryMethod := config.DiscoveryMethod
		if discoveryMethod == "" {
			discoveryMethod = "both"
		}
		
		discovered, err := DiscoverPlugs(discoveryMethod, config.ScanSubnet, 2*time.Second)
		if err != nil {
			log.Printf("Warning: Discovery failed: %v", err)
		} else {
			log.Printf("Discovered %d plug(s) via %s", len(discovered), discoveryMethod)
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
		log.Fatalf("No plugs found or configured. Enable auto_discover or add plug_ips to config.")
	}

	log.Printf("Monitoring %d plug(s): %v", len(plugIPs), plugIPs)

	// Setup InfluxDB client
	influxClient := influxdb2.NewClient(config.InfluxURL, config.InfluxToken)
	defer influxClient.Close()

	writeAPI := influxClient.WriteAPIBlocking(config.InfluxOrg, config.InfluxBucket)

	log.Printf("Polling interval: %d seconds", config.PollInterval)

	// Rediscover plugs periodically (every hour)
	if config.AutoDiscover {
		go func() {
			rediscoveryTicker := time.NewTicker(1 * time.Hour)
			defer rediscoveryTicker.Stop()
			
			for range rediscoveryTicker.C {
				log.Println("Re-discovering plugs...")
				discovered, err := DiscoverPlugs(config.DiscoveryMethod, config.ScanSubnet, 2*time.Second)
				if err != nil {
					log.Printf("Warning: Re-discovery failed: %v", err)
					continue
				}
				
				// Update plug list
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
						log.Printf("Found new plug: %s", ip)
						newPlugs = append(newPlugs, ip)
					}
				}
				
				if len(newPlugs) > 0 {
					plugIPs = append(plugIPs, newPlugs...)
					log.Printf("Now monitoring %d plug(s)", len(plugIPs))
				}
			}
		}()
	}

	ticker := time.NewTicker(time.Duration(config.PollInterval) * time.Second)
	defer ticker.Stop()

	// Initial collection
	for _, plugIP := range plugIPs {
		go collectAndLog(plugIP, config.TapoEmail, config.TapoPassword, writeAPI)
	}

	// Periodic collection
	for range ticker.C {
		for _, plugIP := range plugIPs {
			go collectAndLog(plugIP, config.TapoEmail, config.TapoPassword, writeAPI)
		}
	}
}

func collectAndLog(plugIP, email, password string, writeAPI api.WriteAPIBlocking) {
	client := NewTapoClient(plugIP, email, password)

	if err := client.Handshake(); err != nil {
		log.Printf("[%s] Handshake failed: %v", plugIP, err)
		return
	}

	energy, err := client.GetEnergyUsage()
	if err != nil {
		log.Printf("[%s] Failed to get energy usage: %v", plugIP, err)
		return
	}

	if err := writeToInflux(writeAPI, plugIP, energy); err != nil {
		log.Printf("[%s] Failed to write to InfluxDB: %v", plugIP, err)
		return
	}

	log.Printf("[%s] Current power: %.2fW, Today: %.3fkWh", 
		plugIP, 
		float64(energy.Result.CurrentPower)/1000.0,
		float64(energy.Result.TodayEnergy)/1000.0,
	)
}

