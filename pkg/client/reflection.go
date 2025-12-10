package client

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/hjames9/grpcwebcurl/pkg/descriptor"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// ReflectionClient provides server reflection capabilities over gRPC-Web.
type ReflectionClient struct {
	client *Client
}

// NewReflectionClient creates a new reflection client.
func NewReflectionClient(client *Client) *ReflectionClient {
	return &ReflectionClient{client: client}
}

// The gRPC reflection service name and methods.
const (
	reflectionServiceName = "grpc.reflection.v1alpha.ServerReflection"
	reflectionMethod      = "ServerReflectionInfo"

	// v1 reflection service (newer)
	reflectionV1ServiceName = "grpc.reflection.v1.ServerReflection"
)

// Request types for reflection.
type serverReflectionRequest struct {
	Host           string `protobuf:"bytes,1,opt,name=host,proto3"`
	ListServices   string `protobuf:"bytes,3,opt,name=list_services,json=listServices,proto3"`
	FileByFilename string `protobuf:"bytes,4,opt,name=file_by_filename,json=fileByFilename,proto3"`
	FileContaining string `protobuf:"bytes,5,opt,name=file_containing_symbol,json=fileContainingSymbol,proto3"`
}

// ListServices returns a list of all services exposed by the server.
func (reflectionClient *ReflectionClient) ListServices(ctx context.Context) ([]string, error) {
	// Build the reflection request for listing services
	req := &descriptorpb.FileDescriptorProto{}
	_ = req // We'll build manually

	// Create reflection request message manually
	// MessageType: list_services = ""
	reqBytes := encodeListServicesRequest()

	// Try v1alpha first, then v1
	resp, err := reflectionClient.client.Invoke(ctx, &Request{
		Service: reflectionServiceName,
		Method:  reflectionMethod,
		Message: reqBytes,
	})
	if err != nil {
		// Try v1
		resp, err = reflectionClient.client.Invoke(ctx, &Request{
			Service: reflectionV1ServiceName,
			Method:  reflectionMethod,
			Message: reqBytes,
		})
		if err != nil {
			return nil, fmt.Errorf("reflection request failed: %w", err)
		}
	}

	// Check for errors
	if resp.Status != nil && resp.Status.Code != 0 {
		return nil, fmt.Errorf("reflection error: %s (%d)", resp.Status.Message, resp.Status.Code)
	}

	if len(resp.Messages) == 0 {
		return nil, fmt.Errorf("no response from reflection service")
	}

	// Parse the response to extract service names
	services, err := parseListServicesResponse(resp.Messages[0])
	if err != nil {
		return nil, err
	}

	// Check for error response in the reflection data (field 7)
	if errMsg := parseReflectionError(resp.Messages[0]); errMsg != "" {
		return nil, fmt.Errorf("reflection error: %s\n\nNote: Server reflection uses bidirectional streaming which has limited support over gRPC-Web.\nConsider using proto files instead: grpcwebcurl -p <proto-file> ...", errMsg)
	}

	// Debug: if no services found but we got a response, dump the raw bytes
	if len(services) == 0 && len(resp.Messages[0]) > 0 {
		fmt.Fprintf(os.Stderr, "DEBUG: Got %d bytes in response but parsed 0 services\n", len(resp.Messages[0]))
		fmt.Fprintf(os.Stderr, "DEBUG: Raw response (hex): %x\n", resp.Messages[0])
	}

	sort.Strings(services)
	return services, nil
}

// FileContainingSymbol returns the file descriptor for a symbol.
func (reflectionClient *ReflectionClient) FileContainingSymbol(ctx context.Context, symbol string) (*descriptorpb.FileDescriptorProto, error) {
	fds, err := reflectionClient.FileContainingSymbolWithDeps(ctx, symbol)
	if err != nil {
		return nil, err
	}
	if len(fds) == 0 {
		return nil, fmt.Errorf("no file descriptors returned")
	}
	return fds[0], nil
}

