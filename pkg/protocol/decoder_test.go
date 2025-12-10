package protocol

import (
	"bytes"
	"io"
	"testing"
)

func TestDecoder_DecodeFrame(test *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    *Frame
		wantErr bool
	}{
		{
			name:  "data frame",
			input: []byte{0x00, 0x00, 0x00, 0x00, 0x03, 0x01, 0x02, 0x03},
			want:  &Frame{Type: FrameData, Payload: []byte{0x01, 0x02, 0x03}},
		},
		{
			name:  "empty data frame",
			input: []byte{0x00, 0x00, 0x00, 0x00, 0x00},
			want:  &Frame{Type: FrameData, Payload: []byte{}},
		},
		{
			name:  "trailer frame",
			input: append([]byte{0x80, 0x00, 0x00, 0x00, 0x10}, []byte("grpc-status: 0\r\n")...),
			want:  &Frame{Type: FrameTrailer, Payload: []byte("grpc-status: 0\r\n")},
		},
		{
			name:    "truncated header",
			input:   []byte{0x00, 0x00, 0x00},
			wantErr: true,
		},
		{
			name:    "truncated payload",
			input:   []byte{0x00, 0x00, 0x00, 0x00, 0x05, 0x01, 0x02},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			dec := NewDecoder(bytes.NewReader(tt.input))
			got, err := dec.DecodeFrame()

			if (err != nil) != tt.wantErr {
				test.Fatalf("DecodeFrame() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if got.Type != tt.want.Type {
				test.Errorf("DecodeFrame() type = %v, want %v", got.Type, tt.want.Type)
			}
			if !bytes.Equal(got.Payload, tt.want.Payload) {
				test.Errorf("DecodeFrame() payload = %v, want %v", got.Payload, tt.want.Payload)
			}
		})
	}
}

func TestDecoder_Decode(test *testing.T) {
	input := []byte{0x00, 0x00, 0x00, 0x00, 0x03, 0x01, 0x02, 0x03}
	dec := NewDecoder(bytes.NewReader(input))

	got, err := dec.Decode()
	if err != nil {
		test.Fatalf("Decode() error = %v", err)
	}

	want := []byte{0x01, 0x02, 0x03}
	if !bytes.Equal(got, want) {
		test.Errorf("Decode() = %v, want %v", got, want)
	}
}

func TestDecoder_DecodeAll(test *testing.T) {
	// Two data frames followed by a trailer frame
	input := []byte{
		// Frame 1: data
		0x00, 0x00, 0x00, 0x00, 0x02, 0x01, 0x02,
		// Frame 2: data
		0x00, 0x00, 0x00, 0x00, 0x02, 0x03, 0x04,
		// Frame 3: trailer
		0x80, 0x00, 0x00, 0x00, 0x10,
	}
	input = append(input, []byte("grpc-status: 0\r\n")...)

	dec := NewDecoder(bytes.NewReader(input))
	frames, err := dec.DecodeAll()
	if err != nil && err != io.EOF {
		test.Fatalf("DecodeAll() error = %v", err)
	}

	if len(frames) != 3 {
		test.Fatalf("DecodeAll() got %d frames, want 3", len(frames))
	}

	// Check frame types
	if frames[0].Type != FrameData {
		test.Errorf("frame[0].Type = %v, want %v", frames[0].Type, FrameData)
	}
	if frames[1].Type != FrameData {
		test.Errorf("frame[1].Type = %v, want %v", frames[1].Type, FrameData)
	}
	if frames[2].Type != FrameTrailer {
		test.Errorf("frame[2].Type = %v, want %v", frames[2].Type, FrameTrailer)
	}
}

func TestDecodeResponse(test *testing.T) {
	// Build a response with data and trailers
	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	// Data frame with message
	enc.EncodeFrame(Frame{Type: FrameData, Payload: []byte{0x08, 0x96, 0x01}})

	// Trailer frame
	enc.EncodeFrame(Frame{Type: FrameTrailer, Payload: []byte("grpc-status: 0\r\ngrpc-message: OK\r\n")})

	resp, err := DecodeResponse(buf.Bytes())
	if err != nil {
		test.Fatalf("DecodeResponse() error = %v", err)
	}

	// Check messages
	if len(resp.Messages) != 1 {
		test.Errorf("DecodeResponse() got %d messages, want 1", len(resp.Messages))
	}

	// Check trailers
	if resp.Trailers["grpc-status"] != "0" {
		test.Errorf("DecodeResponse() grpc-status = %q, want %q", resp.Trailers["grpc-status"], "0")
	}

	// Check status
	if resp.Status == nil {
		test.Fatal("DecodeResponse() status is nil")
	}
	if resp.Status.Code != 0 {
		test.Errorf("DecodeResponse() status code = %d, want 0", resp.Status.Code)
	}
}

func TestDecodeResponse_Error(test *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	// Trailer frame with error
	enc.EncodeFrame(Frame{Type: FrameTrailer, Payload: []byte("grpc-status: 3\r\ngrpc-message: Invalid argument\r\n")})

	resp, err := DecodeResponse(buf.Bytes())
	if err != nil {
		test.Fatalf("DecodeResponse() error = %v", err)
	}

	if resp.Status == nil {
		test.Fatal("DecodeResponse() status is nil")
	}
	if resp.Status.Code != 3 {
		test.Errorf("DecodeResponse() status code = %d, want 3", resp.Status.Code)
	}
	if resp.Status.Message != "Invalid argument" {
		test.Errorf("DecodeResponse() status message = %q, want %q", resp.Status.Message, "Invalid argument")
	}
}

func TestDecoder_MaxMessageSize(test *testing.T) {
	// Create a frame with size exceeding limit
	input := []byte{0x00, 0x00, 0x01, 0x00, 0x00} // Claims to be 65536 bytes

	dec := NewDecoder(bytes.NewReader(input))
	dec.SetMaxMessageSize(1000) // Set limit to 1000 bytes

	_, err := dec.DecodeFrame()
	if err == nil {
		test.Fatal("DecodeFrame() expected error for oversized message")
	}
}

func TestDecodeMessage(test *testing.T) {
	input := []byte{0x00, 0x00, 0x00, 0x00, 0x03, 0x01, 0x02, 0x03}

	got, err := DecodeMessage(input)
	if err != nil {
		test.Fatalf("DecodeMessage() error = %v", err)
	}

	want := []byte{0x01, 0x02, 0x03}
	if !bytes.Equal(got, want) {
		test.Errorf("DecodeMessage() = %v, want %v", got, want)
	}
}

func TestRoundTrip(test *testing.T) {
	// Test that encode->decode produces the original message
	original := []byte{0x08, 0x96, 0x01, 0x12, 0x05, 0x68, 0x65, 0x6c, 0x6c, 0x6f}

	encoded, err := EncodeMessage(original)
	if err != nil {
		test.Fatalf("EncodeMessage() error = %v", err)
	}

	decoded, err := DecodeMessage(encoded)
	if err != nil {
		test.Fatalf("DecodeMessage() error = %v", err)
	}

	if !bytes.Equal(decoded, original) {
		test.Errorf("Round trip failed: got %v, want %v", decoded, original)
	}
}
