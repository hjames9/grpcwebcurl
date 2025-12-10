// grpcwebcurl is a command-line tool for testing gRPC-Web endpoints.
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/hjames9/grpcwebcurl/pkg/client"
	"github.com/hjames9/grpcwebcurl/pkg/descriptor"
	"github.com/hjames9/grpcwebcurl/pkg/format"
	"github.com/hjames9/grpcwebcurl/pkg/protocol"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

var (
	version = "dev" // Set by ldflags during build

	// Flags
	protoFiles     []string
	importPaths    []string
	data           string
	headers        []string
	insecure       bool
	plaintext      bool
	certFile       string
	keyFile        string
	caFile         string
	resolve        string
	connectTimeout time.Duration
	timeout        time.Duration
	maxMsgSize     int
	emitDefaults   bool
	verbose        bool
	useReflection  bool
	outputFormat   string
	showTrailers   bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "grpcwebcurl [flags] <address> <method>",
		Short: "A CLI tool for testing gRPC-Web endpoints",
		Long: `grpcwebcurl is a command-line tool similar to grpcurl but for gRPC-Web protocol.

It allows you to invoke gRPC-Web methods on a server, providing JSON input
and receiving JSON output.

Examples:
  # Simple unary call with proto file
  grpcwebcurl -proto api.proto -d '{"id": "123"}' \
    https://api.example.com:443 package.Service/Method

  # Using server reflection (no proto file needed)
  grpcwebcurl -d '{"id": "123"}' \
    https://api.example.com:443 package.Service/Method

  # With custom headers
  grpcwebcurl -proto api.proto -H "Authorization: Bearer token" \
    -d '{"id": "123"}' https://api.example.com:443 package.Service/Method

  # Skip TLS verification (development)
  grpcwebcurl -insecure -d '{"id": "123"}' \
    https://localhost:8080 package.Service/Method

  # Use plaintext HTTP (no TLS)
  grpcwebcurl -plaintext -d '{"id": "123"}' \
    http://localhost:8080 package.Service/Method

  # Read request data from stdin
  echo '{"id": "123"}' | grpcwebcurl -proto api.proto -d @ \
    https://api.example.com:443 package.Service/Method`,
		Version:      version,
		Args:         cobra.ExactArgs(2),
		RunE:         runInvoke,
		SilenceUsage: true,
	}

	// Proto file flags (persistent so they're available to subcommands)
	rootCmd.PersistentFlags().StringArrayVarP(&protoFiles, "proto", "p", nil, "Proto file(s) to use for message types")
	rootCmd.PersistentFlags().StringArrayVarP(&importPaths, "import-path", "I", nil, "Import path for proto files")

	// Request flags
	rootCmd.Flags().StringVarP(&data, "data", "d", "", "Request data in JSON format (use @ to read from stdin)")
	rootCmd.PersistentFlags().StringArrayVarP(&headers, "header", "H", nil, "Custom headers in 'Key: Value' format")

	// TLS flags (persistent for subcommands)
	rootCmd.PersistentFlags().BoolVarP(&insecure, "insecure", "k", false, "Skip TLS certificate verification")
	rootCmd.PersistentFlags().BoolVar(&plaintext, "plaintext", false, "Use plaintext HTTP (no TLS)")
	rootCmd.PersistentFlags().StringVar(&certFile, "cert", "", "Client certificate file")
	rootCmd.PersistentFlags().StringVar(&keyFile, "key", "", "Client private key file")
	rootCmd.PersistentFlags().StringVar(&caFile, "cacert", "", "CA certificate file")
	rootCmd.PersistentFlags().StringVar(&resolve, "resolve", "", "Resolve host:port to address (e.g., example.com:443:127.0.0.1)")

	// Timeout flags (persistent for subcommands)
	rootCmd.PersistentFlags().DurationVar(&connectTimeout, "connect-timeout", 10*time.Second, "Connection timeout")
	rootCmd.PersistentFlags().DurationVar(&timeout, "max-time", 30*time.Second, "Maximum time for the request")

	// Output flags
	rootCmd.Flags().IntVar(&maxMsgSize, "max-msg-sz", protocol.MaxMessageSize, "Maximum message size")
	rootCmd.Flags().BoolVar(&emitDefaults, "emit-defaults", false, "Emit fields with default values")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.Flags().StringVarP(&outputFormat, "format", "o", "json", "Output format: json or text")
	rootCmd.Flags().BoolVar(&showTrailers, "show-trailers", false, "Always show response trailers")

	// Add subcommands
	rootCmd.AddCommand(listCmd())
	rootCmd.AddCommand(describeCmd())
	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(completionCmd())

	// Custom error handling: show usage for argument errors only
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		// Check if this is an argument validation error (from cobra.ExactArgs)
		errStr := err.Error()
		if strings.Contains(errStr, "accepts") && strings.Contains(errStr, "arg") {
			// This is an argument count error - show usage
			fmt.Fprintln(os.Stderr, "Error:", err)
			fmt.Fprintln(os.Stderr)
			rootCmd.Usage()
		} else {
			// This is a runtime error - just show the error
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(1)
	}
}

