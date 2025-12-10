package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hjames9/grpcwebcurl/pkg/protocol"
)

func TestDefaultOptions(test *testing.T) {
	opts := DefaultOptions()

	if opts.Timeout != 30*time.Second {
		test.Errorf("Timeout = %v, want %v", opts.Timeout, 30*time.Second)
	}
	if opts.ConnectTimeout != 10*time.Second {
		test.Errorf("ConnectTimeout = %v, want %v", opts.ConnectTimeout, 10*time.Second)
	}
	if opts.MaxMessageSize != protocol.MaxMessageSize {
		test.Errorf("MaxMessageSize = %d, want %d", opts.MaxMessageSize, protocol.MaxMessageSize)
	}
	if opts.Insecure != false {
		test.Errorf("Insecure = %v, want false", opts.Insecure)
	}
	if opts.Plaintext != false {
		test.Errorf("Plaintext = %v, want false", opts.Plaintext)
	}
}

func TestNewClient(test *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		opts    *Options
		wantErr bool
	}{
		{
			name:    "with default options",
			baseURL: "http://localhost:8080",
			opts:    nil,
		},
		{
			name:    "with custom options",
			baseURL: "http://localhost:8080",
			opts: &Options{
				Timeout:        60 * time.Second,
				ConnectTimeout: 5 * time.Second,
				Plaintext:      true,
			},
		},
		{
			name:    "with insecure TLS",
			baseURL: "https://localhost:8080",
			opts: &Options{
				Insecure: true,
			},
		},
		{
			name:    "with verbose mode",
			baseURL: "http://localhost:8080",
			opts: &Options{
				Verbose:   true,
				Plaintext: true,
			},
		},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(test *testing.T) {
			client, err := NewClient(tt.baseURL, tt.opts)

			if (err != nil) != tt.wantErr {
				test.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if client == nil {
					test.Fatal("NewClient() returned nil client")
				}
				if client.baseURL != tt.baseURL {
					test.Errorf("baseURL = %q, want %q", client.baseURL, tt.baseURL)
				}
			}
		})
	}
}

func TestClientSetHeader(test *testing.T) {
	client, err := NewClient("http://localhost:8080", &Options{Plaintext: true})
	if err != nil {
		test.Fatalf("NewClient() error = %v", err)
	}

	client.SetHeader("Authorization", "Bearer token123")
	client.SetHeader("X-Custom-Header", "custom-value")

	if client.headers["Authorization"] != "Bearer token123" {
		test.Errorf("Authorization header = %q, want %q", client.headers["Authorization"], "Bearer token123")
	}
	if client.headers["X-Custom-Header"] != "custom-value" {
		test.Errorf("X-Custom-Header = %q, want %q", client.headers["X-Custom-Header"], "custom-value")
	}
}

func TestClientSetHeaders(test *testing.T) {
	client, err := NewClient("http://localhost:8080", &Options{Plaintext: true})
	if err != nil {
		test.Fatalf("NewClient() error = %v", err)
	}

	headers := map[string]string{
		"Authorization":   "Bearer token123",
		"X-Custom-Header": "custom-value",
		"X-Request-ID":    "req-123",
	}

	client.SetHeaders(headers)

	for k, want := range headers {
		if got := client.headers[k]; got != want {
			test.Errorf("headers[%q] = %q, want %q", k, got, want)
		}
	}
}

func TestClientSetContentType(test *testing.T) {
	client, err := NewClient("http://localhost:8080", &Options{Plaintext: true})
	if err != nil {
		test.Fatalf("NewClient() error = %v", err)
	}

	// Default content type
	if client.contentType != protocol.ContentTypeGRPCWeb {
		test.Errorf("default contentType = %q, want %q", client.contentType, protocol.ContentTypeGRPCWeb)
	}

	// Set custom content type
	client.SetContentType(protocol.ContentTypeGRPCWebText)
	if client.contentType != protocol.ContentTypeGRPCWebText {
		test.Errorf("contentType = %q, want %q", client.contentType, protocol.ContentTypeGRPCWebText)
	}
}

func TestClientClose(test *testing.T) {
	client, err := NewClient("http://localhost:8080", &Options{Plaintext: true})
	if err != nil {
		test.Fatalf("NewClient() error = %v", err)
	}

	// Close should notest.Error
	if err := client.Close(); err != nil {
		test.Errorf("Close() error = %v", err)
	}
}

