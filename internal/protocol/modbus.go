package protocol

import (
	"encoding/binary"
	"errors"
)

var (
	ErrInvalidFrame     = errors.New("invalid frame")
	ErrInvalidCRC       = errors.New("invalid CRC")
	ErrInvalidProtocol  = errors.New("invalid protocol ID")
	ErrInvalidLength    = errors.New("invalid length field")
	ErrIllegalFunction  = errors.New("illegal function")
	ErrIllegalDataAddr  = errors.New("illegal data address")
	ErrIllegalDataValue = errors.New("illegal data value")
)

// Modbus Exception Codes (per Modbus Application Protocol V1.1b3)
const (
	ExceptionIllegalFunction  = 0x01 // Function code not supported
	ExceptionIllegalDataAddr  = 0x02 // Address out of range
	ExceptionIllegalDataValue = 0x03 // Value out of range (e.g., quantity)
	ExceptionSlaveFailure     = 0x04 // Unrecoverable error in slave
	ExceptionAcknowledge      = 0x05 // Request accepted, processing
	ExceptionSlaveBusy        = 0x06 // Slave is busy
)

// Modbus quantity limits (per Modbus Application Protocol V1.1b3)
const (
	MaxCoilsRead     = 2000 // FC01, FC02: max 2000 bits
	MaxRegistersRead = 125  // FC03, FC04: max 125 registers (16-bit)
	MaxCoilsWrite    = 1968 // FC0F: max 1968 bits
	MaxRegistersWrite = 123 // FC10: max 123 registers (16-bit)
)

// Modbus function codes
const (
	FCReadCoils          = 0x01
	FCReadDiscreteInputs = 0x02
	FCReadHoldingRegs    = 0x03
	FCReadInputRegs      = 0x04
	FCWriteSingleCoil    = 0x05
	FCWriteSingleReg     = 0x06
	FCWriteMultipleCoils = 0x0F
	FCWriteMultipleRegs  = 0x10
)

// BroadcastAddress is the Modbus broadcast address (no response expected)
const BroadcastAddress = 0x00

// Modbus PDU address range is 0-65535 (16-bit)
const (
	MaxAddressPerType = 0xFFFF
)

// ValidUnitIDMin and ValidUnitIDMax define valid slave address range
const (
	ValidUnitIDMin = 1   // Minimum valid slave address
	ValidUnitIDMax = 247 // Maximum valid slave address (248-255 reserved)
)

// Request represents a parsed Modbus request
type Request struct {
	TransactionID uint16 // Only for TCP
	ProtocolID    uint16 // Only for TCP (always 0)
	UnitID        byte   // Slave address
	FunctionCode  byte
	StartAddress  uint16
	Quantity      uint16
	WriteData     []byte // Data for write operations (FC05, FC06, FC0F, FC10)
	RawData       []byte
}

// Response represents a Modbus response to be sent
type Response struct {
	TransactionID uint16
	ProtocolID    uint16
	UnitID        byte
	FunctionCode  byte
	Data          []byte
	IsError       bool
	ErrorCode     byte
	IsEcho        bool // For FC05/FC06/FC0F/FC10: echo request instead of byte count format
}

// Handler interface for processing Modbus requests
type Handler interface {
	// ReadCoils handles function code 0x01
	ReadCoils(slaveID byte, startAddr, quantity uint16) ([]byte, error)
	// ReadDiscreteInputs handles function code 0x02
	ReadDiscreteInputs(slaveID byte, startAddr, quantity uint16) ([]byte, error)
	// ReadHoldingRegisters handles function code 0x03
	ReadHoldingRegisters(slaveID byte, startAddr, quantity uint16) ([]byte, error)
	// ReadInputRegisters handles function code 0x04
	ReadInputRegisters(slaveID byte, startAddr, quantity uint16) ([]byte, error)
}