// getDescriptorSource returns a descriptor source, using either proto files or reflection.
func getDescriptorSource(ctx context.Context, address string, c *client.Client) (descriptor.Source, error) {
	if len(protoFiles) > 0 {
		// Use proto files
		parser := descriptor.NewParser(append([]string{"."}, importPaths...))
		return parser.ParseFiles(protoFiles...)
	}

	// Use server reflection
	if verbose {
		fmt.Fprintln(os.Stderr, "Using server reflection to discover services...")
	}

	reflClient := client.NewReflectionClient(c)
	return client.NewReflectionSource(ctx, reflClient)
}

// createClient creates a gRPC-Web client with the current options.
func createClient(address string) (*client.Client, error) {
	clientOpts := &client.Options{
		Insecure:       insecure,
		Plaintext:      plaintext,
		CertFile:       certFile,
		KeyFile:        keyFile,
		CAFile:         caFile,
		Resolve:        resolve,
		Timeout:        timeout,
		ConnectTimeout: connectTimeout,
		MaxMessageSize: maxMsgSize,
		Verbose:        verbose,
	}

	return client.NewClient(address, clientOpts)
}

// readRequestData reads request data from the -d flag or stdin.
func readRequestData() (string, error) {
	if data == "@" {
		// Read from stdin
		reader := bufio.NewReader(os.Stdin)
		var builder strings.Builder

		for {
			line, err := reader.ReadString('\n')
			builder.WriteString(line)
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", fmt.Errorf("failed to read from stdin: %w", err)
			}
		}

		return strings.TrimSpace(builder.String()), nil
	}

	return data, nil
}

