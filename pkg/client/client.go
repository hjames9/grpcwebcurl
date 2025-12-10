// Package client provides an HTTP client for gRPC-Web requests.
package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hjames9/grpcwebcurl/pkg/protocol"
)

// Client is a gRPC-Web client.
type Client struct {
	httpClient     *http.Client
	baseURL        string
	headers        map[string]string
	contentType    string
	timeout        time.Duration
	connectTimeout time.Duration
	maxMsgSize     int
	verbose        bool
}

// Options configures the client.
type Options struct {
	// TLS options
	Insecure   bool   // Skip TLS verification
	Plaintext  bool   // Use plaintext HTTP (no TLS)
	CertFile   string // Client certificate file
	KeyFile    string // Client key file
	CAFile     string // CA certificate file
	ServerName string // Override server name for TLS

	// Timeouts
	Timeout        time.Duration // Total request timeout
	ConnectTimeout time.Duration // Connection timeout

	// Message size
	MaxMessageSize int

	// Debugging
	Verbose bool
}

// DefaultOptions returns default client options.
func DefaultOptions() *Options {
	return &Options{
		Timeout:        30 * time.Second,
		ConnectTimeout: 10 * time.Second,
		MaxMessageSize: protocol.MaxMessageSize,
	}
}

// NewClient creates a new gRPC-Web client.
func NewClient(baseURL string, opts *Options) (*Client, error) {
	if opts == nil {
		opts = DefaultOptions()
	}

	// Create HTTP transport
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
	}

	// Configure TLS unless plaintext mode
	if !opts.Plaintext {
		tlsConfig, err := configureTLS(opts)
		if err != nil {
			return nil, fmt.Errorf("failed to configure TLS: %w", err)
		}
		transport.TLSClientConfig = tlsConfig
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   opts.Timeout,
	}

	return &Client{
		httpClient:     httpClient,
		baseURL:        baseURL,
		headers:        make(map[string]string),
		contentType:    protocol.ContentTypeGRPCWeb,
		timeout:        opts.Timeout,
		connectTimeout: opts.ConnectTimeout,
		maxMsgSize:     opts.MaxMessageSize,
		verbose:        opts.Verbose,
	}, nil
}

