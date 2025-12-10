// Package format provides input/output formatting for gRPC-Web messages.
package format

import (
	"bytes"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// JSONFormatter handles JSON encoding/decoding of protobuf messages.
type JSONFormatter struct {
	marshalOpts   protojson.MarshalOptions
	unmarshalOpts protojson.UnmarshalOptions
}

// JSONOptions configures JSON formatting.
type JSONOptions struct {
	// EmitDefaults outputs fields with default values
	EmitDefaults bool
	// Indent specifies the indentation for pretty-printing
	Indent string
	// UseProtoNames uses proto field names instead of camelCase
	UseProtoNames bool
	// UseEnumNumbers outputs enum values as numbers instead of strings
	UseEnumNumbers bool
}

// DefaultJSONOptions returns default JSON formatting options.
func DefaultJSONOptions() *JSONOptions {
	return &JSONOptions{
		EmitDefaults:   false,
		Indent:         "  ",
		UseProtoNames:  false,
		UseEnumNumbers: false,
	}
}

// NewJSONFormatter creates a new JSON formatter.
func NewJSONFormatter(opts *JSONOptions) *JSONFormatter {
	if opts == nil {
		opts = DefaultJSONOptions()
	}

	return &JSONFormatter{
		marshalOpts: protojson.MarshalOptions{
			EmitDefaultValues: opts.EmitDefaults,
			Indent:            opts.Indent,
			UseProtoNames:     opts.UseProtoNames,
			UseEnumNumbers:    opts.UseEnumNumbers,
		},
		unmarshalOpts: protojson.UnmarshalOptions{
			DiscardUnknown: true,
		},
	}
}

// Marshal converts a protobuf message to JSON.
func (formatter *JSONFormatter) Marshal(msg proto.Message) ([]byte, error) {
	return formatter.marshalOpts.Marshal(msg)
}

// MarshalToString converts a protobuf message to a JSON string.
func (formatter *JSONFormatter) MarshalToString(msg proto.Message) (string, error) {
	data, err := formatter.Marshal(msg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Unmarshal parses JSON into a protobuf message.
func (formatter *JSONFormatter) Unmarshal(data []byte, msg proto.Message) error {
	return formatter.unmarshalOpts.Unmarshal(data, msg)
}

// UnmarshalDynamic parses JSON into a dynamic protobuf message.
func (formatter *JSONFormatter) UnmarshalDynamic(data []byte, desc protoreflect.MessageDescriptor) (*dynamicpb.Message, error) {
	msg := dynamicpb.NewMessage(desc)
	if err := formatter.Unmarshal(data, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return msg, nil
}

// MarshalDynamic converts a dynamic protobuf message to JSON.
func (formatter *JSONFormatter) MarshalDynamic(msg *dynamicpb.Message) ([]byte, error) {
	return formatter.Marshal(msg)
}

// ParseRequestJSON parses JSON request data into a protobuf message.
func ParseRequestJSON(data []byte, msgDesc protoreflect.MessageDescriptor) (*dynamicpb.Message, error) {
	formatter := NewJSONFormatter(nil)
	return formatter.UnmarshalDynamic(data, msgDesc)
}

// FormatResponseJSON formats a protobuf message as JSON.
func FormatResponseJSON(msg proto.Message, opts *JSONOptions) (string, error) {
	formatter := NewJSONFormatter(opts)
	return formatter.MarshalToString(msg)
}

// FormatResponseBytes formats raw protobuf bytes as JSON using a message descriptor.
func FormatResponseBytes(data []byte, msgDesc protoreflect.MessageDescriptor, opts *JSONOptions) (string, error) {
	msg := dynamicpb.NewMessage(msgDesc)
	if err := proto.Unmarshal(data, msg); err != nil {
		return "", fmt.Errorf("failed to unmarshal protobuf: %w", err)
	}

	formatter := NewJSONFormatter(opts)
	return formatter.MarshalToString(msg)
}

// PrettyPrintJSON pretty-prints a JSON string.
func PrettyPrintJSON(data []byte) ([]byte, error) {
	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return json.MarshalIndent(value, "", "  ")
}

// CompactJSON compacts a JSON string by removing whitespace.
func CompactJSON(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
