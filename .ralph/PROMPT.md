# Modbus Simulator Go - Development Instructions

## Project Overview

Build a lightweight Modbus TCP/RTU simulator in Go. This is a rewrite of the existing .NET + Next.js version, simplified to a single binary with embedded web UI.

## Core Requirements

### 1. Modbus Protocol Support
- **ModbusTCP**: MBAP header (7 bytes) + PDU
- **ModbusRTU over TCP**: Slave address + Function code + Data + CRC-16
- **Function Codes**: 01, 02, 03, 04 (read operations only)

### 2. Data Model
- **Connection**: TCP port binding with protocol type selection
- **Slave**: Device under connection, address 1-247
- **Register**: Hex data storage with logical addressing

### 3. Web Interface
- Device tree: Connection → Slave → Register hierarchy
- Register table: Multi-format data display (Hex, Int16, Float32, etc.)
- CRUD dialogs for all entities

### 4. API Endpoints
Replicate the existing .NET API:
- `/api/connections` - CRUD for connections
- `/api/connections/:id/slaves` - CRUD for slaves
- `/api/connections/:id/slaves/:slaveId/registers` - CRUD for registers
- `/api/connections/tree` - Full hierarchy tree

## Technical Stack

- **Web Framework**: Fiber v2 (Express-like, high performance)
- **UI**: htmx + Tailwind CSS (embedded in binary)
- **Storage**: In-memory with mutex protection
- **No external database**

## Implementation Priorities

1. Complete the HTTP API endpoints
2. Implement Modbus TCP server with protocol handling
3. Build the web UI with htmx
4. Add data format conversions (Hex ↔ Int16 ↔ Float32)
5. Testing and cross-platform builds

## Code Style

- Use standard Go project layout
- Keep packages small and focused
- Error handling: return errors, don't panic
- Use context for cancellation
- Thread-safe storage with sync.RWMutex

## Build & Run

```bash
# Development
go run ./cmd/modbus-simulator

# Build
go build -o modbus-simulator ./cmd/modbus-simulator

# Test
go test ./...
```

## Address Mapping

| Function Code | Register Type | Protocol Address | Logical Address |
|---------------|---------------|------------------|-----------------|
| 01 | Coil | 0-9998 | 1-9999 |
| 02 | Discrete Input | 0-9998 | 10001-19999 |
| 03 | Holding Register | 0-9998 | 40001-49999 |
| 04 | Input Register | 0-9998 | 30001-39999 |

## RALPH_STATUS Block

When completing a task, include:
```
RALPH_STATUS:
PROGRESS: percentage%
TASKS_COMPLETED: list
TASKS_REMAINING: list
EXIT_SIGNAL: true/false
```

Set EXIT_SIGNAL: true only when ALL features are implemented and tested.
