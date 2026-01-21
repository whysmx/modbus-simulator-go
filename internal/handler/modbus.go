package handler

import (
	"encoding/hex"
	"strings"

	"github.com/whysmx/modbus-simulator-go/internal/model"
	"github.com/whysmx/modbus-simulator-go/internal/protocol"
	"github.com/whysmx/modbus-simulator-go/internal/store"
)

// ModbusHandler implements protocol.Handler for reading data from store
type ModbusHandler struct {
	store  *store.Store
	connID string
}

// NewModbusHandler creates a handler for a specific connection
func NewModbusHandler(s *store.Store, connID string) *ModbusHandler {
	return &ModbusHandler{
		store:  s,
		connID: connID,
	}
}

// ReadCoils handles function code 0x01
func (h *ModbusHandler) ReadCoils(slaveID byte, startAddr, quantity uint16) ([]byte, error) {
	return h.readBits(slaveID, startAddr, quantity, model.Coil)
}

// ReadDiscreteInputs handles function code 0x02
func (h *ModbusHandler) ReadDiscreteInputs(slaveID byte, startAddr, quantity uint16) ([]byte, error) {
	return h.readBits(slaveID, startAddr, quantity, model.DiscreteInput)
}

// ReadHoldingRegisters handles function code 0x03
func (h *ModbusHandler) ReadHoldingRegisters(slaveID byte, startAddr, quantity uint16) ([]byte, error) {
	return h.readRegisters(slaveID, startAddr, quantity, model.HoldingRegister)
}

// ReadInputRegisters handles function code 0x04
func (h *ModbusHandler) ReadInputRegisters(slaveID byte, startAddr, quantity uint16) ([]byte, error) {
	return h.readRegisters(slaveID, startAddr, quantity, model.InputRegister)
}

// readBits reads coils or discrete inputs
func (h *ModbusHandler) readBits(slaveID byte, startAddr, quantity uint16, regType model.RegisterType) ([]byte, error) {
	slave, err := h.store.GetSlaveByAddress(h.connID, int(slaveID))
	if err != nil {
		return nil, protocol.ErrIllegalDataAddr
	}

	// Convert to logical address
	logicalStart := h.toLogicalAddress(startAddr, regType)

	// Find registers that contain requested data
	registers := h.store.GetRegistersBySlave(slave.ID)
	data := h.collectBitData(registers, logicalStart, int(quantity), regType)
	if data == nil {
		return nil, protocol.ErrIllegalDataAddr
	}

	return data, nil
}

// readRegisters reads holding or input registers
func (h *ModbusHandler) readRegisters(slaveID byte, startAddr, quantity uint16, regType model.RegisterType) ([]byte, error) {
	slave, err := h.store.GetSlaveByAddress(h.connID, int(slaveID))
	if err != nil {
		return nil, protocol.ErrIllegalDataAddr
	}

	// Convert to logical address
	logicalStart := h.toLogicalAddress(startAddr, regType)

	// Find registers that contain requested data
	registers := h.store.GetRegistersBySlave(slave.ID)
	data := h.collectRegisterData(registers, logicalStart, int(quantity), regType)
	if data == nil {
		return nil, protocol.ErrIllegalDataAddr
	}

	return data, nil
}

// toLogicalAddress converts 0-based PDU address to logical 1-based address
func (h *ModbusHandler) toLogicalAddress(pduAddr uint16, regType model.RegisterType) int {
	switch regType {
	case model.Coil:
		return int(pduAddr) + 1
	case model.DiscreteInput:
		return int(pduAddr) + 10001
	case model.InputRegister:
		return int(pduAddr) + 30001
	case model.HoldingRegister:
		return int(pduAddr) + 40001
	default:
		return int(pduAddr) + 1
	}
}

// collectRegisterData collects register data from matching registers
func (h *ModbusHandler) collectRegisterData(registers []*model.Register, logicalStart, quantity int, regType model.RegisterType) []byte {
	result := make([]byte, quantity*2)

	for i := 0; i < quantity; i++ {
		addr := logicalStart + i
		found := false

		for _, reg := range registers {
			if model.GetRegisterType(reg.StartAddr) != regType {
				continue
			}

			if addr >= reg.StartAddr && addr <= reg.EndAddr() {
				offset := (addr - reg.StartAddr) * 4 // 4 hex chars per register
				if offset+4 <= len(reg.HexData) {
					hexStr := reg.HexData[offset : offset+4]
					bytes, err := hex.DecodeString(hexStr)
					if err == nil && len(bytes) == 2 {
						result[i*2] = bytes[0]
						result[i*2+1] = bytes[1]
						found = true
						break
					}
				}
			}
		}

		if !found {
			// Return zeros for unmapped addresses
			result[i*2] = 0
			result[i*2+1] = 0
		}
	}

	return result
}

// collectBitData collects bit data from matching registers
func (h *ModbusHandler) collectBitData(registers []*model.Register, logicalStart, quantity int, regType model.RegisterType) []byte {
	// Calculate bytes needed
	byteCount := (quantity + 7) / 8
	result := make([]byte, byteCount)

	for i := 0; i < quantity; i++ {
		addr := logicalStart + i
		bitValue := false

		for _, reg := range registers {
			if model.GetRegisterType(reg.StartAddr) != regType {
				continue
			}

			if addr >= reg.StartAddr && addr <= reg.EndAddr() {
				// Calculate bit position within register's hex data
				bitOffset := addr - reg.StartAddr
				byteIdx := bitOffset / 8
				bitIdx := bitOffset % 8

				// Convert hex to bytes
				hexData := strings.ToUpper(reg.HexData)
				if byteIdx*2+2 <= len(hexData) {
					hexByte := hexData[byteIdx*2 : byteIdx*2+2]
					bytes, err := hex.DecodeString(hexByte)
					if err == nil && len(bytes) == 1 {
						bitValue = (bytes[0] & (1 << bitIdx)) != 0
					}
				}
				break
			}
		}

		// Set bit in result
		if bitValue {
			result[i/8] |= (1 << (i % 8))
		}
	}

	return result
}