// FileContainingSymbolWithDeps returns file descriptors for a symbol and all its dependencies.
func (reflectionClient *ReflectionClient) FileContainingSymbolWithDeps(ctx context.Context, symbol string) ([]*descriptorpb.FileDescriptorProto, error) {
	reqBytes := encodeFileContainingSymbolRequest(symbol)

	resp, err := reflectionClient.client.Invoke(ctx, &Request{
		Service: reflectionServiceName,
		Method:  reflectionMethod,
		Message: reqBytes,
	})
	if err != nil {
		// Try v1
		resp, err = reflectionClient.client.Invoke(ctx, &Request{
			Service: reflectionV1ServiceName,
			Method:  reflectionMethod,
			Message: reqBytes,
		})
		if err != nil {
			return nil, fmt.Errorf("reflection request failed: %w", err)
		}
	}

	if resp.Status != nil && resp.Status.Code != 0 {
		return nil, fmt.Errorf("reflection error: %s (%d)", resp.Status.Message, resp.Status.Code)
	}

	if len(resp.Messages) == 0 {
		return nil, fmt.Errorf("no response from reflection service")
	}

	// Parse all file descriptors from the response (includes dependencies)
	return parseAllFileDescriptors(resp.Messages[0])
}

// ResolveService resolves a service name to its descriptor using reflection.
func (reflectionClient *ReflectionClient) ResolveService(ctx context.Context, serviceName string) (protoreflect.ServiceDescriptor, error) {
	// Get file descriptors containing the service and its dependencies
	fds, err := reflectionClient.FileContainingSymbolWithDeps(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	// Build a file registry with all descriptors
	files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{
		File: fds,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create file registry: %w", err)
	}

	// Find the service
	desc, err := files.FindDescriptorByName(protoreflect.FullName(serviceName))
	if err != nil {
		return nil, fmt.Errorf("service not found: %s", serviceName)
	}

	svc, ok := desc.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("%s is not a service", serviceName)
	}

	return svc, nil
}

// ResolveMethod resolves a method to its descriptor using reflection.
func (reflectionClient *ReflectionClient) ResolveMethod(ctx context.Context, service, method string) (protoreflect.MethodDescriptor, error) {
	svc, err := reflectionClient.ResolveService(ctx, service)
	if err != nil {
		return nil, err
	}

	md := svc.Methods().ByName(protoreflect.Name(method))
	if md == nil {
		return nil, fmt.Errorf("method not found: %s/%s", service, method)
	}

	return md, nil
}

// GetSource returns a descriptor source using reflection.
func (reflectionClient *ReflectionClient) GetSource(ctx context.Context) (descriptor.Source, error) {
	services, err := reflectionClient.ListServices(ctx)
	if err != nil {
		return nil, err
	}

	// Collect all file descriptors
	var allFiles []*descriptorpb.FileDescriptorProto
	seen := make(map[string]bool)

	for _, svc := range services {
		// Skip the reflection service itself
		if strings.HasPrefix(svc, "grpc.reflection.") {
			continue
		}

		fdp, err := reflectionClient.FileContainingSymbol(ctx, svc)
		if err != nil {
			continue // Skip services we can't resolve
		}

		if !seen[fdp.GetName()] {
			seen[fdp.GetName()] = true
			allFiles = append(allFiles, fdp)
		}
	}

	return descriptor.NewFileSource(allFiles...)
}

// Encoding helpers for reflection protocol.
// These manually encode the protobuf messages since we don't want to import
// the full grpc reflection proto package.

// encodeListServicesRequest encodes a ServerReflectionRequest with list_services.
func encodeListServicesRequest() []byte {
	// Field 7 (list_services) = "" (empty string means list all)
	// Wire type 2 (length-delimited), field 7 = 0x3a (7 << 3 | 2 = 58 = 0x3a)
	// Length 0
	return []byte{0x3a, 0x00}
}

