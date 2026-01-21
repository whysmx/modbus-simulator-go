package model

// ProtocolType defines the Modbus protocol variant
type ProtocolType int

const (
	ModbusRtuOverTcp ProtocolType = 0
	ModbusTcp        ProtocolType = 1
)

// Connection represents a TCP port binding for Modbus communication
type Connection struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Port         int          `json:"port"`
	ProtocolType ProtocolType `json:"protocolType"`
}

// Slave represents a Modbus slave device
type Slave struct {
	ID      string `json:"id"`
	ConnID  string `json:"connId"`
	Name    string `json:"name"`
	SlaveID int    `json:"slaveid"` // 1-247
}

// Register represents a Modbus register with hex data
type Register struct {
	ID           string `json:"id"`
	SlaveID      string `json:"slaveid"`
	StartAddr    int    `json:"startaddr"`    // Logical address
	HexData      string `json:"hexdata"`      // Hex string data
	Names        string `json:"names"`        // Comma-separated names
	Coefficients string `json:"coefficients"` // Comma-separated coefficients
}

// RegisterType defines the type of Modbus register
type RegisterType int

const (
	Coil            RegisterType = iota // Function code 01, address 1-9999
	DiscreteInput                       // Function code 02, address 10001-19999
	InputRegister                       // Function code 04, address 30001-39999
	HoldingRegister                     // Function code 03, address 40001-49999
)

// FunctionCode returns the Modbus function code for reading this register type
func (rt RegisterType) FunctionCode() byte {
	switch rt {
	case Coil:
		return 0x01
	case DiscreteInput:
		return 0x02
	case HoldingRegister:
		return 0x03
	case InputRegister:
		return 0x04
	default:
		return 0x00
	}
}
