package descriptor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Parser parses .proto files into descriptors.
type Parser struct {
	importPaths []string
}

// NewParser creates a new proto parser.
func NewParser(importPaths []string) *Parser {
	return &Parser{
		importPaths: importPaths,
	}
}

// ParseFiles parses one or more .proto files and returns a FileSource.
func (parser *Parser) ParseFiles(protoFiles ...string) (*FileSource, error) {
	fds, err := parser.CompileToDescriptorSet(protoFiles...)
	if err != nil {
		return nil, err
	}

	return NewFileSource(fds.File...)
}

// CompileToDescriptorSet compiles proto files to a FileDescriptorSet using protoparse.
func (parser *Parser) CompileToDescriptorSet(protoFiles ...string) (*descriptorpb.FileDescriptorSet, error) {
	// Create protoparse parser
	// The parser will automatically use the filesystem for imports and
	// has built-in support for google/protobuf well-known types
	protoParser := &protoparse.Parser{
		ImportPaths:           parser.importPaths,
		IncludeSourceCodeInfo: true,
	}

	// Parse the proto files
	fileDescriptors, err := protoParser.ParseFiles(protoFiles...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proto files: %w", err)
	}

	// Collect all file descriptors including transitive dependencies
	// The jhump/protoreflect library's FileDescriptor has GetDependencies()
	seen := make(map[string]bool)
	var allDescriptors []*descriptorpb.FileDescriptorProto

	var collectDeps func(fd *desc.FileDescriptor)
	collectDeps = func(fd *desc.FileDescriptor) {
		name := fd.GetName()
		if seen[name] {
			return
		}
		seen[name] = true

		// First collect dependencies (so they appear before files that depend on them)
		for _, dep := range fd.GetDependencies() {
			collectDeps(dep)
		}

		// Then add this file
		allDescriptors = append(allDescriptors, fd.AsFileDescriptorProto())
	}

	// Process all explicitly parsed files (and their dependencies recursively)
	for _, fd := range fileDescriptors {
		collectDeps(fd)
	}

	return &descriptorpb.FileDescriptorSet{File: allDescriptors}, nil
}

// resolveProtoFile resolves a proto file path using import paths.
func (parser *Parser) resolveProtoFile(protoFile string) (string, error) {
	// If it's an absolute path or starts with ./, use as-is
	if filepath.IsAbs(protoFile) || strings.HasPrefix(protoFile, "./") || strings.HasPrefix(protoFile, "../") {
		if _, err := os.Stat(protoFile); err != nil {
			return "", fmt.Errorf("proto file not found: %s", protoFile)
		}
		return protoFile, nil
	}

	// Try current directory first
	if _, err := os.Stat(protoFile); err == nil {
		return protoFile, nil
	}

	// Try each import path
	for _, importPath := range parser.importPaths {
		fullPath := filepath.Join(importPath, protoFile)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, nil
		}
	}

	return "", fmt.Errorf("proto file not found: %s (searched import paths: %v)", protoFile, parser.importPaths)
}

// AddImportPath adds an import path for proto file resolution.
func (parser *Parser) AddImportPath(path string) {
	parser.importPaths = append(parser.importPaths, path)
}

// GetImportPaths returns the current import paths.
func (parser *Parser) GetImportPaths() []string {
	return parser.importPaths
}

// DefaultImportPaths returns common default import paths for proto files.
func DefaultImportPaths() []string {
	paths := []string{"."}

	// Add GOPATH-based paths if available (common location for go-generated protos)
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		protoPath := filepath.Join(gopath, "src")
		if _, err := os.Stat(protoPath); err == nil {
			paths = append(paths, protoPath)
		}
	}

	return paths
}