// encodeFileContainingSymbolRequest encodes a request for a symbol's file descriptor.
func encodeFileContainingSymbolRequest(symbol string) []byte {
	// Field 4 (file_containing_symbol) = symbol
	// Wire type 2 (length-delimited), field 4 = 0x22 (4 << 3 | 2 = 34 = 0x22)
	symbolBytes := []byte(symbol)
	result := make([]byte, 0, 2+len(symbolBytes))
	result = append(result, 0x22)                   // Field 4, wire type 2
	result = append(result, byte(len(symbolBytes))) // Length (assuming < 128)
	result = append(result, symbolBytes...)
	return result
}

// parseListServicesResponse parses the response to extract service names.
func parseListServicesResponse(data []byte) ([]string, error) {
	// The response is a ServerReflectionResponse with list_services_response
	// Field 6: list_services_response (ListServiceResponse)
	//   Field 1: service (repeated ServiceResponse)
	//     Field 1: name (string)

	var services []string
	pos := 0

	for pos < len(data) {
		if pos >= len(data) {
			break
		}

		// Read field tag
		tag := data[pos]
		pos++

		fieldNum := tag >> 3
		wireType := tag & 0x07

		switch wireType {
		case 0: // Varint
			// Skip varint
			for pos < len(data) && data[pos]&0x80 != 0 {
				pos++
			}
			pos++
		case 2: // Length-delimited
			if pos >= len(data) {
				break
			}
			// Read varint length (can be multi-byte)
			length, bytesRead := readVarint(data[pos:])
			pos += bytesRead

			if pos+length > len(data) {
				break
			}

			if fieldNum == 6 { // list_services_response
				// Parse the nested ListServiceResponse
				nested := data[pos : pos+length]
				nestedServices := parseServiceList(nested)
				services = append(services, nestedServices...)
			}
			pos += length
		default:
			// Skip unknown wire types
			pos++
		}
	}

	return services, nil
}

// readVarint reads a varint from the byte slice and returns the value and number of bytes read.
func readVarint(data []byte) (int, int) {
	value := 0
	shift := 0
	bytesRead := 0

	for iter := 0; iter < len(data) && iter < 10; iter++ { // Max 10 bytes for varint
		byteVal := data[iter]
		bytesRead++
		value |= int(byteVal&0x7f) << shift
		if byteVal&0x80 == 0 {
			break
		}
		shift += 7
	}

	return value, bytesRead
}

// parseServiceList parses a ListServiceResponse to extract service names.
func parseServiceList(data []byte) []string {
	var services []string
	pos := 0

	for pos < len(data) {
		if pos >= len(data) {
			break
		}

		tag := data[pos]
		pos++

		fieldNum := tag >> 3
		wireType := tag & 0x07

		if wireType == 2 { // Length-delimited
			if pos >= len(data) {
				break
			}
			length, bytesRead := readVarint(data[pos:])
			pos += bytesRead

			if pos+length > len(data) {
				break
			}

			if fieldNum == 1 { // service (repeated ServiceResponse)
				// Parse ServiceResponse to get name
				serviceData := data[pos : pos+length]
				name := parseServiceName(serviceData)
				if name != "" {
					services = append(services, name)
				}
			}
			pos += length
		} else if wireType == 0 { // Varint
			for pos < len(data) && data[pos]&0x80 != 0 {
				pos++
			}
			pos++
		} else {
			pos++
		}
	}

	return services
}

