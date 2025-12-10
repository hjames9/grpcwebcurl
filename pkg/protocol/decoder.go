package protocol

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

// MaxMessageSize is the maximum allowed message size (16MB).
const MaxMessageSize = 16 * 1024 * 1024

// Decoder decodes gRPC-Web binary format messages.
type Decoder struct {
	reader     io.Reader
	maxMsgSize int
}

// NewDecoder creates a new gRPC-Web decoder that reads from reader.
func NewDecoder(reader io.Reader) *Decoder {
	return &Decoder{
		reader:     reader,
		maxMsgSize: MaxMessageSize,
	}
}

// SetMaxMessageSize sets the maximum allowed message size.
func (decoder *Decoder) SetMaxMessageSize(size int) {
	decoder.maxMsgSize = size
}

// DecodeFrame reads and decodes the next frame from the stream.
// Returns io.EOF when no more frames are available.
func (decoder *Decoder) DecodeFrame() (*Frame, error) {
	// Read frame header (5 bytes: 1 byte type + 4 bytes length)
	header := make([]byte, 5)
	if _, err := io.ReadFull(decoder.reader, header); err != nil {
		return nil, err
	}

	frameType := FrameType(header[0])
	length := binary.BigEndian.Uint32(header[1:5])

	// Validate message size
	if int(length) > decoder.maxMsgSize {
		return nil, fmt.Errorf("message size %d exceeds maximum %d", length, decoder.maxMsgSize)
	}

	// Read payload
	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(decoder.reader, payload); err != nil {
			return nil, fmt.Errorf("failed to read frame payload: %w", err)
		}
	}

	return &Frame{Type: frameType, Payload: payload}, nil
}

// Decode reads and decodes the next data frame, returning the message payload.
// This skips trailer frames and only returns data frames.
func (decoder *Decoder) Decode() ([]byte, error) {
	for {
		frame, err := decoder.DecodeFrame()
		if err != nil {
			return nil, err
		}

		// Return data frames, skip trailer frames
		if frame.Type == FrameData {
			return frame.Payload, nil
		}
	}
}

// DecodeAll reads all frames from the stream and returns them.
func (decoder *Decoder) DecodeAll() ([]*Frame, error) {
	var frames []*Frame
	for {
		frame, err := decoder.DecodeFrame()
		if err == io.EOF {
			break
		}
		if err != nil {
			return frames, err
		}
		frames = append(frames, frame)
	}
	return frames, nil
}

// DecodedResponse contains the parsed response from a gRPC-Web call.
type DecodedResponse struct {
	Messages [][]byte
	Trailers map[string]string
	Status   *Status
}

// Status represents a gRPC status from the response.
type Status struct {
	Code    int
	Message string
}

// DecodeResponse decodes a complete gRPC-Web response, separating data and trailers.
func DecodeResponse(data []byte) (*DecodedResponse, error) {
	decoder := NewDecoder(bytes.NewReader(data))
	frames, err := decoder.DecodeAll()
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to decode frames: %w", err)
	}

	resp := &DecodedResponse{
		Trailers: make(map[string]string),
	}

	for _, frame := range frames {
		switch frame.Type {
		case FrameData:
			resp.Messages = append(resp.Messages, frame.Payload)
		case FrameTrailer:
			// Parse trailers (HTTP header format)
			trailers, status := parseTrailers(frame.Payload)
			for key, value := range trailers {
				resp.Trailers[key] = value
			}
			if status != nil {
				resp.Status = status
			}
		default:
			// Check if it's a trailer frame (high bit set)
			if frame.Type&0x80 != 0 {
				trailers, status := parseTrailers(frame.Payload)
				for key, value := range trailers {
					resp.Trailers[key] = value
				}
				if status != nil {
					resp.Status = status
				}
			}
		}
	}

	return resp, nil
}

// parseTrailers parses trailer data in HTTP header format.
func parseTrailers(data []byte) (map[string]string, *Status) {
	trailers := make(map[string]string)
	var status *Status

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])
		trailers[key] = value

		// Extract gRPC status
		if key == "grpc-status" {
			code := 0
			fmt.Sscanf(value, "%d", &code)
			if status == nil {
				status = &Status{}
			}
			status.Code = code
		} else if key == "grpc-message" {
			if status == nil {
				status = &Status{}
			}
			status.Message = value
		}
	}

	return trailers, status
}

// DecodeMessage decodes a single gRPC-Web message from bytes.
func DecodeMessage(data []byte) ([]byte, error) {
	decoder := NewDecoder(bytes.NewReader(data))
	return decoder.Decode()
}
