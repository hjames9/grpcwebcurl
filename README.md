# grpcwebcurl

A command-line tool for testing gRPC-Web endpoints, similar to [grpcurl](https://github.com/fullstorydev/grpcurl) but specifically designed for gRPC-Web protocol.

## Overview

`grpcwebcurl` allows you to interact with gRPC-Web services from the command line, making it easy to test and debug gRPC-Web endpoints served through proxies like Envoy. It supports server reflection, custom headers, multiple output formats, and both unary and server streaming calls.

## Features

- **gRPC-Web Protocol Support**: Communicates using the gRPC-Web binary protocol
- **Server Reflection**: Automatically discovers services without proto files
- **Multiple Output Formats**: JSON (default) or text format
- **Server Streaming**: Full support for server streaming methods
- **TLS/mTLS Support**: Secure connections with optional client certificates
- **Shell Completions**: Bash, Zsh, Fish, and PowerShell
- **Helpful Error Messages**: Context-aware suggestions for common issues

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/hjames9/grpcwebcurl.git
cd grpcwebcurl

# Build
make build

# Install to GOPATH/bin
make install
```

### Pre-built Binaries

Download pre-built binaries from the [Releases](https://github.com/hjames9/grpcwebcurl/releases) page.

Available platforms:
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64, arm64)

### Cross-compile All Platforms

```bash
make build-all
# Binaries will be in bin/ directory
```

## Quick Start

### List Services (using reflection)

```bash
grpcwebcurl --plaintext list http://localhost:9180
```

### Make a Unary Call

```bash
# Using server reflection (no proto files needed)
grpcwebcurl --plaintext \
  -d '{"user_id": "123"}' \
  http://localhost:9180 \
  mypackage.UserService/GetUser

# Using proto files
grpcwebcurl --plaintext \
  -p api.proto \
  -d '{"user_id": "123"}' \
  http://localhost:9180 \
  mypackage.UserService/GetUser
```

### With Authentication

```bash
grpcwebcurl --plaintext \
  -H 'Authorization: Bearer <token>' \
  -d '{"user_id": "123"}' \
  http://localhost:9180 \
  mypackage.UserService/GetUser
```

## Commands

| Command | Description |
|---------|-------------|
| `grpcwebcurl <address> <method>` | Invoke a gRPC method (default) |
| `grpcwebcurl list <address>` | List available services |
| `grpcwebcurl describe <address> [symbol]` | Describe a service or message |
| `grpcwebcurl completion <shell>` | Generate shell completions |
| `grpcwebcurl version` | Print version information |

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--proto` | `-p` | Proto file(s) for message types |
| `--import-path` | `-I` | Import path for proto files |
| `--data` | `-d` | Request data in JSON (use `@` for stdin) |
| `--header` | `-H` | Custom header in 'Key: Value' format |
| `--plaintext` | | Use plaintext HTTP (no TLS) |
| `--insecure` | `-k` | Skip TLS certificate verification |
| `--cert` | | Client certificate file |
| `--key` | | Client private key file |
| `--cacert` | | CA certificate file |
| `--resolve` | | Resolve host:port to address (e.g., example.com:443:127.0.0.1) |
| `--connect-timeout` | | Connection timeout (default: 10s) |
| `--max-time` | | Request timeout (default: 30s) |
| `--max-msg-sz` | | Max message size (default: 16MB) |
| `--emit-defaults` | | Include default values in output |
| `--format` | `-o` | Output format: json or text |
| `--show-trailers` | | Show response trailers |
| `--verbose` | `-v` | Verbose output |

## Examples

### Basic Usage

```bash
# List services
grpcwebcurl --plaintext list http://localhost:9180

# Describe a service
grpcwebcurl --plaintext describe http://localhost:9180 mypackage.UserService

# Make a call
grpcwebcurl --plaintext \
  -d '{"id": "123"}' \
  http://localhost:9180 \
  mypackage.Service/Method
```

### Authentication

```bash
# Bearer token
grpcwebcurl --plaintext \
  -H 'Authorization: Bearer eyJhbGc...' \
  -d '{"id": "123"}' \
  http://localhost:9180 \
  mypackage.Service/Method

# Multiple headers
grpcwebcurl --plaintext \
  -H 'Authorization: Bearer token' \
  -H 'X-Request-ID: abc123' \
  -d '{"id": "123"}' \
  http://localhost:9180 \
  mypackage.Service/Method
```

### TLS Connections

```bash
# HTTPS (default)
grpcwebcurl \
  -d '{"id": "123"}' \
  https://api.example.com:443 \
  mypackage.Service/Method

# Skip certificate verification
grpcwebcurl -k \
  -d '{"id": "123"}' \
  https://localhost:8443 \
  mypackage.Service/Method

# mTLS with client certificates
grpcwebcurl \
  --cert client.crt \
  --key client.key \
  --cacert ca.crt \
  -d '{"id": "123"}' \
  https://api.example.com:443 \
  mypackage.Service/Method

# Custom DNS resolution with TLS verification
# Useful for testing with custom IP while keeping hostname verification
grpcwebcurl --cacert ca.crt \
  -H 'Host: dev.api.example.com' \
  --resolve api.example.com:443:172.1.230.150 \
  -d '{"id": "123"}' \
  https://api.example.com \
  mypackage.Service/Method
```

### Output Formats

```bash
# JSON output (default)
grpcwebcurl --plaintext \
  -d '{"id": "123"}' \
  http://localhost:9180 \
  mypackage.Service/Method

# Text format output
grpcwebcurl --plaintext \
  -o text \
  -d '{"id": "123"}' \
  http://localhost:9180 \
  mypackage.Service/Method

# Show trailers
grpcwebcurl --plaintext \
  --show-trailers \
  -d '{"id": "123"}' \
  http://localhost:9180 \
  mypackage.Service/Method
```

### Reading from Stdin

```bash
# Pipe JSON data
echo '{"id": "123"}' | grpcwebcurl --plaintext \
  -d @ \
  http://localhost:9180 \
  mypackage.Service/Method

# From a file
cat request.json | grpcwebcurl --plaintext \
  -d @ \
  http://localhost:9180 \
  mypackage.Service/Method
```

### Using Proto Files

```bash
# Single proto file
grpcwebcurl --plaintext \
  -p api.proto \
  -d '{"id": "123"}' \
  http://localhost:9180 \
  mypackage.Service/Method

# With import paths
grpcwebcurl --plaintext \
  -I /path/to/protos \
  -I /path/to/google/protos \
  -p api.proto \
  -d '{"id": "123"}' \
  http://localhost:9180 \
  mypackage.Service/Method
```

## Shell Completions

### Bash

```bash
# Add to current session
source <(grpcwebcurl completion bash)

# Install permanently (Linux)
grpcwebcurl completion bash > /etc/bash_completion.d/grpcwebcurl

# Install permanently (macOS with Homebrew)
grpcwebcurl completion bash > $(brew --prefix)/etc/bash_completion.d/grpcwebcurl
```

### Zsh

```bash
# Enable completions (add to ~/.zshrc if not already)
autoload -U compinit; compinit

# Install
grpcwebcurl completion zsh > "${fpath[1]}/_grpcwebcurl"
```

### Fish

```bash
grpcwebcurl completion fish > ~/.config/fish/completions/grpcwebcurl.fish
```

### PowerShell

```powershell
grpcwebcurl completion powershell | Out-String | Invoke-Expression
```

## Differences from grpcurl

| Feature | grpcurl | grpcwebcurl |
|---------|---------|-------------|
| Protocol | Native gRPC (HTTP/2) | gRPC-Web (HTTP/1.1 or HTTP/2) |
| Use case | Direct gRPC servers | gRPC-Web proxies (Envoy, etc.) |
| Client streaming | Supported | Not supported* |
| Server streaming | Supported | Supported |
| Bidirectional streaming | Supported | Not supported* |
| Reflection | Supported | Supported |

*These limitations are inherent to the gRPC-Web protocol specification.

## When to Use grpcwebcurl

Use `grpcwebcurl` when:
- Testing gRPC-Web endpoints through Envoy or other gRPC-Web proxies
- Debugging what mobile/web clients would see
- Verifying gRPC-Web proxy configuration
- Testing authentication through the proxy layer

Use `grpcurl` when:
- Testing native gRPC servers directly
- Need client streaming or bidirectional streaming
- Connecting directly to gRPC services (bypassing proxy)

## Architecture

```
┌─────────────┐     gRPC-Web      ┌─────────────┐     gRPC      ┌─────────────┐
│ grpcwebcurl │ ───────────────── │    Envoy    │ ────────────  │ gRPC Server │
└─────────────┘   HTTP/1.1 or 2   └─────────────┘   HTTP/2      └─────────────┘
```

## Development

### Building

```bash
make build          # Build binary
make test           # Run tests
make test-coverage  # Run tests with coverage report
make fmt            # Format code
make vet            # Run go vet
make clean          # Clean build artifacts
```

### Running Tests

```bash
# All tests
make test

# With verbose output
make test-verbose

# With coverage
make test-coverage
# Open coverage.html in browser
```

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.