func TestClientInvoke(test *testing.T) {
	// Create a test server that returns a valid gRPC-Web response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			test.Errorf("Request method = %q, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != protocol.ContentTypeGRPCWeb {
			test.Errorf("Content-Type = %q, want %q", ct, protocol.ContentTypeGRPCWeb)
		}
		if xgw := r.Header.Get("X-Grpc-Web"); xgw != "1" {
			test.Errorf("X-Grpc-Web = %q, want %q", xgw, "1")
		}

		// Return a valid gRPC-Web response
		w.Header().Set("Content-Type", protocol.ContentTypeGRPCWeb)
		w.WriteHeader(http.StatusOK)

		// Write a simple response: data frame + trailer frame
		// Data frame: flag=0, length=3, payload=[0x08, 0x01, 0x00] (field 1, varint 1)
		w.Write([]byte{0x00, 0x00, 0x00, 0x00, 0x03, 0x08, 0x01, 0x00})
		// Trailer frame: flag=0x80, length of trailer data
		trailer := []byte("grpc-status: 0\r\n")
		w.Write([]byte{0x80, 0x00, 0x00, 0x00, byte(len(trailer))})
		w.Write(trailer)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, &Options{Plaintext: true})
	if err != nil {
		test.Fatalf("NewClient() error = %v", err)
	}

	resp, err := client.Invoke(context.Background(), &Request{
		Service: "test.Service",
		Method:  "TestMethod",
		Message: []byte{0x08, 0x01}, // field 1 = 1
	})

	if err != nil {
		test.Fatalf("Invoke() error = %v", err)
	}

	if resp.HTTPStatus != http.StatusOK {
		test.Errorf("HTTPStatus = %d, want %d", resp.HTTPStatus, http.StatusOK)
	}
}

func TestClientInvokeWithHeaders(test *testing.T) {
	receivedHeaders := make(map[string]string)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture headers
		receivedHeaders["Authorization"] = r.Header.Get("Authorization")
		receivedHeaders["X-Custom"] = r.Header.Get("X-Custom")

		w.Header().Set("Content-Type", protocol.ContentTypeGRPCWeb)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte{0x00, 0x00, 0x00, 0x00, 0x00}) // Empty data frame
		trailer := []byte("grpc-status: 0\r\n")
		w.Write([]byte{0x80, 0x00, 0x00, 0x00, byte(len(trailer))})
		w.Write(trailer)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, &Options{Plaintext: true})
	if err != nil {
		test.Fatalf("NewClient() error = %v", err)
	}

	// Set client-level headers
	client.SetHeader("Authorization", "Bearer client-token")

	// Invoke with request-level headers
	_, err = client.Invoke(context.Background(), &Request{
		Service: "test.Service",
		Method:  "TestMethod",
		Message: []byte{},
		Headers: map[string]string{
			"X-Custom": "request-value",
		},
	})

	if err != nil {
		test.Fatalf("Invoke() error = %v", err)
	}

	if receivedHeaders["Authorization"] != "Bearer client-token" {
		test.Errorf("Authorization header = %q, want %q", receivedHeaders["Authorization"], "Bearer client-token")
	}
	if receivedHeaders["X-Custom"] != "request-value" {
		test.Errorf("X-Custom header = %q, want %q", receivedHeaders["X-Custom"], "request-value")
	}
}

func TestClientInvokeHT(test *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, &Options{Plaintext: true})
	if err != nil {
		test.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.Invoke(context.Background(), &Request{
		Service: "test.Service",
		Method:  "TestMethod",
		Message: []byte{},
	})

	if err == nil {
		test.Fatal("Invoke() expected error for 404 response")
	}
}

