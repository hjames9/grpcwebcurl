// Package descriptor provides proto file parsing and descriptor management.
package descriptor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Source provides access to protobuf descriptors.
type Source interface {
	// FindSymbol looks up a symbol by its fully qualified name.
	FindSymbol(name string) (protoreflect.Descriptor, error)
	// ListServices returns all available service names.
	ListServices() ([]string, error)
	// FindService looks up a service by name.
	FindService(name string) (protoreflect.ServiceDescriptor, error)
	// FindMethod looks up a method by service and method name.
	FindMethod(service, method string) (protoreflect.MethodDescriptor, error)
}

// FileSource provides descriptors from proto files.
type FileSource struct {
	files    *protoregistry.Files
	services map[string]protoreflect.ServiceDescriptor
}

// NewFileSource creates a new source from compiled proto file descriptors.
func NewFileSource(descriptors ...*descriptorpb.FileDescriptorProto) (*FileSource, error) {
	files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{
		File: descriptors,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create file registry: %w", err)
	}

	// Index services
	services := make(map[string]protoreflect.ServiceDescriptor)
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		for iter := 0; iter < fd.Services().Len(); iter++ {
			svc := fd.Services().Get(iter)
			services[string(svc.FullName())] = svc
		}
		return true
	})

	return &FileSource{
		files:    files,
		services: services,
	}, nil
}

// FindSymbol looks up a symbol by its fully qualified name.
func (fileSource *FileSource) FindSymbol(name string) (protoreflect.Descriptor, error) {
	// Try as service
	if svc, ok := fileSource.services[name]; ok {
		return svc, nil
	}

	// Try finding in registry
	desc, err := fileSource.files.FindDescriptorByName(protoreflect.FullName(name))
	if err != nil {
		return nil, fmt.Errorf("symbol not found: %s", name)
	}
	return desc, nil
}

// ListServices returns all available service names.
func (fileSource *FileSource) ListServices() ([]string, error) {
	var services []string
	for name := range fileSource.services {
		services = append(services, name)
	}
	return services, nil
}

// FindService looks up a service by name.
func (fileSource *FileSource) FindService(name string) (protoreflect.ServiceDescriptor, error) {
	svc, ok := fileSource.services[name]
	if !ok {
		return nil, fmt.Errorf("service not found: %s", name)
	}
	return svc, nil
}

// FindMethod looks up a method by service and method name.
func (fileSource *FileSource) FindMethod(service, method string) (protoreflect.MethodDescriptor, error) {
	svc, err := fileSource.FindService(service)
	if err != nil {
		return nil, err
	}

	md := svc.Methods().ByName(protoreflect.Name(method))
	if md == nil {
		return nil, fmt.Errorf("method not found: %s/%s", service, method)
	}
	return md, nil
}

// ParseServiceMethod parses a "package.Service/Method" string into service and method parts.
func ParseServiceMethod(fullMethod string) (service, method string, err error) {
	parts := strings.Split(fullMethod, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid method format: %s (expected package.Service/Method)", fullMethod)
	}
	return parts[0], parts[1], nil
}

// LoadProtoFile loads a proto file descriptor from a binary descriptor file.
func LoadProtoFile(path string) (*descriptorpb.FileDescriptorProto, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read proto file: %w", err)
	}

	var fdp descriptorpb.FileDescriptorProto
	if err := proto.Unmarshal(data, &fdp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal descriptor: %w", err)
	}

	return &fdp, nil
}

// LoadProtoSet loads a FileDescriptorSet from a binary file.
func LoadProtoSet(path string) (*descriptorpb.FileDescriptorSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read descriptor set: %w", err)
	}

	var fds descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(data, &fds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal descriptor set: %w", err)
	}

	return &fds, nil
}

// ResolveImportPaths resolves proto file paths with import paths.
func ResolveImportPaths(protoFile string, importPaths []string) (string, error) {
	// First check if the file exists as-is
	if _, err := os.Stat(protoFile); err == nil {
		return protoFile, nil
	}

	// Try each import path
	for _, importPath := range importPaths {
		fullPath := filepath.Join(importPath, protoFile)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, nil
		}
	}

	return "", fmt.Errorf("proto file not found: %s", protoFile)
}
