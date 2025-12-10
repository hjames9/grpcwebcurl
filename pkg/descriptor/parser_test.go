package descriptor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewParser(test *testing.T) {
	tests := []struct {
		name        string
		importPaths []string
	}{
		{
			name:        "empty import paths",
			importPaths: []string{},
		},
		{
			name:        "single import path",
			importPaths: []string{"."},
		},
		{
			name:        "multiple import paths",
			importPaths: []string{".", "/path/to/protos", "/custom/path"},
		},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			parser := NewParser(tt.importPaths)

			if parser == nil {
				test.Fatal("NewParser() returned nil")
			}

			if len(parser.importPaths) != len(tt.importPaths) {
				test.Errorf("importPaths length = %d, want %d", len(parser.importPaths), len(tt.importPaths))
			}
		})
	}
}

func TestParserAddImportPath(test *testing.T) {
	parser := NewParser([]string{})

	parser.AddImportPath("/path1")
	parser.AddImportPath("/path2")

	if len(parser.importPaths) != 2 {
		test.Errorf("importPaths length = %d, want 2", len(parser.importPaths))
	}

	if parser.importPaths[0] != "/path1" {
		test.Errorf("importPaths[0] = %q, want %q", parser.importPaths[0], "/path1")
	}
	if parser.importPaths[1] != "/path2" {
		test.Errorf("importPaths[1] = %q, want %q", parser.importPaths[1], "/path2")
	}
}

func TestParserGetImportPaths(test *testing.T) {
	importPaths := []string{"/path1", "/path2", "/path3"}
	parser := NewParser(importPaths)

	got := parser.GetImportPaths()

	if len(got) != len(importPaths) {
		test.Errorf("GetImportPaths() length = %d, want %d", len(got), len(importPaths))
	}

	for i, path := range got {
		if path != importPaths[i] {
			test.Errorf("GetImportPaths()[%d] = %q, want %q", i, path, importPaths[i])
		}
	}
}

func TestDefaultImportPaths(test *testing.T) {
	paths := DefaultImportPaths()

	// Should at least include current directory
	found := false
	for _, path := range paths {
		if path == "." {
			found = true
			break
		}
	}

	if !found {
		test.Error("DefaultImportPaths() should include current directory '.'")
	}
}

func TestParserResolveProtoFile(test *testing.T) {
	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "grpcwebcurl-parser-test-*")
	if err != nil {
		test.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test proto file
	testProto := filepath.Join(tmpDir, "test.proto")
	if err := os.WriteFile(testProto, []byte("syntax = \"proto3\";"), 0644); err != nil {
		test.Fatalf("Failed to create test proto: %v", err)
	}

	// Create a subdirectory with another proto
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		test.Fatalf("Failed to create subdir: %v", err)
	}
	subProto := filepath.Join(subDir, "sub.proto")
	if err := os.WriteFile(subProto, []byte("syntax = \"proto3\";"), 0644); err != nil {
		test.Fatalf("Failed to create sub proto: %v", err)
	}

	parser := NewParser([]string{tmpDir})

	tests := []struct {
		name      string
		protoFile string
		wantErr   bool
	}{
		{
			name:      "absolute path exists",
			protoFile: testProto,
		},
		{
			name:      "relative to import path",
			protoFile: "test.proto",
		},
		{
			name:      "in subdirectory",
			protoFile: "sub/sub.proto",
		},
		{
			name:      "non-existent file",
			protoFile: "nonexistent.proto",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			resolved, err := parser.resolveProtoFile(tt.protoFile)

			if (err != nil) != tt.wantErr {
				test.Errorf("resolveProtoFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the resolved path exists
				if _, err := os.Stat(resolved); err != nil {
					test.Errorf("resolved path %q does not exist", resolved)
				}
			}
		})
	}
}