// parseReflectionError checks for an error response in the reflection data.
// Field 7 is error_response in ServerReflectionResponse.
func parseReflectionError(data []byte) string {
	pos := 0
	for pos < len(data) {
		if pos >= len(data) {
			break
		}

		tag := data[pos]
		pos++

		fieldNum := tag >> 3
		wireType := tag & 0x07

		if wireType == 2 { // Length-delimited
			if pos >= len(data) {
				break
			}
			length := int(data[pos])
			pos++

			if fieldNum == 7 { // error_response
				// Parse ErrorResponse: field 1 = error_code, field 2 = error_message
				return parseErrorResponse(data[pos : pos+length])
			}
			pos += length
		} else if wireType == 0 { // Varint
			for pos < len(data) && data[pos]&0x80 != 0 {
				pos++
			}
			pos++
		} else {
			pos++
		}
	}
	return ""
}

// parseErrorResponse extracts the error message from an ErrorResponse.
func parseErrorResponse(data []byte) string {
	pos := 0
	errorCode := 0
	errorMessage := ""

	for pos < len(data) {
		if pos >= len(data) {
			break
		}

		tag := data[pos]
		pos++

		fieldNum := tag >> 3
		wireType := tag & 0x07

		if wireType == 0 && fieldNum == 1 { // error_code (varint)
			errorCode = int(data[pos])
			pos++
		} else if wireType == 2 && fieldNum == 2 { // error_message (string)
			if pos >= len(data) {
				break
			}
			length := int(data[pos])
			pos++
			if pos+length <= len(data) {
				errorMessage = string(data[pos : pos+length])
			}
			pos += length
		} else if wireType == 2 {
			if pos >= len(data) {
				break
			}
			length := int(data[pos])
			pos++
			pos += length
		} else if wireType == 0 {
			pos++
		} else {
			pos++
		}
	}

	if errorMessage != "" {
		return fmt.Sprintf("%s (code %d)", errorMessage, errorCode)
	}
	if errorCode != 0 {
		return fmt.Sprintf("error code %d", errorCode)
	}
	return ""
}

// parseServiceName parses a ServiceResponse to get the service name.
func parseServiceName(data []byte) string {
	pos := 0
	for pos < len(data) {
		if pos >= len(data) {
			break
		}

		tag := data[pos]
		pos++

		fieldNum := tag >> 3
		wireType := tag & 0x07

		if wireType == 2 && fieldNum == 1 { // name field
			if pos >= len(data) {
				break
			}
			length, bytesRead := readVarint(data[pos:])
			pos += bytesRead
			if pos+length <= len(data) {
				return string(data[pos : pos+length])
			}
		} else if wireType == 2 {
			if pos >= len(data) {
				break
			}
			length, bytesRead := readVarint(data[pos:])
			pos += bytesRead
			pos += length
		} else if wireType == 0 { // Varint
			for pos < len(data) && data[pos]&0x80 != 0 {
				pos++
			}
			pos++
		} else {
			pos++
		}
	}
	return ""
}

// parseFileDescriptorResponse parses a reflection response containing file descriptors.
func parseFileDescriptorResponse(data []byte) (*descriptorpb.FileDescriptorProto, error) {
	fds, err := parseAllFileDescriptors(data)
	if err != nil {
		return nil, err
	}
	if len(fds) == 0 {
		return nil, fmt.Errorf("no file descriptor in response")
	}
	return fds[0], nil
}

// parseAllFileDescriptors parses a reflection response and returns all file descriptors.
func parseAllFileDescriptors(data []byte) ([]*descriptorpb.FileDescriptorProto, error) {
	// Field 4: file_descriptor_response (FileDescriptorResponse)
	//   Field 1: file_descriptor_proto (repeated bytes) - serialized FileDescriptorProto

	pos := 0
	for pos < len(data) {
		if pos >= len(data) {
			break
		}

		tag := data[pos]
		pos++

		fieldNum := tag >> 3
		wireType := tag & 0x07

		if wireType == 2 { // Length-delimited
			if pos >= len(data) {
				break
			}

			// Handle multi-byte length (varint)
			length, bytesRead := readVarint(data[pos:])
			pos += bytesRead

			if pos+length > len(data) {
				break
			}

			if fieldNum == 4 { // file_descriptor_response
				// Parse nested to find all file_descriptor_proto entries
				nested := data[pos : pos+length]
				return parseAllFileDescriptorProtos(nested)
			}
			pos += length
		} else if wireType == 0 { // Varint
			for pos < len(data) && data[pos]&0x80 != 0 {
				pos++
			}
			pos++
		} else {
			pos++
		}
	}

	return nil, fmt.Errorf("no file descriptor in response")
}

