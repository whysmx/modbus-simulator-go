# Fix Plan - Modbus Simulator Go

## Priority 1: Core Infrastructure [IN PROGRESS]

- [x] Project structure setup
- [x] Data models (Connection, Slave, Register)
- [x] In-memory store with thread safety
- [x] Modbus protocol parsing (TCP + RTU)
- [x] CRC-16 calculation
- [ ] HTTP server setup with Fiber
- [ ] API endpoint handlers

## Priority 2: Modbus TCP Server

- [ ] TCP listener management (start/stop per connection)
- [ ] ModbusTCP protocol handler
- [ ] ModbusRTU over TCP protocol handler
- [ ] Function code 01: Read Coils
- [ ] Function code 02: Read Discrete Inputs
- [ ] Function code 03: Read Holding Registers
- [ ] Function code 04: Read Input Registers
- [ ] Error response handling

## Priority 3: HTTP API

- [ ] GET /api/connections - List connections
- [ ] POST /api/connections - Create connection
- [ ] PUT /api/connections/:id - Update connection
- [ ] DELETE /api/connections/:id - Delete connection
- [ ] GET /api/connections/tree - Full hierarchy
- [ ] GET /api/connections/protocol-types - Protocol enum
- [ ] Slave CRUD endpoints
- [ ] Register CRUD endpoints

## Priority 4: Web UI

- [ ] Base HTML template with Tailwind
- [ ] Device tree component (htmx)
- [ ] Register table component
- [ ] Connection config dialog
- [ ] Slave config dialog
- [ ] Register config dialog
- [ ] Data format switching (Hex/Int16/Float32)

## Priority 5: Data Handling

- [ ] Hex string parsing/validation
- [ ] Register data overlap handling
- [ ] Int16/UInt16 conversion
- [ ] Float32 conversion (4 byte orders: ABCD, CDAB, BADC, DCBA)
- [ ] Bit extraction for coils

## Priority 6: Polish

- [ ] Cross-platform build script
- [ ] Embed static files in binary
- [ ] Unit tests for protocol
- [ ] Integration tests for API
- [ ] JSON export/import for persistence

## Blocked

(none)

## Notes

- Reference implementation: /Users/wen/Desktop/code/10Modbus/modbus-simulator
- Keep it simple: no database, no complex abstractions
- Target: single binary < 10MB
