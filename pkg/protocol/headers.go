package protocol

import (
	"fmt"
	"net/http"
)

// Content types for gRPC-Web.
const (
	ContentTypeGRPCWeb     = "application/grpc-web+proto"
	ContentTypeGRPCWebText = "application/grpc-web-text+proto"
	ContentTypeGRPCWebJSON = "application/grpc-web+json"
)

// Header names for gRPC-Web.
const (
	HeaderContentType   = "Content-Type"
	HeaderAccept        = "Accept"
	HeaderGRPCWeb       = "X-Grpc-Web"
	HeaderUserAgent     = "X-User-Agent"
	HeaderGRPCStatus    = "Grpc-Status"
	HeaderGRPCMessage   = "Grpc-Message"
	HeaderGRPCEncoding  = "Grpc-Encoding"
	HeaderGRPCTimeout   = "Grpc-Timeout"
	HeaderAuthorization = "Authorization"
)

// Version is the version of grpcwebcurl.
const Version = "0.1.0"

// DefaultUserAgent is the default user agent string.
var DefaultUserAgent = "grpcwebcurl/" + Version

// SetRequestHeaders sets the required headers for a gRPC-Web request.
func SetRequestHeaders(req *http.Request, contentType string) {
	if contentType == "" {
		contentType = ContentTypeGRPCWeb
	}

	req.Header.Set(HeaderContentType, contentType)
	req.Header.Set(HeaderAccept, contentType)
	req.Header.Set(HeaderGRPCWeb, "1")
	req.Header.Set(HeaderUserAgent, DefaultUserAgent)
}

// SetCustomHeaders adds custom headers to the request.
func SetCustomHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		req.Header.Set(key, value)
	}
}

// SetTimeout sets the gRPC timeout header.
// Timeout format: positive integer with unit (n = nanoseconds, u = microseconds,
// m = milliseconds, S = seconds, M = minutes, H = hours).
func SetTimeout(req *http.Request, timeout string) {
	if timeout != "" {
		req.Header.Set(HeaderGRPCTimeout, timeout)
	}
}

// GetGRPCStatus extracts the gRPC status code from response headers.
func GetGRPCStatus(resp *http.Response) (int, string) {
	status := resp.Header.Get(HeaderGRPCStatus)
	message := resp.Header.Get(HeaderGRPCMessage)

	code := 0
	if status != "" {
		_, _ = fmt.Sscanf(status, "%d", &code)
	}

	return code, message
}

// gRPC status codes.
const (
	StatusOK                 = 0
	StatusCancelled          = 1
	StatusUnknown            = 2
	StatusInvalidArgument    = 3
	StatusDeadlineExceeded   = 4
	StatusNotFound           = 5
	StatusAlreadyExists      = 6
	StatusPermissionDenied   = 7
	StatusResourceExhausted  = 8
	StatusFailedPrecondition = 9
	StatusAborted            = 10
	StatusOutOfRange         = 11
	StatusUnimplemented      = 12
	StatusInternal           = 13
	StatusUnavailable        = 14
	StatusDataLoss           = 15
	StatusUnauthenticated    = 16
)

// StatusName returns the name of a gRPC status code.
func StatusName(code int) string {
	names := map[int]string{
		StatusOK:                 "OK",
		StatusCancelled:          "CANCELLED",
		StatusUnknown:            "UNKNOWN",
		StatusInvalidArgument:    "INVALID_ARGUMENT",
		StatusDeadlineExceeded:   "DEADLINE_EXCEEDED",
		StatusNotFound:           "NOT_FOUND",
		StatusAlreadyExists:      "ALREADY_EXISTS",
		StatusPermissionDenied:   "PERMISSION_DENIED",
		StatusResourceExhausted:  "RESOURCE_EXHAUSTED",
		StatusFailedPrecondition: "FAILED_PRECONDITION",
		StatusAborted:            "ABORTED",
		StatusOutOfRange:         "OUT_OF_RANGE",
		StatusUnimplemented:      "UNIMPLEMENTED",
		StatusInternal:           "INTERNAL",
		StatusUnavailable:        "UNAVAILABLE",
		StatusDataLoss:           "DATA_LOSS",
		StatusUnauthenticated:    "UNAUTHENTICATED",
	}
	if name, ok := names[code]; ok {
		return name
	}
	return "UNKNOWN"
}
