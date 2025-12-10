// Package protocol implements gRPC-Web binary format encoding and decoding.
package protocol

import (
	"bytes"
	"encoding/binary"
	"io"
)

// FrameType represents the type of gRPC-Web frame.
type FrameType byte

const (
	// FrameData indicates a data frame (compressed flag = 0 for uncompressed).
	FrameData FrameType = 0x00
	// FrameTrailer indicates a trailer frame (compressed flag = 0x80).
	FrameTrailer FrameType = 0x80
)

// Frame represents a gRPC-Web frame containing either data or trailers.
type Frame struct {
	Type    FrameType
	Payload []byte
}

// Encoder encodes messages into gRPC-Web binary format.
type Encoder struct {
	writer io.Writer
}

// NewEncoder creates a new gRPC-Web encoder that writes to writer.
func NewEncoder(writer io.Writer) *Encoder {
	return &Encoder{writer: writer}
}

// Encode writes a message in gRPC-Web binary format.
// Format: [Compressed-Flag (1 byte)][Message-Length (4 bytes)][Message (N bytes)]
func (encoder *Encoder) Encode(message []byte) error {
	return encoder.EncodeFrame(Frame{Type: FrameData, Payload: message})
}

// EncodeFrame writes a frame in gRPC-Web binary format.
func (encoder *Encoder) EncodeFrame(frame Frame) error {
	// Write frame type (1 byte)
	if _, err := encoder.writer.Write([]byte{byte(frame.Type)}); err != nil {
		return err
	}

	// Write message length (4 bytes, big-endian)
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(frame.Payload)))
	if _, err := encoder.writer.Write(lengthBuf); err != nil {
		return err
	}

	// Write message payload
	if _, err := encoder.writer.Write(frame.Payload); err != nil {
		return err
	}

	return nil
}

// EncodeMessage encodes a single message into gRPC-Web binary format and returns the bytes.
func EncodeMessage(message []byte) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := NewEncoder(&buffer)
	if err := encoder.Encode(message); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// EncodeTrailer encodes trailer metadata into gRPC-Web format.
// Trailers are encoded as HTTP header format within a trailer frame.
func EncodeTrailer(trailers map[string]string) ([]byte, error) {
	var buffer bytes.Buffer

	for key, value := range trailers {
		buffer.WriteString(key)
		buffer.WriteString(": ")
		buffer.WriteString(value)
		buffer.WriteString("\r\n")
	}

	return EncodeFrame(Frame{Type: FrameTrailer, Payload: buffer.Bytes()})
}

// EncodeFrame is a convenience function to encode a single frame.
func EncodeFrame(frame Frame) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := NewEncoder(&buffer)
	if err := encoder.EncodeFrame(frame); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
