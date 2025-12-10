package format

import (
	"fmt"
	"io"
	"strings"

	"github.com/hjames9/grpcwebcurl/pkg/protocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Printer provides formatted output for gRPC-Web responses.
type Printer struct {
	writer io.Writer
	indent string
	color  bool
}

// NewPrinter creates a new printer.
func NewPrinter(writer io.Writer, color bool) *Printer {
	return &Printer{
		writer: writer,
		indent: "  ",
		color:  color,
	}
}

// PrintResponse prints a formatted response.
func (printer *Printer) PrintResponse(jsonData string, status *protocol.Status, trailers map[string]string) {
	// Print response data
	fmt.Fprintln(printer.writer, jsonData)

	// Print status if there's an error
	if status != nil && status.Code != 0 {
		fmt.Fprintln(printer.writer)
		printer.printError(status)
	}

	// Print trailers if present
	if len(trailers) > 0 {
		fmt.Fprintln(printer.writer)
		printer.printTrailers(trailers)
	}
}

// printError prints a gRPC error status.
func (printer *Printer) printError(status *protocol.Status) {
	statusName := protocol.StatusName(status.Code)
	if printer.color {
		fmt.Fprintf(printer.writer, "\033[31mError: %s (%d)\033[0m\n", statusName, status.Code)
	} else {
		fmt.Fprintf(printer.writer, "Error: %s (%d)\n", statusName, status.Code)
	}
	if status.Message != "" {
		fmt.Fprintf(printer.writer, "Message: %s\n", status.Message)
	}
}

// printTrailers prints response trailers.
func (printer *Printer) printTrailers(trailers map[string]string) {
	if printer.color {
		fmt.Fprintln(printer.writer, "\033[90mTrailers:\033[0m")
	} else {
		fmt.Fprintln(printer.writer, "Trailers:")
	}
	for key, value := range trailers {
		fmt.Fprintf(printer.writer, "%s%s: %s\n", printer.indent, key, value)
	}
}

// PrintServices prints a list of services.
func (printer *Printer) PrintServices(services []string) {
	for _, svc := range services {
		fmt.Fprintln(printer.writer, svc)
	}
}

// PrintServiceDescription prints a detailed service description.
func (printer *Printer) PrintServiceDescription(svc protoreflect.ServiceDescriptor) {
	fmt.Fprintf(printer.writer, "service %s {\n", svc.Name())

	for iter := 0; iter < svc.Methods().Len(); iter++ {
		method := svc.Methods().Get(iter)
		printer.printMethodSignature(method)
	}

	fmt.Fprintln(printer.writer, "}")
}

// printMethodSignature prints a method signature.
func (printer *Printer) printMethodSignature(method protoreflect.MethodDescriptor) {
	var streamPrefix string
	if method.IsStreamingClient() && method.IsStreamingServer() {
		streamPrefix = "stream "
	} else if method.IsStreamingClient() {
		streamPrefix = "client streaming "
	} else if method.IsStreamingServer() {
		streamPrefix = "server streaming "
	}

	inputType := method.Input().FullName()
	outputType := method.Output().FullName()

	fmt.Fprintf(printer.writer, "%srpc %s(%s) returns (%s);\n",
		printer.indent, method.Name(), inputType, outputType)

	if streamPrefix != "" {
		fmt.Fprintf(printer.writer, "%s%s// %s\n", printer.indent, printer.indent, streamPrefix)
	}
}

// PrintMessageDescription prints a detailed message description.
func (printer *Printer) PrintMessageDescription(msg protoreflect.MessageDescriptor) {
	fmt.Fprintf(printer.writer, "message %s {\n", msg.Name())

	fields := msg.Fields()
	for iter := 0; iter < fields.Len(); iter++ {
		field := fields.Get(iter)
		printer.printFieldDescription(field)
	}

	fmt.Fprintln(printer.writer, "}")
}

// printFieldDescription prints a field description.
func (printer *Printer) printFieldDescription(field protoreflect.FieldDescriptor) {
	var repeated string
	if field.IsList() {
		repeated = "repeated "
	}

	typeName := fieldTypeName(field)
	fmt.Fprintf(printer.writer, "%s%s%s %s = %d;\n",
		printer.indent, repeated, typeName, field.Name(), field.Number())
}

// fieldTypeName returns the type name for a field.
func fieldTypeName(field protoreflect.FieldDescriptor) string {
	switch field.Kind() {
	case protoreflect.MessageKind:
		return string(field.Message().FullName())
	case protoreflect.EnumKind:
		return string(field.Enum().FullName())
	default:
		return strings.ToLower(field.Kind().String())
	}
}

// PrintVerbose prints verbose request/response information.
func (printer *Printer) PrintVerbose(direction string, headers map[string]string) {
	prefix := ">"
	if direction == "response" {
		prefix = "<"
	}

	for key, value := range headers {
		fmt.Fprintf(printer.writer, "%s %s: %s\n", prefix, key, value)
	}
	fmt.Fprintln(printer.writer)
}
