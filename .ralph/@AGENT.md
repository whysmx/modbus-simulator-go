# Build and Run Instructions

## Development

```bash
# Run in development mode
go run ./cmd/modbus-simulator

# Run with race detector
go run -race ./cmd/modbus-simulator
```

## Build

```bash
# Standard build
go build -o modbus-simulator ./cmd/modbus-simulator

# Cross-platform builds
GOOS=linux GOARCH=amd64 go build -o modbus-simulator-linux-amd64 ./cmd/modbus-simulator
GOOS=windows GOARCH=amd64 go build -o modbus-simulator-windows-amd64.exe ./cmd/modbus-simulator
GOOS=darwin GOARCH=arm64 go build -o modbus-simulator-darwin-arm64 ./cmd/modbus-simulator

# Optimized build (smaller binary)
go build -ldflags="-s -w" -o modbus-simulator ./cmd/modbus-simulator
```

## Test

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific package tests
go test -v ./internal/protocol/...
go test -v ./internal/store/...

# Run with coverage
go test -cover ./...
```

## Dependencies

```bash
# Download dependencies
go mod download

# Tidy up go.mod
go mod tidy

# Verify dependencies
go mod verify
```

## Environment

- Go 1.22 or later
- No external runtime dependencies
- Single binary deployment
