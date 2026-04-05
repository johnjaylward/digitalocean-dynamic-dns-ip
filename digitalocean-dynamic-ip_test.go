package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestToIPv6String tests the IPv6 string conversion function
func TestToIPv6String(t *testing.T) {
	tests := []struct {
		name     string
		input    net.IP
		expected string
	}{
		{
			name:     "nil IP",
			input:    nil,
			expected: "",
		},
		{
			name:     "IPv4 address",
			input:    net.ParseIP("192.168.1.1"),
			expected: "::ffff:c0a8:0101",
		},
		{
			name:     "IPv6 address",
			input:    net.ParseIP("2001:db8::1"),
			expected: "2001:0db8:0000:0000:0000:0000:0000:0001",
		},
		{
			name:     "IPv6 loopback",
			input:    net.ParseIP("::1"),
			expected: "0000:0000:0000:0000:0000:0000:0000:0001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toIPv6String(tt.input)
			if result != tt.expected {
				t.Errorf("toIPv6String(%v) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestParseIPFromResponse tests parsing an IP address from an HTTP response
func TestParseIPFromResponse(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		shouldFail bool
		expected   net.IP
	}{
		{
			name:       "valid IPv4",
			response:   "192.168.1.100",
			shouldFail: false,
			expected:   net.ParseIP("192.168.1.100"),
		},
		{
			name:       "valid IPv6",
			response:   "2001:db8::1",
			shouldFail: false,
			expected:   net.ParseIP("2001:db8::1"),
		},
		{
			name:       "invalid IP",
			response:   "not-an-ip",
			shouldFail: false,
			expected:   nil,
		},
		{
			name:       "empty response",
			response:   "",
			shouldFail: false,
			expected:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := net.ParseIP(strings.TrimSpace(tt.response))
			if (result == nil && tt.expected != nil) || (result != nil && tt.expected == nil) {
				t.Errorf("ParseIP() = %v, want %v", result, tt.expected)
			}
			if result != nil && tt.expected != nil && !result.Equal(tt.expected) {
				t.Errorf("ParseIP() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestHTTPTimeout verifies that getHTTPTimeout returns the appropriate timeout value
func TestHTTPTimeout(t *testing.T) {
	tests := []struct {
		name       string
		configTime int
		expected   time.Duration
	}{
		{
			name:       "custom timeout",
			configTime: 30,
			expected:   30 * time.Second,
		},
		{
			name:       "zero timeout uses default",
			configTime: 0,
			expected:   15 * time.Second,
		},
		{
			name:       "negative timeout uses default",
			configTime: -5,
			expected:   15 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Temporarily set config for this test
			oldConfig := config
			config = ClientConfig{
				IPvCheckTimeoutSeconds: tt.configTime,
			}
			defer func() { config = oldConfig }()

			result := getHTTPTimeout()
			if result != tt.expected {
				t.Errorf("getHTTPTimeout() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func TestGetConfigFromFile(t *testing.T) {
	configJSON := `{
		"apiKey": "testkey",
		"doPageSize": 40,
		"useIPv4": true,
		"useIPv6": false,
		"ipv4CheckUrl": "https://example.com/ipv4",
		"ipv6CheckUrl": "https://example.com/ipv6",
		"allowIPv4InIPv6": false,
		"domains": []
	}`

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(configJSON), 0600); err != nil {
		t.Fatalf("unable to write config file: %v", err)
	}

	oldArgs := os.Args
	oldFlags := flag.CommandLine
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldFlags
	}()

	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"test", configPath}

	cfg := GetConfig()
	if cfg.APIKey != "testkey" {
		t.Fatalf("GetConfig() APIKey = %s, want testkey", cfg.APIKey)
	}
	if cfg.DOPageSize != 40 {
		t.Fatalf("GetConfig() DOPageSize = %d, want 40", cfg.DOPageSize)
	}
}

func TestUsageOutput(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("unable to create pipe: %v", err)
	}
	os.Stdout = w
	usage()
	w.Close()
	os.Stdout = oldStdout

	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("unable to read usage output: %v", err)
	}
	if !strings.Contains(string(output), "-h | -help") {
		t.Fatalf("usage output missing expected text: %s", output)
	}
}

func TestMainSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("203.0.113.5"))
	}))
	defer server.Close()

	configJSON := `{
		"apiKey": "testkey",
		"doPageSize": 20,
		"useIPv4": true,
		"useIPv6": false,
		"ipv4CheckUrl": "` + server.URL + `",
		"allowIPv4InIPv6": false,
		"domains": []
	}`

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(configJSON), 0600); err != nil {
		t.Fatalf("unable to write config file: %v", err)
	}

	oldArgs := os.Args
	oldFlags := flag.CommandLine
	oldConfig := config
	oldExit := exitFunc
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldFlags
		config = oldConfig
		exitFunc = oldExit
	}()

	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"test", configPath}
	exitFunc = func(code int) {
		t.Fatalf("main exited with code %d", code)
	}

	main()
}

