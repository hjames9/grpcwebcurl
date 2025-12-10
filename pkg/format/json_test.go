package format

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDefaultJSONOptions(test *testing.T) {
	opts := DefaultJSONOptions()

	if opts.EmitDefaults != false {
		test.Errorf("EmitDefaults = %v, want false", opts.EmitDefaults)
	}
	if opts.Indent != "  " {
		test.Errorf("Indent = %q, want %q", opts.Indent, "  ")
	}
	if opts.UseProtoNames != false {
		test.Errorf("UseProtoNames = %v, want false", opts.UseProtoNames)
	}
	if opts.UseEnumNumbers != false {
		test.Errorf("UseEnumNumbers = %v, want false", opts.UseEnumNumbers)
	}
}

func TestNewJSONFormatter(test *testing.T) {
	tests := []struct {
		name string
		opts *JSONOptions
	}{
		{
			name: "nil options uses defaults",
			opts: nil,
		},
		{
			name: "custom options",
			opts: &JSONOptions{
				EmitDefaults:   true,
				Indent:         "\t",
				UseProtoNames:  true,
				UseEnumNumbers: true,
			},
		},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			formatter := NewJSONFormatter(tt.opts)
			if formatter == nil {
				test.Fatal("NewJSONFormatter returned nil")
			}
		})
	}
}

func TestPrettyPrintJSON(test *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "simple object",
			input: `{"name":"test","value":123}`,
			want:  "{\n  \"name\": \"test\",\n  \"value\": 123\n}",
		},
		{
			name:  "nested object",
			input: `{"outer":{"inner":"value"}}`,
			want:  "{\n  \"outer\": {\n    \"inner\": \"value\"\n  }\n}",
		},
		{
			name:  "array",
			input: `[1,2,3]`,
			want:  "[\n  1,\n  2,\n  3\n]",
		},
		{
			name:    "invalid json",
			input:   `{invalid}`,
			wantErr: true,
		},
		{
			name:  "already pretty",
			input: "{\n  \"key\": \"value\"\n}",
			want:  "{\n  \"key\": \"value\"\n}",
		},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			got, err := PrettyPrintJSON([]byte(tt.input))

			if (err != nil) != tt.wantErr {
				test.Errorf("PrettyPrintJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && string(got) != tt.want {
				test.Errorf("PrettyPrintJSON() = %q, want %q", string(got), tt.want)
			}
		})
	}
}

func TestCompactJSON(test *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "already compact",
			input: `{"name":"test"}`,
			want:  `{"name":"test"}`,
		},
		{
			name:  "with whitespace",
			input: "{\n  \"name\": \"test\"\n}",
			want:  `{"name":"test"}`,
		},
		{
			name:  "with extra spaces",
			input: `{  "key"  :  "value"  }`,
			want:  `{"key":"value"}`,
		},
		{
			name:    "invalid json",
			input:   `{invalid}`,
			wantErr: true,
		},
		{
			name:  "array with whitespace",
			input: "[\n  1,\n  2,\n  3\n]",
			want:  `[1,2,3]`,
		},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			got, err := CompactJSON([]byte(tt.input))

			if (err != nil) != tt.wantErr {
				test.Errorf("CompactJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && string(got) != tt.want {
				test.Errorf("CompactJSON() = %q, want %q", string(got), tt.want)
			}
		})
	}
}

func TestPrettyCompactRoundTrip(test *testing.T) {
	original := `{"users":[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}],"count":2}`

	// Pretty print
	pretty, err := PrettyPrintJSON([]byte(original))
	if err != nil {
		test.Fatalf("PrettyPrintJSON() error = %v", err)
	}

	// Compact again
	compact, err := CompactJSON(pretty)
	if err != nil {
		test.Fatalf("CompactJSON() error = %v", err)
	}

	// Should match original (semantic equality)
	var originalMap, compactMap map[string]interface{}
	if err := json.Unmarshal([]byte(original), &originalMap); err != nil {
		test.Fatalf("Unmarshal original error = %v", err)
	}
	if err := json.Unmarshal(compact, &compactMap); err != nil {
		test.Fatalf("Unmarshal compact error = %v", err)
	}

	// Compare the maps
	originalJSON, _ := json.Marshal(originalMap)
	compactJSON, _ := json.Marshal(compactMap)
	if string(originalJSON) != string(compactJSON) {
		test.Errorf("Round trip mismatch: got %q, want %q", string(compactJSON), string(originalJSON))
	}
}

func TestJSONOptionsApplied(test *testing.T) {
	// Test that options are correctly applied
	opts := &JSONOptions{
		EmitDefaults:   true,
		Indent:         "    ", // 4 spaces
		UseProtoNames:  true,
		UseEnumNumbers: true,
	}

	formatter := NewJSONFormatter(opts)
	if formatter == nil {
		test.Fatal("NewJSONFormatter returned nil")
	}

	// Verify the formatter was created with options
	// We can't directly inspect internal state, but we can verify it doesn't panic
}

func TestPrettyPrintJSONEdgeCases(test *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty object", "{}"},
		{"empty array", "[]"},
		{"null", "null"},
		{"string", `"hello"`},
		{"number", "42"},
		{"boolean true", "true"},
		{"boolean false", "false"},
		{"unicode", `{"emoji":"😀"}`},
		{"escaped chars", `{"path":"C:\\Users\\test"}`},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			result, err := PrettyPrintJSON([]byte(tt.input))
			if err != nil {
				test.Errorf("PrettyPrintJSON(%q) error = %v", tt.input, err)
				return
			}

			// Verify the result is valid JSON
			var v interface{}
			if err := json.Unmarshal(result, &v); err != nil {
				test.Errorf("PrettyPrintJSON result is not valid JSON: %v", err)
			}
		})
	}
}

func TestCompactJSONEdgeCases(test *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty object", "{}"},
		{"empty array", "[]"},
		{"null", "null"},
		{"deeply nested", `{"a":{"b":{"c":{"d":"value"}}}}`},
		{"large array", `[1,2,3,4,5,6,7,8,9,10]`},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			result, err := CompactJSON([]byte(tt.input))
			if err != nil {
				test.Errorf("CompactJSON(%q) error = %v", tt.input, err)
				return
			}

			// Verify the result contains no unnecessary whitespace
			resultStr := string(result)
			if strings.Contains(resultStr, "\n") || strings.Contains(resultStr, "  ") {
				test.Errorf("CompactJSON result contains whitespace: %q", resultStr)
			}
		})
	}
}
