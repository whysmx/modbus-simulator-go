package protocol

import (
	"encoding/binary"
	"errors"
)

var (
	ErrInvalidFrame     = errors.New("invalid frame")
	ErrInvalidCRC       = errors.New("invalid CRC")
	ErrIllegalFunction  = errors.New("illegal function")
	ErrIllegalDataAddr  = errors.New("illegal data address")
	ErrIllegalDataValue = errors.New("illegal data value")
)

// ModbusError codes
const (
	ExceptionIllegalFunction  = 0x01
	ExceptionIllegalDataAddr  = 0x02
	ExceptionIllegalDataValue = 0x03
)

// Request represents a parsed Modbus request
type Request struct {
	TransactionID uint16 // Only for TCP
	ProtocolID    uint16 // Only for TCP (always 0)
	UnitID        byte   // Slave address
	FunctionCode  byte
	StartAddress  uint16
	Quantity      uint16
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

	req := &Request{
		TransactionID: binary.BigEndian.Uint16(data[0:2]),
		ProtocolID:    binary.BigEndian.Uint16(data[2:4]),
		UnitID:        data[6],
		FunctionCode:  data[7],
		StartAddress:  binary.BigEndian.Uint16(data[8:10]),
		Quantity:      binary.BigEndian.Uint16(data[10:12]),
		RawData:       data,
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

	req := &Request{
		UnitID:       data[0],
		FunctionCode: data[1],
		StartAddress: binary.BigEndian.Uint16(data[2:4]),
		Quantity:     binary.BigEndian.Uint16(data[4:6]),
		RawData:      data,
	}

	return req, nil
}

// BuildTCPResponse builds a Modbus TCP response frame
func BuildTCPResponse(resp *Response) []byte {
	var pdu []byte

	if resp.IsError {
		pdu = []byte{resp.FunctionCode | 0x80, resp.ErrorCode}
	} else {
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
	} else {
		pdu = append([]byte{resp.UnitID, resp.FunctionCode, byte(len(resp.Data))}, resp.Data...)
	}

	crc := CRC16Bytes(pdu)
	return append(pdu, crc[0], crc[1])
}