// parseAllFileDescriptorProtos parses a FileDescriptorResponse to get all protos.
func parseAllFileDescriptorProtos(data []byte) ([]*descriptorpb.FileDescriptorProto, error) {
	var fds []*descriptorpb.FileDescriptorProto
	pos := 0

	for pos < len(data) {
		if pos >= len(data) {
			break
		}

		tag := data[pos]
		pos++

		fieldNum := tag >> 3
		wireType := tag & 0x07

		if wireType == 2 { // Length-delimited
			if pos >= len(data) {
				break
			}

			// Handle multi-byte length
			length, bytesRead := readVarint(data[pos:])
			pos += bytesRead

			if pos+length > len(data) {
				break
			}

			if fieldNum == 1 { // file_descriptor_proto (repeated)
				fdpBytes := data[pos : pos+length]
				fdp := &descriptorpb.FileDescriptorProto{}
				if err := proto.Unmarshal(fdpBytes, fdp); err != nil {
					return nil, err
				}
				fds = append(fds, fdp)
			}
			pos += length
		} else if wireType == 0 {
			for pos < len(data) && data[pos]&0x80 != 0 {
				pos++
			}
			pos++
		} else {
			pos++
		}
	}

	if len(fds) == 0 {
		return nil, fmt.Errorf("no file descriptor protos found")
	}
	return fds, nil
}

// parseFileDescriptorProto parses a FileDescriptorResponse to get the first proto.
func parseFileDescriptorProto(data []byte) (*descriptorpb.FileDescriptorProto, error) {
	fds, err := parseAllFileDescriptorProtos(data)
	if err != nil {
		return nil, err
	}
	if len(fds) == 0 {
		return nil, fmt.Errorf("no file descriptor proto found")
	}
	return fds[0], nil
}

// ReflectionSource implements descriptor.Source using server reflection.
type ReflectionSource struct {
	client   *ReflectionClient
	ctx      context.Context
	files    *protoregistry.Files
	services map[string]protoreflect.ServiceDescriptor
}

// NewReflectionSource creates a source that uses server reflection.
func NewReflectionSource(ctx context.Context, client *ReflectionClient) (*ReflectionSource, error) {
	return &ReflectionSource{
		client:   client,
		ctx:      ctx,
		services: make(map[string]protoreflect.ServiceDescriptor),
	}, nil
}

// FindSymbol looks up a symbol by name.
func (source *ReflectionSource) FindSymbol(name string) (protoreflect.Descriptor, error) {
	// Try to get the file containing this symbol
	fdp, err := source.client.FileContainingSymbol(source.ctx, name)
	if err != nil {
		return nil, err
	}

	files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fdp},
	})
	if err != nil {
		return nil, err
	}

	return files.FindDescriptorByName(protoreflect.FullName(name))
}

// ListServices returns all service names.
func (source *ReflectionSource) ListServices() ([]string, error) {
	return source.client.ListServices(source.ctx)
}

// FindService finds a service by name.
func (source *ReflectionSource) FindService(name string) (protoreflect.ServiceDescriptor, error) {
	return source.client.ResolveService(source.ctx, name)
}

// FindMethod finds a method by service and method name.
func (source *ReflectionSource) FindMethod(service, method string) (protoreflect.MethodDescriptor, error) {
	return source.client.ResolveMethod(source.ctx, service, method)
}

// Ensure ReflectionSource implements descriptor.Source
var _ descriptor.Source = (*ReflectionSource)(nil)
