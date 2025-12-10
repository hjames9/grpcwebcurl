package client

import (
	"bytes"
	"testing"
)

func TestEncodeListServicesRequest(test *testing.T) {
	req := encodeListServicesRequest()

	// Should be field 7 (list_services), wire type 2 (length-delimited), length 0
	// 7 << 3 | 2 = 58 = 0x3a
	expected := []byte{0x3a, 0x00}

	if !bytes.Equal(req, expected) {
		test.Errorf("encodeListServicesRequest() = %v, want %v", req, expected)
	}
}

func TestEncodeFileContainingSymbolRequest(test *testing.T) {
	symbol := "test.Service"
	req := encodeFileContainingSymbolRequest(symbol)

	// Should start with field 4 (file_containing_symbol), wire type 2
	// 4 << 3 | 2 = 34 = 0x22
	if req[0] != 0x22 {
		test.Errorf("field tag = %#x, want %#x", req[0], 0x22)
	}

	// Length should be correct
	if int(req[1]) != len(symbol) {
		test.Errorf("length = %d, want %d", req[1], len(symbol))
	}

	// Symbol should be present
	if string(req[2:]) != symbol {
		test.Errorf("symbol = %q, want %q", string(req[2:]), symbol)
	}
}

func TestParseServiceName(test *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name: "simple service name",
			// Field 1 (name), wire type 2, length 12, "test.Service"
			input:    append([]byte{0x0a, 0x0c}, []byte("test.Service")...),
			expected: "test.Service",
		},
		{
			name:     "empty",
			input:    []byte{},
			expected: "",
		},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			result := parseServiceName(tt.input)
			if result != tt.expected {
				test.Errorf("parseServiceName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParseServiceList(test *testing.T) {
	// Build a ListServiceResponse with two services
	// Each service is a ServiceResponse with field 1 = name

	// ServiceResponse 1: name = "service.One"
	service1 := append([]byte{0x0a, 0x0b}, []byte("service.One")...)
	// ServiceResponse 2: name = "service.Two"
	service2 := append([]byte{0x0a, 0x0b}, []byte("service.Two")...)

	// ListServiceResponse: repeated ServiceResponse (field 1)
	input := []byte{}
	// Field 1, wire type 2, length of service1
	input = append(input, 0x0a, byte(len(service1)))
	input = append(input, service1...)
	// Field 1, wire type 2, length of service2
	input = append(input, 0x0a, byte(len(service2)))
	input = append(input, service2...)

	services := parseServiceList(input)

	if len(services) != 2 {
		test.Fatalf("parseServiceList() returned %d services, want 2", len(services))
	}

	if services[0] != "service.One" {
		test.Errorf("services[0] = %q, want %q", services[0], "service.One")
	}
	if services[1] != "service.Two" {
		test.Errorf("services[1] = %q, want %q", services[1], "service.Two")
	}
}
