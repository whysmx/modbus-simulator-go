# Modbus Simulator Go

A lightweight, single-binary Modbus TCP/RTU simulator written in Go.

## Features

- **Dual Protocol Support**: ModbusTCP and ModbusRTU over TCP
- **Function Codes**: 01 (Coils), 02 (Discrete Inputs), 03 (Holding Registers), 04 (Input Registers)
- **Web UI**: Built-in web interface for configuration
- **Single Binary**: No dependencies, easy deployment
- **In-Memory Storage**: Fast, with optional JSON export

## Quick Start

```bash
# Build
go build -o modbus-simulator ./cmd/modbus-simulator

# Run
./modbus-simulator

# Access web UI at http://localhost:8080
```

## Project Structure

```
├── cmd/modbus-simulator/   # Application entry point
├── internal/
│   ├── model/              # Data models (Connection, Slave, Register)
│   ├── store/              # In-memory storage
│   ├── protocol/           # Modbus TCP/RTU protocol handling
│   ├── server/             # HTTP and Modbus TCP servers
│   └── handler/            # Request handlers
├── web/
│   ├── static/             # CSS, JavaScript
│   └── templates/          # HTML templates
└── go.mod
```

## API Endpoints

- `GET /api/connections` - List all connections
- `POST /api/connections` - Create connection
- `PUT /api/connections/:id` - Update connection
- `DELETE /api/connections/:id` - Delete connection
- `GET /api/connections/:id/slaves` - List slaves
- `POST /api/connections/:id/slaves` - Create slave
- ... (more endpoints)

## License

MIT