// configureTLS sets up TLS configuration based on options.
func configureTLS(opts *Options) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: opts.Insecure,
	}

	if opts.ServerName != "" {
		tlsConfig.ServerName = opts.ServerName
	}

	// Load client certificate if provided
	if opts.CertFile != "" && opts.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(opts.CertFile, opts.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Load CA certificate if provided
	if opts.CAFile != "" {
		caCert, err := os.ReadFile(opts.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

// SetHeader sets a custom header for all requests.
func (client *Client) SetHeader(key, value string) {
	client.headers[key] = value
}

// SetHeaders sets multiple custom headers.
func (client *Client) SetHeaders(headers map[string]string) {
	for key, value := range headers {
		client.headers[key] = value
	}
}

// SetContentType sets the content type for requests.
func (client *Client) SetContentType(contentType string) {
	client.contentType = contentType
}

// Request represents a gRPC-Web request.
type Request struct {
	Service string
	Method  string
	Message []byte
	Headers map[string]string
}

// Response represents a gRPC-Web response.
type Response struct {
	Messages    [][]byte
	Trailers    map[string]string
	Status      *protocol.Status
	HTTPStatus  int
	HTTPHeaders http.Header
}

// Invoke makes a unary gRPC-Web call.
func (client *Client) Invoke(ctx context.Context, req *Request) (*Response, error) {
	// Build URL: baseURL/package.Service/Method
	url := fmt.Sprintf("%s/%s/%s", client.baseURL, req.Service, req.Method)

	// Encode message
	body, err := protocol.EncodeMessage(req.Message)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set standard gRPC-Web headers
	protocol.SetRequestHeaders(httpReq, client.contentType)

	// Set custom headers from client
	for key, value := range client.headers {
		// Special handling for Host header - must set req.Host field
		if strings.EqualFold(key, "Host") {
			httpReq.Host = value
		} else {
			httpReq.Header.Set(key, value)
		}
	}

	// Set custom headers from request
	for key, value := range req.Headers {
		// Special handling for Host header - must set req.Host field
		if strings.EqualFold(key, "Host") {
			httpReq.Host = value
		} else {
			httpReq.Header.Set(key, value)
		}
	}

	if client.verbose {
		fmt.Printf("> %s %s\n", httpReq.Method, httpReq.URL)
		for key, values := range httpReq.Header {
			fmt.Printf("> %s: %s\n", key, values)
		}
		fmt.Println()
	}

	// Make request
	httpResp, err := client.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if client.verbose {
		fmt.Printf("< %s\n", httpResp.Status)
		for key, values := range httpResp.Header {
			fmt.Printf("< %s: %s\n", key, values)
		}
		fmt.Println()
	}

	// Read response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if httpResp.StatusCode != http.StatusOK {
		// Try to extract gRPC status from headers
		code, msg := protocol.GetGRPCStatus(httpResp)
		if code != 0 || msg != "" {
			return &Response{
				HTTPStatus:  httpResp.StatusCode,
				HTTPHeaders: httpResp.Header,
				Status: &protocol.Status{
					Code:    code,
					Message: msg,
				},
			}, nil
		}
		return nil, fmt.Errorf("HTTP error: %s", httpResp.Status)
	}

	// Decode gRPC-Web response
	decoded, err := protocol.DecodeResponse(respBody)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &Response{
		Messages:    decoded.Messages,
		Trailers:    decoded.Trailers,
		Status:      decoded.Status,
		HTTPStatus:  httpResp.StatusCode,
		HTTPHeaders: httpResp.Header,
	}, nil
}

// Close closes the client and releases resources.
func (client *Client) Close() error {
	client.httpClient.CloseIdleConnections()
	return nil
}

// StreamHandler is called for each message received in a server streaming call.
type StreamHandler func(message []byte) error

// InvokeServerStream makes a server streaming gRPC-Web call.
// The handler is called for each message received from the server.
func (client *Client) InvokeServerStream(ctx context.Context, req *Request, handler StreamHandler) (*Response, error) {
	// Build URL: baseURL/package.Service/Method
	url := fmt.Sprintf("%s/%s/%s", client.baseURL, req.Service, req.Method)

	// Encode message
	body, err := protocol.EncodeMessage(req.Message)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set standard gRPC-Web headers
	protocol.SetRequestHeaders(httpReq, client.contentType)

	// Set custom headers from client
	for key, value := range client.headers {
		// Special handling for Host header - must set req.Host field
		if strings.EqualFold(key, "Host") {
			httpReq.Host = value
		} else {
			httpReq.Header.Set(key, value)
		}
	}

	// Set custom headers from request
	for key, value := range req.Headers {
		// Special handling for Host header - must set req.Host field
		if strings.EqualFold(key, "Host") {
			httpReq.Host = value
		} else {
			httpReq.Header.Set(key, value)
		}
	}

	if client.verbose {
		fmt.Printf("> %s %s\n", httpReq.Method, httpReq.URL)
		for key, values := range httpReq.Header {
			fmt.Printf("> %s: %s\n", key, values)
		}
		fmt.Println()
	}

	// Make request
	httpResp, err := client.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if client.verbose {
		fmt.Printf("< %s\n", httpResp.Status)
		for key, values := range httpResp.Header {
			fmt.Printf("< %s: %s\n", key, values)
		}
		fmt.Println()
	}

	// Check for HTTP errors
	if httpResp.StatusCode != http.StatusOK {
		// Try to extract gRPC status from headers
		code, msg := protocol.GetGRPCStatus(httpResp)
		if code != 0 || msg != "" {
			return &Response{
				HTTPStatus:  httpResp.StatusCode,
				HTTPHeaders: httpResp.Header,
				Status: &protocol.Status{
					Code:    code,
					Message: msg,
				},
			}, nil
		}
		return nil, fmt.Errorf("HTTP error: %s", httpResp.Status)
	}

	// Read and process streaming response
	decoder := protocol.NewDecoder(httpResp.Body)
	decoder.SetMaxMessageSize(client.maxMsgSize)

	var messages [][]byte
	trailers := make(map[string]string)
	var status *protocol.Status

	for {
		frame, err := decoder.DecodeFrame()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to decode frame: %w", err)
		}

		switch frame.Type {
		case protocol.FrameData:
			// Call handler for each message
			if handler != nil {
				if err := handler(frame.Payload); err != nil {
					return nil, fmt.Errorf("handler error: %w", err)
				}
			}
			messages = append(messages, frame.Payload)

		case protocol.FrameTrailer:
			// Parse trailers
			trailerStr := string(frame.Payload)
			for _, line := range strings.Split(trailerStr, "\r\n") {
				if line == "" {
					continue
				}
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(strings.ToLower(parts[0]))
					value := strings.TrimSpace(parts[1])
					trailers[key] = value
				}
			}

			// Extract status from trailers
			if statusStr, ok := trailers["grpc-status"]; ok {
				code := 0
				fmt.Sscanf(statusStr, "%d", &code)
				status = &protocol.Status{
					Code:    code,
					Message: trailers["grpc-message"],
				}
			}
		}
	}

	// Default to OK status if not set
	if status == nil {
		status = &protocol.Status{Code: 0}
	}

	return &Response{
		Messages:    messages,
		Trailers:    trailers,
		Status:      status,
		HTTPStatus:  httpResp.StatusCode,
		HTTPHeaders: httpResp.Header,
	}, nil
}
