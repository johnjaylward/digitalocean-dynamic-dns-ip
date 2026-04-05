package main

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
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