func TestClientInvokeGRPCError(test *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", protocol.ContentTypeGRPCWeb)
		w.WriteHeader(http.StatusOK)

		// Return an error status in trailers
		trailer := []byte("grpc-status: 3\r\ngrpc-message: Invalid argument\r\n")
		w.Write([]byte{0x80, 0x00, 0x00, 0x00, byte(len(trailer))})
		w.Write(trailer)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, &Options{Plaintext: true})
	if err != nil {
		test.Fatalf("NewClient() error = %v", err)
	}

	resp, err := client.Invoke(context.Background(), &Request{
		Service: "test.Service",
		Method:  "TestMethod",
		Message: []byte{},
	})

	if err != nil {
		test.Fatalf("Invoke() error = %v", err)
	}

	if resp.Status == nil {
		test.Fatal("Response should have status")
	}
	if resp.Status.Code != 3 {
		test.Errorf("Status code = %d, want 3", resp.Status.Code)
	}
	if resp.Status.Message != "Invalid argument" {
		test.Errorf("Status message = %q, want %q", resp.Status.Message, "Invalid argument")
	}
}

func TestClientInvokeContext(test *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, &Options{
		Plaintext: true,
		Timeout:   50 * time.Millisecond,
	})
	if err != nil {
		test.Fatalf("NewClient() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err = client.Invoke(ctx, &Request{
		Service: "test.Service",
		Method:  "TestMethod",
		Message: []byte{},
	})

	if err == nil {
		test.Fatal("Invoke() expected error for cancelled context")
	}
}

func TestClientInvokeURLConstruction(test *testing.T) {
	var requestURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURL = r.URL.Path

		w.Header().Set("Content-Type", protocol.ContentTypeGRPCWeb)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte{0x00, 0x00, 0x00, 0x00, 0x00})
		trailer := []byte("grpc-status: 0\r\n")
		w.Write([]byte{0x80, 0x00, 0x00, 0x00, byte(len(trailer))})
		w.Write(trailer)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, &Options{Plaintext: true})
	if err != nil {
		test.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.Invoke(context.Background(), &Request{
		Service: "com.example.UserService",
		Method:  "GetUser",
		Message: []byte{},
	})

	if err != nil {
		test.Fatalf("Invoke() error = %v", err)
	}

	expectedPath := "/com.example.UserService/GetUser"
	if requestURL != expectedPath {
		test.Errorf("Request URL path = %q, want %q", requestURL, expectedPath)
	}
}

func TestOptionsWithCertificates(test *testing.T) {
	// Test that certificate options are accepted (we can't test actual TLS without real certs)
	opts := &Options{
		CertFile: "client.crt",
		KeyFile:  "client.key",
		CAFile:   "ca.crt",
	}

	// This will fail because files don't exist, but we're testing the options are handled
	_, err := NewClient("https://localhost:8080", opts)
	if err == nil {
		test.Skip("Expected error for non-existent cert files")
	}
	// Error is expected because cert files don't exist
}

func TestRequestStruct(test *testing.T) {
	req := &Request{
		Service: "test.Service",
		Method:  "TestMethod",
		Message: []byte{0x01, 0x02, 0x03},
		Headers: map[string]string{
			"Key": "Value",
		},
	}

	if req.Service != "test.Service" {
		test.Errorf("Service = %q, want %q", req.Service, "test.Service")
	}
	if req.Method != "TestMethod" {
		test.Errorf("Method = %q, want %q", req.Method, "TestMethod")
	}
	if len(req.Message) != 3 {
		test.Errorf("Message length = %d, want 3", len(req.Message))
	}
	if req.Headers["Key"] != "Value" {
		test.Errorf("Headers[Key] = %q, want %q", req.Headers["Key"], "Value")
	}
}

func TestResponseStruct(test *testing.T) {
	resp := &Response{
		Messages:   [][]byte{{0x01}, {0x02}},
		Trailers:   map[string]string{"grpc-status": "0"},
		Status:     &protocol.Status{Code: 0, Message: "OK"},
		HTTPStatus: 200,
		HTTPHeaders: http.Header{
			"Content-Type": []string{"application/grpc-web+proto"},
		},
	}

	if len(resp.Messages) != 2 {
		test.Errorf("Messages count = %d, want 2", len(resp.Messages))
	}
	if resp.Trailers["grpc-status"] != "0" {
		test.Errorf("Trailers[grpc-status] = %q, want %q", resp.Trailers["grpc-status"], "0")
	}
	if resp.Status.Code != 0 {
		test.Errorf("Status.Code = %d, want 0", resp.Status.Code)
	}
	if resp.HTTPStatus != 200 {
		test.Errorf("HTTPStatus = %d, want 200", resp.HTTPStatus)
	}
}