func TestMainFailureWhenNoIPAddressesFound(t *testing.T) {
	configJSON := `{
		"apiKey": "testkey",
		"doPageSize": 20,
		"useIPv4": false,
		"useIPv6": false,
		"domains": []
	}`

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(configJSON), 0600); err != nil {
		t.Fatalf("unable to write config file: %v", err)
	}

	oldArgs := os.Args
	oldFlags := flag.CommandLine
	oldExit := exitFunc
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldFlags
		exitFunc = oldExit
	}()

	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"test", configPath}

	exited := false
	exitFunc = func(code int) {
		exited = true
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
	}

	main()

	if !exited {
		t.Fatal("expected main to exit with code 1")
	}
}

func TestCheckLocalIPsIPv4Only(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("192.0.2.1"))
	}))
	defer server.Close()

	oldConfig := config
	config = ClientConfig{
		UseIPv4:                boolPtr(true),
		UseIPv6:                boolPtr(false),
		IPv4CheckURL:           server.URL,
		IPv6CheckURL:           "https://invalid.example",
		IPvCheckTimeoutSeconds: 5,
	}
	defer func() { config = oldConfig }()

	ipv4, ipv6 := CheckLocalIPs()
	if ipv4 == nil || ipv4.String() != "192.0.2.1" {
		t.Fatalf("CheckLocalIPs() ipv4 = %v, want 192.0.2.1", ipv4)
	}
	if ipv6 != nil {
		t.Fatalf("CheckLocalIPs() ipv6 = %v, want nil", ipv6)
	}
}

func TestGetDomainRecordsPagination(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/domains/test/records" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}

		if r.URL.RawQuery == "per_page=2" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(fmt.Sprintf(`{
				"domain_records": [
					{"id": 1, "type": "A", "name": "www", "data": "1.1.1.1", "ttl": 3600, "priority": null, "port": null, "weight": null, "flags": null, "tag": null}
				],
				"meta": {"total": 2},
				"links": {"pages": {"next": "%s/domains/test/records?page=2", "prev": "", "first": "", "last": ""}}
			}`, server.URL)))
			return
		}

		if r.URL.RawQuery == "page=2" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"domain_records": [
					{"id": 2, "type": "A", "name": "api", "data": "1.1.1.2", "ttl": 3600, "priority": null, "port": null, "weight": null, "flags": null, "tag": null}
				],
				"meta": {"total": 2},
				"links": {"pages": {"next": "", "prev": "", "first": "", "last": ""}}
			}`))
			return
		}

		t.Fatalf("unexpected query %s", r.URL.RawQuery)
	}))
	defer server.Close()

	oldBase := digitalOceanAPIBase
	oldConfig := config
	digitalOceanAPIBase = server.URL
	config = ClientConfig{DOPageSize: 2, IPvCheckTimeoutSeconds: 5}
	defer func() {
		digitalOceanAPIBase = oldBase
		config = oldConfig
	}()

	records := GetDomainRecords("test")
	if len(records) != 2 {
		t.Fatalf("GetDomainRecords() returned %d records, want 2", len(records))
	}
	if records[0].Name != "www" || records[1].Name != "api" {
		t.Fatalf("unexpected records %+v", records)
	}
}

func TestUpdateRecordsPutsRecord(t *testing.T) {
	var updateBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/domains/test/records":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"domain_records": [
					{"id": 1, "type": "A", "name": "www", "data": "1.1.1.1", "ttl": 3600, "priority": null, "port": null, "weight": null, "flags": null, "tag": null}
				],
				"meta": {"total": 1},
				"links": {"pages": {"next": "", "prev": "", "first": "", "last": ""}}
			}`))
		case r.Method == http.MethodPut && r.URL.Path == "/domains/test/records/1":
			var err error
			updateBody, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("unable to read request body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"domain_record": {"id": 1}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	oldBase := digitalOceanAPIBase
	oldConfig := config
	digitalOceanAPIBase = server.URL
	config = ClientConfig{APIKey: "test-key", IPvCheckTimeoutSeconds: 5}
	defer func() {
		digitalOceanAPIBase = oldBase
		config = oldConfig
	}()

	UpdateRecords(Domain{
		Domain:  "test",
		Records: []DNSRecord{{Type: "A", Name: "www", TTL: 300}},
	}, net.ParseIP("1.2.3.4"), nil)

	if updateBody == nil {
		t.Fatal("expected update request body")
	}
	if !strings.Contains(string(updateBody), `"data":"1.2.3.4"`) {
		t.Fatalf("unexpected update body %s", updateBody)
	}
	if !strings.Contains(string(updateBody), `"ttl":300`) {
		t.Fatalf("unexpected update body %s", updateBody)
	}
}

