package descriptor

import (
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"
)

func TestParseServiceMethod(test *testing.T) {
	tests := []struct {
		name        string
		fullMethod  string
		wantService string
		wantMethod  string
		wantErr     bool
	}{
		{
			name:        "valid method",
			fullMethod:  "package.Service/Method",
			wantService: "package.Service",
			wantMethod:  "Method",
		},
		{
			name:        "nested package",
			fullMethod:  "com.example.api.UserService/GetUser",
			wantService: "com.example.api.UserService",
			wantMethod:  "GetUser",
		},
		{
			name:        "simple service",
			fullMethod:  "Greeter/SayHello",
			wantService: "Greeter",
			wantMethod:  "SayHello",
		},
		{
			name:       "missing slash",
			fullMethod: "package.Service.Method",
			wantErr:    true,
		},
		{
			name:       "too many slashes",
			fullMethod: "package/Service/Method",
			wantErr:    true,
		},
		{
			name:       "empty string",
			fullMethod: "",
			wantErr:    true,
		},
		{
			name:        "only slash",
			fullMethod:  "/",
			wantService: "",
			wantMethod:  "",
		},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			service, method, err := ParseServiceMethod(tt.fullMethod)

			if (err != nil) != tt.wantErr {
				test.Errorf("ParseServiceMethod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if service != tt.wantService {
					test.Errorf("service = %q, want %q", service, tt.wantService)
				}
				if method != tt.wantMethod {
					test.Errorf("method = %q, want %q", method, tt.wantMethod)
				}
			}
		})
	}
}

func TestResolveImportPaths(test *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir, err := os.MkdirTemp("", "grpcwebcurl-test-*")
	if err != nil {
		test.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	subDir := filepath.Join(tmpDir, "protos")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		test.Fatalf("Failed to create subdir: %v", err)
	}

	testFile := filepath.Join(subDir, "test.proto")
	if err := os.WriteFile(testFile, []byte("syntax = \"proto3\";"), 0644); err != nil {
		test.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name        string
		protoFile   string
		importPaths []string
		wantPath    string
		wantErr     bool
	}{
		{
			name:        "file exists directly",
			protoFile:   testFile,
			importPaths: []string{},
			wantPath:    testFile,
		},
		{
			name:        "file found via import path",
			protoFile:   "test.proto",
			importPaths: []string{subDir},
			wantPath:    filepath.Join(subDir, "test.proto"),
		},
		{
			name:        "file not found",
			protoFile:   "nonexistent.proto",
			importPaths: []string{tmpDir},
			wantErr:     true,
		},
		{
			name:        "empty import paths",
			protoFile:   "nonexistent.proto",
			importPaths: []string{},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			path, err := ResolveImportPaths(tt.protoFile, tt.importPaths)

			if (err != nil) != tt.wantErr {
				test.Errorf("ResolveImportPaths() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && path != tt.wantPath {
				test.Errorf("ResolveImportPaths() = %q, want %q", path, tt.wantPath)
			}
		})
	}
}

func TestNewFileSource(test *testing.T) {
	// Create a minimal file descriptor
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("test.proto"),
		Package: strPtr("test"),
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: strPtr("TestService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       strPtr("TestMethod"),
						InputType:  strPtr(".test.TestRequest"),
						OutputType: strPtr(".test.TestResponse"),
					},
				},
			},
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: strPtr("TestRequest")},
			{Name: strPtr("TestResponse")},
		},
	}

	source, err := NewFileSource(fdp)
	if err != nil {
		test.Fatalf("NewFileSource() error = %v", err)
	}

	if source == nil {
		test.Fatal("NewFileSource() returned nil")
	}
}

func TestFileSourceListServices(test *testing.T) {
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("test.proto"),
		Package: strPtr("test"),
		Service: []*descriptorpb.ServiceDescriptorProto{
			{Name: strPtr("ServiceA")},
			{Name: strPtr("ServiceB")},
		},
	}

	source, err := NewFileSource(fdp)
	if err != nil {
		test.Fatalf("NewFileSource() error = %v", err)
	}

	services, err := source.ListServices()
	if err != nil {
		test.Fatalf("ListServices() error = %v", err)
	}

	if len(services) != 2 {
		test.Errorf("ListServices() returned %d services, want 2", len(services))
	}

	// Check that both services are present
	serviceMap := make(map[string]bool)
	for _, service := range services {
		serviceMap[service] = true
	}

	if !serviceMap["test.ServiceA"] {
		test.Error("ListServices() missing test.ServiceA")
	}
	if !serviceMap["test.ServiceB"] {
		test.Error("ListServices() missing test.ServiceB")
	}
}