func TestParserParseFilesWithRealProto(test *testing.T) {
	// Create a temp directory with a real proto file
	tmpDir, err := os.MkdirTemp("", "grpcwebcurl-real-proto-*")
	if err != nil {
		test.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a realistic proto file with services
	protoContent := `syntax = "proto3";

package helloworld;

service Greeter {
  rpc SayHello (HelloRequest) returns (HelloReply) {}
  rpc SayHelloStream (HelloRequest) returns (stream HelloReply) {}
}

message HelloRequest {
  string name = 1;
}

message HelloReply {
  string message = 1;
}
`
	protoFile := filepath.Join(tmpDir, "helloworld.proto")
	if err := os.WriteFile(protoFile, []byte(protoContent), 0644); err != nil {
		test.Fatalf("Failed to create proto file: %v", err)
	}

	parser := NewParser([]string{tmpDir})

	source, err := parser.ParseFiles("helloworld.proto")
	if err != nil {
		test.Fatalf("ParseFiles() error = %v", err)
	}

	if source == nil {
		test.Fatal("ParseFiles() returned nil source")
	}

	// Verify services were parsed
	services, err := source.ListServices()
	if err != nil {
		test.Fatalf("ListServices() error = %v", err)
	}

	if len(services) == 0 {
		test.Error("ParseFiles() should have found at least one service")
	}

	// Verify the specific service was found
	found := false
	for _, svc := range services {
		if svc == "helloworld.Greeter" {
			found = true
			break
		}
	}
	if !found {
		test.Errorf("Expected to find helloworld.Greeter service, got: %v", services)
	}
}

func TestParserCompileToDescriptorSetInvalidProto(test *testing.T) {
	// Create a temp directory with an invalid proto file
	tmpDir, err := os.MkdirTemp("", "grpcwebcurl-invalid-proto-*")
	if err != nil {
		test.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invalidProto := filepath.Join(tmpDir, "invalid.proto")
	if err := os.WriteFile(invalidProto, []byte("this is not valid proto syntax"), 0644); err != nil {
		test.Fatalf("Failed to create invalid proto: %v", err)
	}

	parser := NewParser([]string{tmpDir})

	_, err = parser.CompileToDescriptorSet(invalidProto)
	if err == nil {
		test.Error("CompileToDescriptorSet() should error for invalid proto")
	}
}

func TestParserCompileToDescriptorSetNonexistent(test *testing.T) {
	parser := NewParser([]string{"."})

	_, err := parser.CompileToDescriptorSet("/nonexistent/path/test.proto")
	if err == nil {
		test.Error("CompileToDescriptorSet() should error for non-existent file")
	}
}

func TestParserCompileWithValidProto(test *testing.T) {
	// Create a temp directory with a valid proto file
	tmpDir, err := os.MkdirTemp("", "grpcwebcurl-valid-proto-*")
	if err != nil {
		test.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	validProto := filepath.Join(tmpDir, "valid.proto")
	protoContent := `syntax = "proto3";
package test;

message Request {
  string id = 1;
}

message Response {
  string result = 1;
}

service TestService {
  rpc DoSomething(Request) returns (Response);
}
`
	if err := os.WriteFile(validProto, []byte(protoContent), 0644); err != nil {
		test.Fatalf("Failed to create valid proto: %v", err)
	}

	parser := NewParser([]string{tmpDir})

	// Pass just the filename since we're using the import path
	fds, err := parser.CompileToDescriptorSet("valid.proto")
	if err != nil {
		test.Fatalf("CompileToDescriptorSet() error = %v", err)
	}

	if fds == nil {
		test.Fatal("CompileToDescriptorSet() returned nil")
	}

	if len(fds.File) == 0 {
		test.Error("CompileToDescriptorSet() returned empty FileDescriptorSet")
	}

	// Verify the service is present
	found := false
	for _, fd := range fds.File {
		for _, svc := range fd.Service {
			if svc.GetName() == "TestService" {
				found = true
				break
			}
		}
	}

	if !found {
		test.Error("CompileToDescriptorSet() did not include TestService")
	}
}
