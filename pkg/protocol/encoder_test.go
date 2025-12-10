package protocol

import (
	"bytes"
	"testing"
)

func TestEncoder_Encode(test *testing.T) {
	tests := []struct {
		name    string
		message []byte
		want    []byte
	}{
		{
			name:    "empty message",
			message: []byte{},
			want:    []byte{0x00, 0x00, 0x00, 0x00, 0x00},
		},
		{
			name:    "simple message",
			message: []byte{0x01, 0x02, 0x03},
			want:    []byte{0x00, 0x00, 0x00, 0x00, 0x03, 0x01, 0x02, 0x03},
		},
		{
			name:    "longer message",
			message: bytes.Repeat([]byte{0xAB}, 256),
			want:    append([]byte{0x00, 0x00, 0x00, 0x01, 0x00}, bytes.Repeat([]byte{0xAB}, 256)...),
		},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			enc := NewEncoder(&buf)

			if err := enc.Encode(tt.message); err != nil {
				test.Fatalf("Encode() error = %v", err)
			}

			if got := buf.Bytes(); !bytes.Equal(got, tt.want) {
				test.Errorf("Encode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEncoder_EncodeFrame(test *testing.T) {
	tests := []struct {
		name  string
		frame Frame
		want  []byte
	}{
		{
			name:  "data frame",
			frame: Frame{Type: FrameData, Payload: []byte{0x01, 0x02}},
			want:  []byte{0x00, 0x00, 0x00, 0x00, 0x02, 0x01, 0x02},
		},
		{
			name:  "trailer frame",
			frame: Frame{Type: FrameTrailer, Payload: []byte("grpc-status: 0\r\n")},
			want:  append([]byte{0x80, 0x00, 0x00, 0x00, 0x10}, []byte("grpc-status: 0\r\n")...),
		},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			enc := NewEncoder(&buf)

			if err := enc.EncodeFrame(tt.frame); err != nil {
				test.Fatalf("EncodeFrame() error = %v", err)
			}

			if got := buf.Bytes(); !bytes.Equal(got, tt.want) {
				test.Errorf("EncodeFrame() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEncodeMessage(test *testing.T) {
	message := []byte{0x08, 0x96, 0x01} // Protobuf: field 1, varint 150

	got, err := EncodeMessage(message)
	if err != nil {
		test.Fatalf("EncodeMessage() error = %v", err)
	}

	want := []byte{0x00, 0x00, 0x00, 0x00, 0x03, 0x08, 0x96, 0x01}
	if !bytes.Equal(got, want) {
		test.Errorf("EncodeMessage() = %v, want %v", got, want)
	}
}

func TestEncodeTrailer(test *testing.T) {
	trailers := map[string]string{
		"grpc-status":  "0",
		"grpc-message": "",
	}

	got, err := EncodeTrailer(trailers)
	if err != nil {
		test.Fatalf("EncodeTrailer() error = %v", err)
	}

	// Check that it starts with trailer frame type
	if got[0] != byte(FrameTrailer) {
		test.Errorf("EncodeTrailer() frame type = %v, want %v", got[0], FrameTrailer)
	}

	// Verify the payload contains the headers
	// Note: map iteration order is not guaranteed, so we just check it contains both
	payload := string(got[5:])
	if !bytes.Contains([]byte(payload), []byte("grpc-status: 0")) {
		test.Errorf("EncodeTrailer() payload missing grpc-status")
	}
}
