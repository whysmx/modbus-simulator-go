# Modbus Simulator Go - Technical Specifications

## Protocol Specifications

### ModbusTCP Frame Format

```
MBAP Header (7 bytes):
  [0-1] Transaction ID (echoed in response)
  [2-3] Protocol ID (always 0x0000)
  [4-5] Length (bytes following, including Unit ID)
  [6]   Unit ID (slave address)

PDU:
  [7]   Function Code
  [8+]  Data (function-specific)
```

### ModbusRTU over TCP Frame Format

```
  [0]     Slave Address
  [1]     Function Code
  [2-n-2] Data
  [n-1]   CRC Low Byte
  [n]     CRC High Byte
```

### Function Code Request/Response

#### FC 01/02 - Read Coils/Discrete Inputs
Request:
```
  [0-1] Start Address
  [2-3] Quantity of Coils (1-2000)
```
Response:
```
  [0]   Byte Count (N = Quantity/8, rounded up)
  [1-N] Coil Status (bit-packed, LSB first)
```

#### FC 03/04 - Read Holding/Input Registers
Request:
```
  [0-1] Start Address
  [2-3] Quantity of Registers (1-125)
```
Response:
```
  [0]     Byte Count (2 * Quantity)
  [1-2N]  Register Values (2 bytes each, big-endian)
```

### Error Response
```
  [0] Function Code | 0x80
  [1] Exception Code:
      0x01 = Illegal Function
      0x02 = Illegal Data Address
      0x03 = Illegal Data Value
```

## Data Model

### Connection
```go
type Connection struct {
    ID           string       // UUID
    Name         string       // Display name
    Port         int          // TCP port (1-65535)
    ProtocolType ProtocolType // 0=RTU, 1=TCP
}
```

### Slave
```go
type Slave struct {
    ID      string // UUID
    ConnID  string // Parent connection
    Name    string // Display name
    SlaveID int    // Address (1-247)
}
```

### Register
```go
type Register struct {
    ID           string // UUID
    SlaveID      string // Parent slave
    StartAddr    int    // Logical address
    HexData      string // Hex string (e.g., "0A1B2C3D")
    Names        string // Comma-separated names
    Coefficients string // Comma-separated coefficients
}
```

## Address Ranges

| Type | Logical Range | Function Code |
|------|---------------|---------------|
| Coil | 1-9999 | 01 |
| Discrete Input | 10001-19999 | 02 |
| Input Register | 30001-39999 | 04 |
| Holding Register | 40001-49999 | 03 |

## API Specification

### Connections

```
GET    /api/connections           → []Connection
POST   /api/connections           → Connection
PUT    /api/connections/:id       → Connection
DELETE /api/connections/:id       → 204
GET    /api/connections/tree      → []ConnectionWithSlaves
GET    /api/connections/protocol-types → []ProtocolType
```

### Slaves

```
GET    /api/connections/:connId/slaves           → []Slave
POST   /api/connections/:connId/slaves           → Slave
PUT    /api/connections/:connId/slaves/:id       → Slave
DELETE /api/connections/:connId/slaves/:id       → 204
```

### Registers

```
GET    /api/connections/:connId/slaves/:slaveId/registers      → []Register
POST   /api/connections/:connId/slaves/:slaveId/registers      → Register
PUT    /api/connections/:connId/slaves/:slaveId/registers/:id  → Register
DELETE /api/connections/:connId/slaves/:slaveId/registers/:id  → 204
```

## Float32 Byte Orders

| Name | Order | Example (1234.5) |
|------|-------|------------------|
| ABCD | Big-endian | 44 9A 50 00 |
| CDAB | Little-endian swap | 50 00 44 9A |
| BADC | Byte swap | 9A 44 00 50 |
| DCBA | Full reverse | 00 50 9A 44 |