func runInvoke(cmd *cobra.Command, args []string) error {
	address := args[0]
	fullMethod := args[1]

	// Validate output format
	if outputFormat != "json" && outputFormat != "text" {
		return fmt.Errorf("invalid output format %q: must be 'json' or 'text'", outputFormat)
	}

	// Parse service and method
	service, method, err := descriptor.ParseServiceMethod(fullMethod)
	if err != nil {
		return suggestMethodFormat(fullMethod, err)
	}

	// Read request data
	requestData, err := readRequestData()
	if err != nil {
		return err
	}

	if requestData == "" {
		return fmt.Errorf("request data is required (-d flag)\n\nExample:\n  grpcwebcurl -d '{\"id\": \"123\"}' %s %s", address, fullMethod)
	}

	// Create client
	c, err := createClient(address)
	if err != nil {
		return suggestClientError(address, err)
	}
	defer c.Close()

	// Set custom headers
	for _, header := range headers {
		parts := strings.SplitN(header, ":", 2)
		if len(parts) == 2 {
			c.SetHeader(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}

	// Create context
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Get descriptor source (proto files or reflection)
	source, err := getDescriptorSource(ctx, address, c)
	if err != nil {
		return suggestDescriptorError(err)
	}

	// Find the method descriptor
	methodDesc, err := source.FindMethod(service, method)
	if err != nil {
		return suggestMethodNotFound(service, method, source, err)
	}

	// Parse request JSON
	formatter := format.NewJSONFormatter(nil)
	reqMsg, err := formatter.UnmarshalDynamic([]byte(requestData), methodDesc.Input())
	if err != nil {
		return fmt.Errorf("failed to parse request JSON: %w\n\nExpected message type: %s", err, methodDesc.Input().FullName())
	}

	// Serialize request message
	reqBytes, err := proto.Marshal(reqMsg)
	if err != nil {
		return fmt.Errorf("failed to serialize request: %w", err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Calling %s/%s\n", service, method)
		fmt.Fprintf(os.Stderr, "Request: %s\n", requestData)
		if methodDesc.IsStreamingServer() {
			fmt.Fprintln(os.Stderr, "Method type: server streaming")
		} else {
			fmt.Fprintln(os.Stderr, "Method type: unary")
		}
		fmt.Fprintln(os.Stderr)
	}

	// JSON formatting options
	jsonOpts := &format.JSONOptions{
		EmitDefaults: emitDefaults,
		Indent:       "  ",
	}

	var resp *client.Response

	// Check if this is a server streaming method
	if methodDesc.IsStreamingServer() {
		// Handle server streaming
		msgCount := 0
		resp, err = c.InvokeServerStream(ctx, &client.Request{
			Service: service,
			Method:  method,
			Message: reqBytes,
		}, func(msgBytes []byte) error {
			msgCount++
			return printResponseMessage(msgBytes, methodDesc.Output(), jsonOpts, msgCount)
		})
	} else {
		// Handle unary call
		resp, err = c.Invoke(ctx, &client.Request{
			Service: service,
			Method:  method,
			Message: reqBytes,
		})
	}

	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	// Check for gRPC errors
	if resp.Status != nil && resp.Status.Code != 0 {
		printGRPCError(resp.Status)
		os.Exit(1)
	}

	// For unary calls, print the response (streaming already printed via handler)
	if !methodDesc.IsStreamingServer() {
		for _, msgBytes := range resp.Messages {
			if err := printResponseMessage(msgBytes, methodDesc.Output(), jsonOpts, 0); err != nil {
				return err
			}
		}
	}

	// Print trailers if requested or verbose
	if (showTrailers || verbose) && len(resp.Trailers) > 0 {
		printTrailers(resp.Trailers)
	}

	return nil
}

// printResponseMessage formats and prints a single response message.
func printResponseMessage(msgBytes []byte, outputDesc protoreflect.MessageDescriptor, jsonOpts *format.JSONOptions, msgNum int) error {
	respMsg := dynamicpb.NewMessage(outputDesc)
	if err := proto.Unmarshal(msgBytes, respMsg); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if outputFormat == "text" {
		// Text format output
		if msgNum > 0 {
			fmt.Printf("--- Message %d ---\n", msgNum)
		}
		printMessageAsText(respMsg, "")
	} else {
		// JSON format output
		jsonOutput, err := format.FormatResponseJSON(respMsg, jsonOpts)
		if err != nil {
			return fmt.Errorf("failed to format response: %w", err)
		}
		fmt.Println(jsonOutput)
	}

	return nil
}

// printMessageAsText prints a protobuf message in text format.
func printMessageAsText(msg *dynamicpb.Message, indent string) {
	msg.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		name := fd.Name()
		if fd.IsList() {
			list := v.List()
			for iter := 0; iter < list.Len(); iter++ {
				printFieldValue(name, fd, list.Get(iter), indent)
			}
		} else if fd.IsMap() {
			mapVal := v.Map()
			mapVal.Range(func(mk protoreflect.MapKey, mv protoreflect.Value) bool {
				fmt.Printf("%s%s[%v]: %v\n", indent, name, mk, mv)
				return true
			})
		} else {
			printFieldValue(name, fd, v, indent)
		}
		return true
	})
}

// printFieldValue prints a single field value.
func printFieldValue(name protoreflect.Name, fd protoreflect.FieldDescriptor, v protoreflect.Value, indent string) {
	switch fd.Kind() {
	case protoreflect.MessageKind:
		fmt.Printf("%s%s {\n", indent, name)
		if dm, ok := v.Message().Interface().(*dynamicpb.Message); ok {
			printMessageAsText(dm, indent+"  ")
		}
		fmt.Printf("%s}\n", indent)
	case protoreflect.EnumKind:
		enumVal := fd.Enum().Values().ByNumber(v.Enum())
		if enumVal != nil {
			fmt.Printf("%s%s: %s\n", indent, name, enumVal.Name())
		} else {
			fmt.Printf("%s%s: %d\n", indent, name, v.Enum())
		}
	case protoreflect.BytesKind:
		fmt.Printf("%s%s: <bytes, len=%d>\n", indent, name, len(v.Bytes()))
	default:
		fmt.Printf("%s%s: %v\n", indent, name, v.Interface())
	}
}

// printGRPCError prints a gRPC error with helpful formatting.
func printGRPCError(status *protocol.Status) {
	fmt.Fprintf(os.Stderr, "ERROR:\n")
	fmt.Fprintf(os.Stderr, "  Code: %s\n", protocol.StatusName(status.Code))
	fmt.Fprintf(os.Stderr, "  Number: %d\n", status.Code)
	if status.Message != "" {
		fmt.Fprintf(os.Stderr, "  Message: %s\n", status.Message)
	}

	// Add helpful suggestions based on error code
	switch status.Code {
	case protocol.StatusUnauthenticated:
		fmt.Fprintf(os.Stderr, "\nHint: Add authentication header with -H 'Authorization: Bearer <token>'\n")
	case protocol.StatusPermissionDenied:
		fmt.Fprintf(os.Stderr, "\nHint: Check if the provided credentials have access to this method\n")
	case protocol.StatusNotFound:
		fmt.Fprintf(os.Stderr, "\nHint: Verify the service and method names are correct\n")
	case protocol.StatusInvalidArgument:
		fmt.Fprintf(os.Stderr, "\nHint: Check the request JSON matches the expected message format\n")
	case protocol.StatusUnavailable:
		fmt.Fprintf(os.Stderr, "\nHint: The service may be down or unreachable. Check the server address.\n")
	case protocol.StatusDeadlineExceeded:
		fmt.Fprintf(os.Stderr, "\nHint: Try increasing the timeout with --max-time\n")
	}
}

// printTrailers prints response trailers.
func printTrailers(trailers map[string]string) {
	fmt.Fprintln(os.Stderr, "\nTrailers:")
	for key, value := range trailers {
		fmt.Fprintf(os.Stderr, "  %s: %s\n", key, value)
	}
}

// suggestMethodFormat provides helpful error messages for method format errors.
func suggestMethodFormat(fullMethod string, err error) error {
	return fmt.Errorf("%w\n\nExpected format: package.Service/Method\nExamples:\n  messages.UserService/GetUser\n  helloworld.Greeter/SayHello", err)
}

// suggestClientError provides helpful error messages for client creation errors.
func suggestClientError(address string, err error) error {
	errStr := err.Error()

	if strings.Contains(errStr, "certificate") || strings.Contains(errStr, "tls") {
		return fmt.Errorf("failed to create client: %w\n\nHints:\n  - Use --plaintext for http:// URLs\n  - Use -k/--insecure to skip certificate verification\n  - Use --cacert to specify a CA certificate", err)
	}

	if strings.Contains(address, "http://") && !plaintext {
		return fmt.Errorf("failed to create client: %w\n\nHint: Use --plaintext flag when connecting to http:// URLs", err)
	}

	return fmt.Errorf("failed to create client: %w", err)
}

// suggestDescriptorError provides helpful error messages for descriptor errors.
func suggestDescriptorError(err error) error {
	errStr := err.Error()

	if strings.Contains(errStr, "reflection") {
		return fmt.Errorf("failed to get service descriptors: %w\n\nHints:\n  - The server may not have reflection enabled\n  - Try providing proto files with -p/--proto flag\n  - Check if authentication is required (-H 'Authorization: Bearer <token>')", err)
	}

	if strings.Contains(errStr, "proto") || strings.Contains(errStr, "not found") {
		return fmt.Errorf("failed to get service descriptors: %w\n\nHints:\n  - Check the proto file path is correct\n  - Use -I/--import-path to specify import directories", err)
	}

	return fmt.Errorf("failed to get service descriptors: %w", err)
}

// suggestMethodNotFound provides helpful error messages when a method is not found.
func suggestMethodNotFound(service, method string, source descriptor.Source, err error) error {
	// Try to list available services for suggestions
	services, listErr := source.ListServices()
	if listErr != nil {
		return err
	}

	var suggestions []string
	for _, svc := range services {
		if strings.Contains(strings.ToLower(svc), strings.ToLower(service)) {
			suggestions = append(suggestions, svc)
		}
	}

	if len(suggestions) > 0 {
		return fmt.Errorf("%w\n\nDid you mean one of these services?\n  %s", err, strings.Join(suggestions, "\n  "))
	}

	return fmt.Errorf("%w\n\nAvailable services:\n  %s\n\nUse 'grpcwebcurl list <address>' to see all services", err, strings.Join(services, "\n  "))
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <address>",
		Short: "List available services",
		Long: `List all services available on the gRPC-Web server.

Uses server reflection if no proto files are specified.

Examples:
  # Using server reflection
  grpcwebcurl list https://api.example.com:443

  # Using proto files
  grpcwebcurl -p api.proto list https://api.example.com:443`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			address := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			c, err := createClient(address)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}
			defer c.Close()

			// Set custom headers
			for _, header := range headers {
				parts := strings.SplitN(header, ":", 2)
				if len(parts) == 2 {
					c.SetHeader(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
				}
			}

			source, err := getDescriptorSource(ctx, address, c)
			if err != nil {
				return fmt.Errorf("failed to get service descriptors: %w", err)
			}

			services, err := source.ListServices()
			if err != nil {
				return err
			}

			for _, svc := range services {
				fmt.Println(svc)
			}

			return nil
		},
	}
}

func describeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "describe <address> [symbol]",
		Short: "Describe a service or message type",
		Long: `Describe a service, method, or message type.

Uses server reflection if no proto files are specified.

Examples:
  # List all services
  grpcwebcurl describe https://api.example.com:443

  # Describe a specific service
  grpcwebcurl describe https://api.example.com:443 package.Service

  # Using proto files
  grpcwebcurl -p api.proto describe localhost package.Service`,
		Args:         cobra.RangeArgs(1, 2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			address := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			c, err := createClient(address)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}
			defer c.Close()

			// Set custom headers
			for _, header := range headers {
				parts := strings.SplitN(header, ":", 2)
				if len(parts) == 2 {
					c.SetHeader(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
				}
			}

			source, err := getDescriptorSource(ctx, address, c)
			if err != nil {
				return fmt.Errorf("failed to get service descriptors: %w", err)
			}

			// If no symbol specified, list services
			if len(args) == 1 {
				services, err := source.ListServices()
				if err != nil {
					return err
				}
				for _, svc := range services {
					fmt.Println(svc)
				}
				return nil
			}

			symbol := args[1]
			printer := format.NewPrinter(os.Stdout, false)

			// Try as service first
			if svc, err := source.FindService(symbol); err == nil {
				printer.PrintServiceDescription(svc)
				return nil
			}

			// Try as generic symbol
			desc, err := source.FindSymbol(symbol)
			if err != nil {
				return err
			}

			// Handle message types
			if msgDesc, ok := desc.(protoreflect.MessageDescriptor); ok {
				printer.PrintMessageDescription(msgDesc)
				return nil
			}

			// Print generic descriptor info
			fmt.Printf("Symbol: %v\n", desc)
			return nil
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("grpcwebcurl version %s\n", version)
		},
	}
}

func completionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for grpcwebcurl.

To load completions:

Bash:
  $ source <(grpcwebcurl completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ grpcwebcurl completion bash > /etc/bash_completion.d/grpcwebcurl
  # macOS:
  $ grpcwebcurl completion bash > $(brew --prefix)/etc/bash_completion.d/grpcwebcurl

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ grpcwebcurl completion zsh > "${fpath[1]}/_grpcwebcurl"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ grpcwebcurl completion fish | source
  # To load completions for each session, execute once:
  $ grpcwebcurl completion fish > ~/.config/fish/completions/grpcwebcurl.fish

PowerShell:
  PS> grpcwebcurl completion powershell | Out-String | Invoke-Expression
  # To load completions for every new session, run:
  PS> grpcwebcurl completion powershell > grpcwebcurl.ps1
  # and source this file from your PowerShell profile.
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		SilenceUsage:          true,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}
}
