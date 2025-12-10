package protocol

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetRequestHeaders(test *testing.T) {
	tests := []struct {
		name        string
		contentType string
		wantHeaders map[string]string
	}{
		{
			name:        "default content type",
			contentType: "",
			wantHeaders: map[string]string{
				HeaderContentType: ContentTypeGRPCWeb,
				HeaderAccept:      ContentTypeGRPCWeb,
				HeaderGRPCWeb:     "1",
				HeaderUserAgent:   DefaultUserAgent,
			},
		},
		{
			name:        "custom content type",
			contentType: ContentTypeGRPCWebText,
			wantHeaders: map[string]string{
				HeaderContentType: ContentTypeGRPCWebText,
				HeaderAccept:      ContentTypeGRPCWebText,
				HeaderGRPCWeb:     "1",
				HeaderUserAgent:   DefaultUserAgent,
			},
		},
		{
			name:        "json content type",
			contentType: ContentTypeGRPCWebJSON,
			wantHeaders: map[string]string{
				HeaderContentType: ContentTypeGRPCWebJSON,
				HeaderAccept:      ContentTypeGRPCWebJSON,
				HeaderGRPCWeb:     "1",
				HeaderUserAgent:   DefaultUserAgent,
			},
		},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://example.com", nil)
			SetRequestHeaders(req, tt.contentType)

			for key, want := range tt.wantHeaders {
				got := req.Header.Get(key)
				if got != want {
					test.Errorf("Header %q = %q, want %q", key, got, want)
				}
			}
		})
	}
}

func TestSetCustomHeaders(test *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://example.com", nil)

	headers := map[string]string{
		"Authorization":   "Bearer token123",
		"X-Custom-Header": "custom-value",
	}

	SetCustomHeaders(req, headers)

	for key, want := range headers {
		got := req.Header.Get(key)
		if got != want {
			test.Errorf("Header %q = %q, want %q", key, got, want)
		}
	}
}

func TestSetTimeout(test *testing.T) {
	tests := []struct {
		name    string
		timeout string
		want    string
	}{
		{
			name:    "empty timeout",
			timeout: "",
			want:    "",
		},
		{
			name:    "seconds timeout",
			timeout: "30S",
			want:    "30S",
		},
		{
			name:    "milliseconds timeout",
			timeout: "5000m",
			want:    "5000m",
		},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://example.com", nil)
			SetTimeout(req, tt.timeout)

			got := req.Header.Get(HeaderGRPCTimeout)
			if got != tt.want {
				test.Errorf("SetTimeout() header = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetGRPCStatus(test *testing.T) {
	tests := []struct {
		name        string
		statusCode  string
		statusMsg   string
		wantCode    int
		wantMessage string
	}{
		{
			name:        "success status",
			statusCode:  "0",
			statusMsg:   "",
			wantCode:    0,
			wantMessage: "",
		},
		{
			name:        "error status",
			statusCode:  "3",
			statusMsg:   "Invalid argument",
			wantCode:    3,
			wantMessage: "Invalid argument",
		},
		{
			name:        "empty status",
			statusCode:  "",
			statusMsg:   "",
			wantCode:    0,
			wantMessage: "",
		},
		{
			name:        "unauthenticated",
			statusCode:  "16",
			statusMsg:   "Missing credentials",
			wantCode:    16,
			wantMessage: "Missing credentials",
		},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Header: make(http.Header),
			}
			if tt.statusCode != "" {
				resp.Header.Set(HeaderGRPCStatus, tt.statusCode)
			}
			if tt.statusMsg != "" {
				resp.Header.Set(HeaderGRPCMessage, tt.statusMsg)
			}

			gotCode, gotMessage := GetGRPCStatus(resp)
			if gotCode != tt.wantCode {
				test.Errorf("GetGRPCStatus() code = %d, want %d", gotCode, tt.wantCode)
			}
			if gotMessage != tt.wantMessage {
				test.Errorf("GetGRPCStatus() message = %q, want %q", gotMessage, tt.wantMessage)
			}
		})
	}
}

func TestStatusName(test *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{StatusOK, "OK"},
		{StatusCancelled, "CANCELLED"},
		{StatusUnknown, "UNKNOWN"},
		{StatusInvalidArgument, "INVALID_ARGUMENT"},
		{StatusDeadlineExceeded, "DEADLINE_EXCEEDED"},
		{StatusNotFound, "NOT_FOUND"},
		{StatusAlreadyExists, "ALREADY_EXISTS"},
		{StatusPermissionDenied, "PERMISSION_DENIED"},
		{StatusResourceExhausted, "RESOURCE_EXHAUSTED"},
		{StatusFailedPrecondition, "FAILED_PRECONDITION"},
		{StatusAborted, "ABORTED"},
		{StatusOutOfRange, "OUT_OF_RANGE"},
		{StatusUnimplemented, "UNIMPLEMENTED"},
		{StatusInternal, "INTERNAL"},
		{StatusUnavailable, "UNAVAILABLE"},
		{StatusDataLoss, "DATA_LOSS"},
		{StatusUnauthenticated, "UNAUTHENTICATED"},
		{99, "UNKNOWN"}, // Unknown code
		{-1, "UNKNOWN"}, // Negative code
	}

	for _, tt := range tests {
		test.Run(tt.want, func(t *testing.T) {
			got := StatusName(tt.code)
			if got != tt.want {
				test.Errorf("StatusName(%d) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestConstants(test *testing.T) {
	// Verify content type constants
	if ContentTypeGRPCWeb != "application/grpc-web+proto" {
		test.Errorf("ContentTypeGRPCWeb = %q, want %q", ContentTypeGRPCWeb, "application/grpc-web+proto")
	}
	if ContentTypeGRPCWebText != "application/grpc-web-text+proto" {
		test.Errorf("ContentTypeGRPCWebText = %q, want %q", ContentTypeGRPCWebText, "application/grpc-web-text+proto")
	}
	if ContentTypeGRPCWebJSON != "application/grpc-web+json" {
		test.Errorf("ContentTypeGRPCWebJSON = %q, want %q", ContentTypeGRPCWebJSON, "application/grpc-web+json")
	}

	// Verify header constants
	if HeaderContentType != "Content-Type" {
		test.Errorf("HeaderContentType = %q, want %q", HeaderContentType, "Content-Type")
	}
	if HeaderGRPCStatus != "Grpc-Status" {
		test.Errorf("HeaderGRPCStatus = %q, want %q", HeaderGRPCStatus, "Grpc-Status")
	}
}
