package handler

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
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
	groupDeltaByRegID := make(map[string]int)

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
						delta, ok := groupDeltaByRegID[reg.ID]
						if !ok {
							delta = randomDelta(clampJitterAmp(reg.JitterAmp))
							groupDeltaByRegID[reg.ID] = delta
						}

						value := int(bytes[0])<<8 | int(bytes[1])
						value += delta
						if value < 0 {
							value = 0
						} else if value > 0xFFFF {
							value = 0xFFFF
						}

						result[i*2] = byte(value >> 8)
						result[i*2+1] = byte(value)
						found = true
						break
					}
				}
			}
		}

		if !found {
			// DESIGN DECISION: Return zeros for unmapped addresses instead of exception.
			// This is intentional simulator behavior to allow flexible testing scenarios.
			// Per strict Modbus spec, unmapped addresses should return Exception Code 0x02
			// (Illegal Data Address), but for simulation purposes we return zero values.
			result[i*2] = 0
			result[i*2+1] = 0
		}
	}

	return result
}

func clampJitterAmp(amp int) int {
	if amp < 0 {
		return 0
	}
	if amp > model.MaxJitterAmp {
		return model.MaxJitterAmp
	}
	return amp
}

func randomDelta(amp int) int {
	if amp <= 0 {
		return 0
	}

	rangeSize := int64(amp*2 + 1)
	n, err := rand.Int(rand.Reader, big.NewInt(rangeSize))
	if err != nil {
		return 0
	}

	return int(n.Int64()) - amp
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

// WriteSingleCoil handles function code 0x05
func (h *ModbusHandler) WriteSingleCoil(slaveID byte, address uint16, value []byte) error {
	slave, err := h.store.GetSlaveByAddress(h.connID, int(slaveID))
	if err != nil {
		return protocol.ErrIllegalDataAddr
	}

	logicalAddr := h.toLogicalAddress(address, model.Coil)
	return h.writeBits(slave.ID, logicalAddr, 1, value, model.Coil)
}

// WriteSingleRegister handles function code 0x06
func (h *ModbusHandler) WriteSingleRegister(slaveID byte, address uint16, value []byte) error {
	slave, err := h.store.GetSlaveByAddress(h.connID, int(slaveID))
	if err != nil {
		return protocol.ErrIllegalDataAddr
	}

	logicalAddr := h.toLogicalAddress(address, model.HoldingRegister)
	return h.writeRegisters(slave.ID, logicalAddr, 1, value, model.HoldingRegister)
}

// WriteMultipleCoils handles function code 0x0F
func (h *ModbusHandler) WriteMultipleCoils(slaveID byte, startAddr, quantity uint16, data []byte) error {
	slave, err := h.store.GetSlaveByAddress(h.connID, int(slaveID))
	if err != nil {
		return protocol.ErrIllegalDataAddr
	}

	logicalStart := h.toLogicalAddress(startAddr, model.Coil)
	return h.writeBits(slave.ID, logicalStart, int(quantity), data, model.Coil)
}

// WriteMultipleRegisters handles function code 0x10
func (h *ModbusHandler) WriteMultipleRegisters(slaveID byte, startAddr, quantity uint16, data []byte) error {
	slave, err := h.store.GetSlaveByAddress(h.connID, int(slaveID))
	if err != nil {
		return protocol.ErrIllegalDataAddr
	}

	logicalStart := h.toLogicalAddress(startAddr, model.HoldingRegister)
	return h.writeRegisters(slave.ID, logicalStart, int(quantity), data, model.HoldingRegister)
}

// writeBits writes coil data to matching registers
func (h *ModbusHandler) writeBits(slaveID string, logicalStart, quantity int, data []byte, regType model.RegisterType) error {
	registers := h.store.GetRegistersBySlave(slaveID)

	for i := 0; i < quantity; i++ {
		addr := logicalStart + i
		byteIdx := i / 8
		bitIdx := i % 8
		bitValue := false
		if byteIdx < len(data) {
			bitValue = (data[byteIdx] & (1 << bitIdx)) != 0
		}

		for _, reg := range registers {
			if model.GetRegisterType(reg.StartAddr) != regType {
				continue
			}

			if addr >= reg.StartAddr && addr <= reg.EndAddr() {
				// Update bit in register's hex data
				bitOffset := addr - reg.StartAddr
				regByteIdx := bitOffset / 8
				regBitIdx := bitOffset % 8

				hexData := strings.ToUpper(reg.HexData)
				if regByteIdx*2+2 <= len(hexData) {
					hexByte := hexData[regByteIdx*2 : regByteIdx*2+2]
					bytes, err := hex.DecodeString(hexByte)
					if err == nil && len(bytes) == 1 {
						if bitValue {
							bytes[0] |= (1 << regBitIdx)
						} else {
							bytes[0] &^= (1 << regBitIdx)
						}
						// Update hex data
						newHex := hexData[:regByteIdx*2] + strings.ToUpper(hex.EncodeToString(bytes)) + hexData[regByteIdx*2+2:]
						reg.HexData = newHex
						h.store.UpdateRegister(reg)
					}
				}
				break
			}
		}
	}

	return nil
}

// writeRegisters writes register data to matching registers
func (h *ModbusHandler) writeRegisters(slaveID string, logicalStart, quantity int, data []byte, regType model.RegisterType) error {
	registers := h.store.GetRegistersBySlave(slaveID)

	for i := 0; i < quantity; i++ {
		addr := logicalStart + i
		if i*2+2 > len(data) {
			break
		}
		value := data[i*2 : i*2+2]

		for _, reg := range registers {
			if model.GetRegisterType(reg.StartAddr) != regType {
				continue
			}

			if addr >= reg.StartAddr && addr <= reg.EndAddr() {
				offset := (addr - reg.StartAddr) * 4 // 4 hex chars per register
				if offset+4 <= len(reg.HexData) {
					newHex := reg.HexData[:offset] + strings.ToUpper(hex.EncodeToString(value)) + reg.HexData[offset+4:]
					reg.HexData = newHex
					h.store.UpdateRegister(reg)
				}
				break
			}
		}
	}

	return nil
}