func TestFileSourceFindService(test *testing.T) {
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("test.proto"),
		Package: strPtr("test"),
		Service: []*descriptorpb.ServiceDescriptorProto{
			{Name: strPtr("UserService")},
		},
	}

	source, err := NewFileSource(fdp)
	if err != nil {
		test.Fatalf("NewFileSource() error = %v", err)
	}

	tests := []struct {
		name    string
		service string
		wantErr bool
	}{
		{
			name:    "existing service",
			service: "test.UserService",
		},
		{
			name:    "non-existent service",
			service: "test.NonExistent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			svc, err := source.FindService(tt.service)

			if (err != nil) != tt.wantErr {
				test.Errorf("FindService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && svc == nil {
				test.Error("FindService() returned nil")
			}
		})
	}
}

func TestFileSourceFindMethod(test *testing.T) {
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("test.proto"),
		Package: strPtr("test"),
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: strPtr("UserService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       strPtr("GetUser"),
						InputType:  strPtr(".test.GetUserRequest"),
						OutputType: strPtr(".test.GetUserResponse"),
					},
					{
						Name:       strPtr("CreateUser"),
						InputType:  strPtr(".test.CreateUserRequest"),
						OutputType: strPtr(".test.CreateUserResponse"),
					},
				},
			},
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: strPtr("GetUserRequest")},
			{Name: strPtr("GetUserResponse")},
			{Name: strPtr("CreateUserRequest")},
			{Name: strPtr("CreateUserResponse")},
		},
	}

	source, err := NewFileSource(fdp)
	if err != nil {
		test.Fatalf("NewFileSource() error = %v", err)
	}

	tests := []struct {
		name    string
		service string
		method  string
		wantErr bool
	}{
		{
			name:    "existing method",
			service: "test.UserService",
			method:  "GetUser",
		},
		{
			name:    "another existing method",
			service: "test.UserService",
			method:  "CreateUser",
		},
		{
			name:    "non-existent method",
			service: "test.UserService",
			method:  "DeleteUser",
			wantErr: true,
		},
		{
			name:    "non-existent service",
			service: "test.NonExistent",
			method:  "GetUser",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			method, err := source.FindMethod(tt.service, tt.method)

			if (err != nil) != tt.wantErr {
				test.Errorf("FindMethod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if method == nil {
					test.Error("FindMethod() returned nil")
				} else if string(method.Name()) != tt.method {
					test.Errorf("FindMethod() name = %q, want %q", method.Name(), tt.method)
				}
			}
		})
	}
}

func TestFileSourceFindSymbol(test *testing.T) {
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("test.proto"),
		Package: strPtr("test"),
		Service: []*descriptorpb.ServiceDescriptorProto{
			{Name: strPtr("TestService")},
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: strPtr("TestMessage")},
		},
	}

	source, err := NewFileSource(fdp)
	if err != nil {
		test.Fatalf("NewFileSource() error = %v", err)
	}

	tests := []struct {
		name    string
		symbol  string
		wantErr bool
	}{
		{
			name:   "find service",
			symbol: "test.TestService",
		},
		{
			name:   "find message",
			symbol: "test.TestMessage",
		},
		{
			name:    "non-existent symbol",
			symbol:  "test.NonExistent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			desc, err := source.FindSymbol(tt.symbol)

			if (err != nil) != tt.wantErr {
				test.Errorf("FindSymbol() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && desc == nil {
				test.Error("FindSymbol() returned nil")
			}
		})
	}
}

func TestLoadProtoFile(test *testing.T) {
	// Test with non-existent file
	_, err := LoadProtoFile("/nonexistent/path/test.pb")
	if err == nil {
		test.Error("LoadProtoFile() should error for non-existent file")
	}

	// Test with invalid file content
	tmpFile, err := os.CreateTemp("", "invalid-*.pb")
	if err != nil {
		test.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write invalid protobuf data
	tmpFile.WriteString("not valid protobuf")
	tmpFile.Close()

	_, err = LoadProtoFile(tmpFile.Name())
	if err == nil {
		test.Error("LoadProtoFile() should error for invalid protobuf")
	}
}

func TestLoadProtoSet(test *testing.T) {
	// Test with non-existent file
	_, err := LoadProtoSet("/nonexistent/path/test.pb")
	if err == nil {
		test.Error("LoadProtoSet() should error for non-existent file")
	}
}

// Helper function to create string pointers
func strPtr(s string) *string {
	return &s
}
