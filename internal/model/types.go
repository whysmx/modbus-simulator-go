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
	ID        string `json:"id"`
	ConnID    string `json:"connId"`
	Name      string `json:"name"`
	SlaveAddr int    `json:"slaveAddr"` // 1-247
}

// Register represents a Modbus register with hex data
type Register struct {
	ID           string `json:"id"`
	SlaveID      string `json:"slaveId"`
	StartAddr    int    `json:"startAddr"`
	HexData      string `json:"hexData"`
	Names        string `json:"names"`
	Coefficients string `json:"coefficients"`
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

// GetRegisterType determines the register type from a logical address
func GetRegisterType(addr int) RegisterType {
	switch {
	case addr >= 1 && addr <= 9999:
		return Coil
	case addr >= 10001 && addr <= 19999:
		return DiscreteInput
	case addr >= 30001 && addr <= 39999:
		return InputRegister
	case addr >= 40001 && addr <= 49999:
		return HoldingRegister
	default:
		return Coil // Default fallback
	}
}

// IsValidAddress checks if an address is in a valid Modbus range
func IsValidAddress(addr int) bool {
	return (addr >= 1 && addr <= 9999) ||
		(addr >= 10001 && addr <= 19999) ||
		(addr >= 30001 && addr <= 39999) ||
		(addr >= 40001 && addr <= 49999)
}

// GetAddressOffset calculates the 0-based offset within the register type
func GetAddressOffset(addr int) uint16 {
	switch {
	case addr >= 1 && addr <= 9999:
		return uint16(addr - 1)
	case addr >= 10001 && addr <= 19999:
		return uint16(addr - 10001)
	case addr >= 30001 && addr <= 39999:
		return uint16(addr - 30001)
	case addr >= 40001 && addr <= 49999:
		return uint16(addr - 40001)
	default:
		return 0
	}
}

// RegisterCount calculates the number of registers/coils based on hexData length and type
func (r *Register) RegisterCount() int {
	regType := GetRegisterType(r.StartAddr)
	switch regType {
	case Coil, DiscreteInput:
		// Bit type: 2 hex = 1 byte = 8 bits
		return len(r.HexData) / 2 * 8
	case InputRegister, HoldingRegister:
		// Register type: 4 hex = 1 register (16 bits)
		return len(r.HexData) / 4
	default:
		return 0
	}
}

// EndAddr returns the ending logical address (inclusive)
func (r *Register) EndAddr() int {
	count := r.RegisterCount()
	if count == 0 {
		return r.StartAddr
	}
	return r.StartAddr + count - 1
}

// Overlaps checks if this register overlaps with another register of the same type
func (r *Register) Overlaps(other *Register) bool {
	// Different types cannot overlap
	if GetRegisterType(r.StartAddr) != GetRegisterType(other.StartAddr) {
		return false
	}
	// Check interval overlap: [a1, a2] and [b1, b2]
	return r.StartAddr <= other.EndAddr() && other.StartAddr <= r.EndAddr()
}