// ParseTCPRequest parses a Modbus TCP frame
func ParseTCPRequest(data []byte) (*Request, error) {
	if len(data) < 12 {
		return nil, ErrInvalidFrame
	}

	// Validate Protocol ID (must be 0x0000 for Modbus)
	protocolID := binary.BigEndian.Uint16(data[2:4])
	if protocolID != 0 {
		return nil, ErrInvalidProtocol
	}

	// Validate Length field (bytes following = UnitID + PDU)
	length := binary.BigEndian.Uint16(data[4:6])
	if int(length) != len(data)-6 {
		return nil, ErrInvalidLength
	}

	functionCode := data[7]
	req := &Request{
		TransactionID: binary.BigEndian.Uint16(data[0:2]),
		ProtocolID:    protocolID,
		UnitID:        data[6],
		FunctionCode:  functionCode,
		StartAddress:  binary.BigEndian.Uint16(data[8:10]),
		RawData:       data,
	}

	// Parse based on function code
	switch functionCode {
	case FCReadCoils, FCReadDiscreteInputs, FCReadHoldingRegs, FCReadInputRegs:
		// Read requests: Address(2) + Quantity(2)
		req.Quantity = binary.BigEndian.Uint16(data[10:12])
	case FCWriteSingleCoil, FCWriteSingleReg:
		// Write single: Address(2) + Value(2)
		req.Quantity = 1
		if len(data) >= 12 {
			req.WriteData = data[10:12]
		}
	case FCWriteMultipleCoils, FCWriteMultipleRegs:
		// Write multiple: Address(2) + Quantity(2) + ByteCount(1) + Data(n)
		if len(data) < 13 {
			return nil, ErrInvalidFrame
		}
		req.Quantity = binary.BigEndian.Uint16(data[10:12])
		byteCount := data[12]
		if len(data) < 13+int(byteCount) {
			return nil, ErrInvalidFrame
		}
		req.WriteData = data[13 : 13+int(byteCount)]
	default:
		// Unknown function code - still parse basic fields
		if len(data) >= 12 {
			req.Quantity = binary.BigEndian.Uint16(data[10:12])
		}
	}

	return req, nil
}

// ParseRTURequest parses a Modbus RTU over TCP frame
func ParseRTURequest(data []byte) (*Request, error) {
	if len(data) < 8 {
		return nil, ErrInvalidFrame
	}

	// Validate CRC
	if !ValidateCRC(data) {
		return nil, ErrInvalidCRC
	}

	functionCode := data[1]
	req := &Request{
		UnitID:       data[0],
		FunctionCode: functionCode,
		StartAddress: binary.BigEndian.Uint16(data[2:4]),
		RawData:      data,
	}

	// Parse based on function code (exclude 2-byte CRC at end)
	pduLen := len(data) - 2
	switch functionCode {
	case FCReadCoils, FCReadDiscreteInputs, FCReadHoldingRegs, FCReadInputRegs:
		// Read requests: Address(2) + Quantity(2)
		req.Quantity = binary.BigEndian.Uint16(data[4:6])
	case FCWriteSingleCoil, FCWriteSingleReg:
		// Write single: Address(2) + Value(2)
		req.Quantity = 1
		if pduLen >= 6 {
			req.WriteData = data[4:6]
		}
	case FCWriteMultipleCoils, FCWriteMultipleRegs:
		// Write multiple: Address(2) + Quantity(2) + ByteCount(1) + Data(n)
		if pduLen < 7 {
			return nil, ErrInvalidFrame
		}
		req.Quantity = binary.BigEndian.Uint16(data[4:6])
		byteCount := data[6]
		if pduLen < 7+int(byteCount) {
			return nil, ErrInvalidFrame
		}
		req.WriteData = data[7 : 7+int(byteCount)]
	default:
		// Unknown function code - still parse basic fields
		if pduLen >= 6 {
			req.Quantity = binary.BigEndian.Uint16(data[4:6])
		}
	}

	return req, nil
}