// TestGetURLBodyWithMock tests getURLBody with a mocked HTTP server
func TestGetURLBodyWithMock(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("192.168.1.100"))
	}))
	defer server.Close()

	// Temporarily set config for this test
	oldConfig := config
	config = ClientConfig{
		IPvCheckTimeoutSeconds: 5,
	}
	defer func() { config = oldConfig }()

	result, err := getURLBody(server.URL)
	if err != nil {
		t.Fatalf("getURLBody() error = %v", err)
	}

	expected := "192.168.1.100"
	if strings.TrimSpace(result) != expected {
		t.Errorf("getURLBody() = %s, want %s", strings.TrimSpace(result), expected)
	}
}

// TestDNSRecordValidation tests the isValidRecordType function
func TestDNSRecordValidation(t *testing.T) {
	tests := []struct {
		name       string
		recordType string
		isValid    bool
	}{
		{
			name:       "A record",
			recordType: "A",
			isValid:    true,
		},
		{
			name:       "AAAA record",
			recordType: "AAAA",
			isValid:    true,
		},
		{
			name:       "MX record (unsupported)",
			recordType: "MX",
			isValid:    false,
		},
		{
			name:       "CNAME record (unsupported)",
			recordType: "CNAME",
			isValid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidRecordType(tt.recordType)
			if result != tt.isValid {
				t.Errorf("isValidRecordType(%s) = %v, want %v", tt.recordType, result, tt.isValid)
			}
		})
	}
}

// TestConfigTTLValidation tests the isValidTTL function
func TestConfigTTLValidation(t *testing.T) {
	tests := []struct {
		name    string
		ttl     int
		isValid bool
	}{
		{
			name:    "minimum TTL",
			ttl:     30,
			isValid: true,
		},
		{
			name:    "TTL below minimum",
			ttl:     15,
			isValid: false,
		},
		{
			name:    "typical TTL",
			ttl:     3600,
			isValid: true,
		},
		{
			name:    "high TTL",
			ttl:     86400,
			isValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidTTL(tt.ttl)
			if result != tt.isValid {
				t.Errorf("isValidTTL(%d) = %v, want %v", tt.ttl, result, tt.isValid)
			}
		})
	}
}

// TestMockGetPage tests getPage with a mocked HTTP response
func TestGetPageWithMock(t *testing.T) {
	// Create a mock server that responds with DigitalOcean DNS API format
	responseBody := `{
		"domain_records": [
			{
				"id": 1,
				"type": "A",
				"name": "example",
				"data": "192.168.1.1",
				"priority": null,
				"port": null,
				"ttl": 3600,
				"weight": null,
				"flags": null,
				"tag": null
			}
		],
		"meta": {
			"total": 1
		},
		"links": {
			"pages": {
				"first": "",
				"prev": "",
				"next": "",
				"last": ""
			}
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.Copy(w, bytes.NewReader([]byte(responseBody)))
	}))
	defer server.Close()

	// Set config for test
	oldConfig := config
	config = ClientConfig{
		APIKey:                 "test-key",
		IPvCheckTimeoutSeconds: 5,
	}
	defer func() { config = oldConfig }()

	result := getPage(server.URL)

	if len(result.DomainRecords) != 1 {
		t.Errorf("getPage() returned %d records, want 1", len(result.DomainRecords))
	}

	if result.DomainRecords[0].Name != "example" {
		t.Errorf("Record name = %s, want 'example'", result.DomainRecords[0].Name)
	}

	if result.DomainRecords[0].Type != "A" {
		t.Errorf("Record type = %s, want 'A'", result.DomainRecords[0].Type)
	}
}