// BuildTCPResponse builds a Modbus TCP response frame
func BuildTCPResponse(resp *Response) []byte {
	var pdu []byte

	if resp.IsError {
		pdu = []byte{resp.FunctionCode | 0x80, resp.ErrorCode}
	} else if resp.IsEcho {
		// FC05/FC06/FC0F/FC10: echo format (no byte count prefix)
		pdu = append([]byte{resp.FunctionCode}, resp.Data...)
	} else {
		// FC01-FC04: byte count format
		pdu = append([]byte{resp.FunctionCode, byte(len(resp.Data))}, resp.Data...)
	}

	length := len(pdu) + 1 // PDU + UnitID
	frame := make([]byte, 7+len(pdu))

	binary.BigEndian.PutUint16(frame[0:2], resp.TransactionID)
	binary.BigEndian.PutUint16(frame[2:4], resp.ProtocolID)
	binary.BigEndian.PutUint16(frame[4:6], uint16(length))
	frame[6] = resp.UnitID
	copy(frame[7:], pdu)

	return frame
}

// BuildRTUResponse builds a Modbus RTU over TCP response frame
func BuildRTUResponse(resp *Response) []byte {
	var pdu []byte

	if resp.IsError {
		pdu = []byte{resp.UnitID, resp.FunctionCode | 0x80, resp.ErrorCode}
	} else if resp.IsEcho {
		// FC05/FC06/FC0F/FC10: echo format (no byte count prefix)
		pdu = append([]byte{resp.UnitID, resp.FunctionCode}, resp.Data...)
	} else {
		// FC01-FC04: byte count format
		pdu = append([]byte{resp.UnitID, resp.FunctionCode, byte(len(resp.Data))}, resp.Data...)
	}

	crc := CRC16Bytes(pdu)
	return append(pdu, crc[0], crc[1])
}

// ValidateQuantity checks if quantity is valid for the given function code
// Returns ErrIllegalDataValue if quantity is out of range
func ValidateQuantity(functionCode byte, quantity uint16) error {
	// FC05/FC06 have quantity=1 implicitly, no validation needed
	if functionCode == FCWriteSingleCoil || functionCode == FCWriteSingleReg {
		return nil
	}

	if quantity == 0 {
		return ErrIllegalDataValue
	}

	switch functionCode {
	case FCReadCoils, FCReadDiscreteInputs: // FC01, FC02: max 2000 bits
		if quantity > MaxCoilsRead {
			return ErrIllegalDataValue
		}
	case FCReadHoldingRegs, FCReadInputRegs: // FC03, FC04: max 125 registers
		if quantity > MaxRegistersRead {
			return ErrIllegalDataValue
		}
	case FCWriteMultipleCoils: // FC0F: max 1968 bits
		if quantity > MaxCoilsWrite {
			return ErrIllegalDataValue
		}
	case FCWriteMultipleRegs: // FC10: max 123 registers
		if quantity > MaxRegistersWrite {
			return ErrIllegalDataValue
		}
	}
	return nil
}

// ValidateAddressRange checks if startAddress + quantity doesn't overflow
// and is within Modbus PDU range (0-65535)
// Returns ErrIllegalDataAddr if address range is invalid
func ValidateAddressRange(startAddress, quantity uint16) error {
	// Check Modbus PDU address range (0-65535)
	if startAddress > MaxAddressPerType {
		return ErrIllegalDataAddr
	}
	// Check for overflow within type range
	endAddress := uint32(startAddress) + uint32(quantity) - 1
	if endAddress > MaxAddressPerType {
		return ErrIllegalDataAddr
	}
	return nil
}

// IsValidUnitID checks if the unit ID is in valid slave range (1-247)
// Returns false for broadcast (0) and reserved (248-255) addresses
func IsValidUnitID(unitID byte) bool {
	return unitID >= ValidUnitIDMin && unitID <= ValidUnitIDMax
}

// IsBroadcast returns true if the unit ID is the broadcast address
func IsBroadcast(unitID byte) bool {
	return unitID == BroadcastAddress
}
