package handler

import (
	"testing"

	"github.com/whysmx/modbus-simulator-go/internal/model"
	"github.com/whysmx/modbus-simulator-go/internal/protocol"
	"github.com/whysmx/modbus-simulator-go/internal/store"
)

func setupTestHandler() (*ModbusHandler, *store.Store) {
	s := store.New()
	conn := &model.Connection{ID: "testconnidtestconnidtestconnidab", Name: "Test", Port: 1502}
	s.CreateConnection(conn)

	slave := &model.Slave{ID: "testslaveidtestslaveidtestslave1", ConnID: conn.ID, Name: "Slave1", SlaveAddr: 1}
	s.CreateSlave(slave)

	// Add holding register (40001-40002) with hex data "12345678"
	reg := &model.Register{ID: "holdregidholdreg1holdregid123456", SlaveID: slave.ID, StartAddr: 40001, HexData: "12345678"}
	s.CreateRegister(reg)

	// Add input register (30001-30002)
	inputReg := &model.Register{ID: "inputregid1inputregid1inputregid", SlaveID: slave.ID, StartAddr: 30001, HexData: "AABBCCDD"}
	s.CreateRegister(inputReg)

	// Add coil register (1-8) representing 8 bits
	coilReg := &model.Register{ID: "coilregidcoilregidcoilregid12345", SlaveID: slave.ID, StartAddr: 1, HexData: "A5"}
	s.CreateRegister(coilReg)

	// Add discrete input register (10001-10008)
	discreteReg := &model.Register{ID: "discregiddiscregiddiscregid12345", SlaveID: slave.ID, StartAddr: 10001, HexData: "5A"}
	s.CreateRegister(discreteReg)

	h := NewModbusHandler(s, conn.ID)
	return h, s
}

func TestNewModbusHandler(t *testing.T) {
	s := store.New()
	h := NewModbusHandler(s, "test-conn-id")

	if h == nil {
		t.Fatal("NewModbusHandler returned nil")
	}
	if h.store != s {
		t.Error("Store not set correctly")
	}
	if h.connID != "test-conn-id" {
		t.Errorf("connID = %s, want test-conn-id", h.connID)
	}
}

func TestReadHoldingRegisters(t *testing.T) {
	h, _ := setupTestHandler()

	// Read 2 holding registers starting at address 0 (PDU address, logical 40001)
	data, err := h.ReadHoldingRegisters(1, 0, 2)
	if err != nil {
		t.Fatalf("ReadHoldingRegisters failed: %v", err)
	}

	// Expected data: 0x12 0x34 0x56 0x78
	if len(data) != 4 {
		t.Errorf("Data length = %d, want 4", len(data))
	}
	if data[0] != 0x12 || data[1] != 0x34 || data[2] != 0x56 || data[3] != 0x78 {
		t.Errorf("Data = %02x %02x %02x %02x, want 12 34 56 78", data[0], data[1], data[2], data[3])
	}
}

func TestReadHoldingRegistersInvalidSlave(t *testing.T) {
	h, _ := setupTestHandler()

	// Non-existent slave
	_, err := h.ReadHoldingRegisters(99, 0, 1)
	if err != protocol.ErrIllegalDataAddr {
		t.Errorf("Expected ErrIllegalDataAddr, got %v", err)
	}
}

func TestReadInputRegisters(t *testing.T) {
	h, _ := setupTestHandler()

	// Read 2 input registers starting at address 0 (PDU address, logical 30001)
	data, err := h.ReadInputRegisters(1, 0, 2)
	if err != nil {
		t.Fatalf("ReadInputRegisters failed: %v", err)
	}

	// Expected data: 0xAA 0xBB 0xCC 0xDD
	if len(data) != 4 {
		t.Errorf("Data length = %d, want 4", len(data))
	}
	if data[0] != 0xAA || data[1] != 0xBB || data[2] != 0xCC || data[3] != 0xDD {
		t.Errorf("Data = %02x %02x %02x %02x, want AA BB CC DD", data[0], data[1], data[2], data[3])
	}
}

func TestReadInputRegistersInvalidSlave(t *testing.T) {
	h, _ := setupTestHandler()

	_, err := h.ReadInputRegisters(99, 0, 1)
	if err != protocol.ErrIllegalDataAddr {
		t.Errorf("Expected ErrIllegalDataAddr, got %v", err)
	}
}

func TestReadCoils(t *testing.T) {
	h, _ := setupTestHandler()

	// Read 8 coils starting at address 0 (PDU address, logical 1)
	// HexData is "A5" = 10100101
	data, err := h.ReadCoils(1, 0, 8)
	if err != nil {
		t.Fatalf("ReadCoils failed: %v", err)
	}

	// Expected: 1 byte for 8 coils
	if len(data) != 1 {
		t.Errorf("Data length = %d, want 1", len(data))
	}
	// 0xA5 = 10100101 in big endian, but coils are bit 0 first
	if data[0] != 0xA5 {
		t.Errorf("Data = %02x, want A5", data[0])
	}
}

func TestReadCoilsInvalidSlave(t *testing.T) {
	h, _ := setupTestHandler()

	_, err := h.ReadCoils(99, 0, 1)
	if err != protocol.ErrIllegalDataAddr {
		t.Errorf("Expected ErrIllegalDataAddr, got %v", err)
	}
}

func TestReadDiscreteInputs(t *testing.T) {
	h, _ := setupTestHandler()

	// Read 8 discrete inputs starting at address 0 (PDU address, logical 10001)
	data, err := h.ReadDiscreteInputs(1, 0, 8)
	if err != nil {
		t.Fatalf("ReadDiscreteInputs failed: %v", err)
	}

	// Expected: 1 byte for 8 bits
	if len(data) != 1 {
		t.Errorf("Data length = %d, want 1", len(data))
	}
	// 0x5A = 01011010
	if data[0] != 0x5A {
		t.Errorf("Data = %02x, want 5A", data[0])
	}
}

func TestReadDiscreteInputsInvalidSlave(t *testing.T) {
	h, _ := setupTestHandler()

	_, err := h.ReadDiscreteInputs(99, 0, 1)
	if err != protocol.ErrIllegalDataAddr {
		t.Errorf("Expected ErrIllegalDataAddr, got %v", err)
	}
}

func TestToLogicalAddress(t *testing.T) {
	h, _ := setupTestHandler()

	tests := []struct {
		pduAddr  uint16
		regType  model.RegisterType
		expected int
	}{
		{0, model.Coil, 1},
		{99, model.Coil, 100},
		{0, model.DiscreteInput, 10001},
		{99, model.DiscreteInput, 10100},
		{0, model.InputRegister, 30001},
		{99, model.InputRegister, 30100},
		{0, model.HoldingRegister, 40001},
		{99, model.HoldingRegister, 40100},
	}

	for _, tt := range tests {
		result := h.toLogicalAddress(tt.pduAddr, tt.regType)
		if result != tt.expected {
			t.Errorf("toLogicalAddress(%d, %d) = %d, want %d", tt.pduAddr, tt.regType, result, tt.expected)
		}
	}
}

func TestToLogicalAddressDefault(t *testing.T) {
	h, _ := setupTestHandler()

	// Test with an invalid register type (default case)
	result := h.toLogicalAddress(0, model.RegisterType(99))
	if result != 1 {
		t.Errorf("toLogicalAddress with invalid type = %d, want 1", result)
	}
}

func TestReadRegistersUnmappedAddress(t *testing.T) {
	h, _ := setupTestHandler()

	// Read registers that aren't mapped - should return zeros
	// Address 100 (logical 40101) is not in our test data
	data, err := h.ReadHoldingRegisters(1, 100, 1)
	if err != nil {
		t.Fatalf("ReadHoldingRegisters failed: %v", err)
	}

	// Should return zeros for unmapped addresses
	if len(data) != 2 {
		t.Errorf("Data length = %d, want 2", len(data))
	}
	if data[0] != 0 || data[1] != 0 {
		t.Errorf("Unmapped address should return zeros, got %02x %02x", data[0], data[1])
	}
}

func TestReadCoilsMultipleBytes(t *testing.T) {
	s := store.New()
	conn := &model.Connection{ID: "testconnmultibytestconnmultibyte", Name: "Test", Port: 1503}
	s.CreateConnection(conn)

	slave := &model.Slave{ID: "slavemultibyteslavemultibyteslav", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	// Add 16 coils (2 bytes)
	coilReg := &model.Register{ID: "coil16bitidcoil16bitidcoil16bit", SlaveID: slave.ID, StartAddr: 1, HexData: "A5F0"}
	s.CreateRegister(coilReg)

	h := NewModbusHandler(s, conn.ID)

	// Read 16 coils
	data, err := h.ReadCoils(1, 0, 16)
	if err != nil {
		t.Fatalf("ReadCoils failed: %v", err)
	}

	if len(data) != 2 {
		t.Errorf("Data length = %d, want 2", len(data))
	}
}

func TestReadCoilsPartialByte(t *testing.T) {
	h, _ := setupTestHandler()

	// Read only 3 coils (less than a byte)
	data, err := h.ReadCoils(1, 0, 3)
	if err != nil {
		t.Fatalf("ReadCoils failed: %v", err)
	}

	// Should still return 1 byte
	if len(data) != 1 {
		t.Errorf("Data length = %d, want 1", len(data))
	}
}

func TestCollectRegisterDataInvalidHex(t *testing.T) {
	s := store.New()
	conn := &model.Connection{ID: "testconninvalidhexconntest123456", Name: "Test", Port: 1504}
	s.CreateConnection(conn)

	slave := &model.Slave{ID: "slaveinvalidhexslaveinvalidhexsl", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	// Add register with odd hex length (invalid for decoding)
	reg := &model.Register{ID: "badregidbadreg1badregid12345678", SlaveID: slave.ID, StartAddr: 40001, HexData: "123"}
	s.CreateRegister(reg)

	h := NewModbusHandler(s, conn.ID)

	// Should still work but return zeros for invalid hex
	data, err := h.ReadHoldingRegisters(1, 0, 1)
	if err != nil {
		t.Fatalf("ReadHoldingRegisters failed: %v", err)
	}

	if len(data) != 2 {
		t.Errorf("Data length = %d, want 2", len(data))
	}
}

func TestReadRegistersPartialData(t *testing.T) {
	s := store.New()
	conn := &model.Connection{ID: "testconnpartialdata123456789012", Name: "Test", Port: 1505}
	s.CreateConnection(conn)

	slave := &model.Slave{ID: "slavepartialslavepart123456789", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	// Add register with only 1 register worth of data
	reg := &model.Register{ID: "regpartialregpartialreg12345678", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"}
	s.CreateRegister(reg)

	h := NewModbusHandler(s, conn.ID)

	// Read 2 registers - second one will be unmapped
	data, err := h.ReadHoldingRegisters(1, 0, 2)
	if err != nil {
		t.Fatalf("ReadHoldingRegisters failed: %v", err)
	}

	if len(data) != 4 {
		t.Errorf("Data length = %d, want 4", len(data))
	}
	// First register should have data
	if data[0] != 0x12 || data[1] != 0x34 {
		t.Errorf("First register = %02x%02x, want 1234", data[0], data[1])
	}
	// Second register should be zeros
	if data[2] != 0 || data[3] != 0 {
		t.Errorf("Second register = %02x%02x, want 0000", data[2], data[3])
	}
}

func TestCollectBitDataUnmapped(t *testing.T) {
	h, _ := setupTestHandler()

	// Read coils from unmapped range
	data, err := h.ReadCoils(1, 100, 8)
	if err != nil {
		t.Fatalf("ReadCoils failed: %v", err)
	}

	// Should return zeros for unmapped coils
	if len(data) != 1 {
		t.Errorf("Data length = %d, want 1", len(data))
	}
	if data[0] != 0 {
		t.Errorf("Unmapped coils should be 0, got %02x", data[0])
	}
}

// Quantity boundary tests

func TestReadHoldingRegistersQuantityZero(t *testing.T) {
	h, _ := setupTestHandler()

	// Read with quantity = 0
	data, err := h.ReadHoldingRegisters(1, 0, 0)
	if err != nil {
		t.Fatalf("ReadHoldingRegisters with quantity=0 failed: %v", err)
	}

	// Should return empty data
	if len(data) != 0 {
		t.Errorf("Data length for quantity=0 = %d, want 0", len(data))
	}
}

func TestReadCoilsQuantityZero(t *testing.T) {
	h, _ := setupTestHandler()

	// Read with quantity = 0
	data, err := h.ReadCoils(1, 0, 0)
	if err != nil {
		t.Fatalf("ReadCoils with quantity=0 failed: %v", err)
	}

	// Should return empty data
	if len(data) != 0 {
		t.Errorf("Data length for quantity=0 = %d, want 0", len(data))
	}
}

func TestReadHoldingRegistersLargeQuantity(t *testing.T) {
	h, _ := setupTestHandler()

	// Read more registers than available - should return zeros for unmapped
	data, err := h.ReadHoldingRegisters(1, 0, 100)
	if err != nil {
		t.Fatalf("ReadHoldingRegisters failed: %v", err)
	}

	// Should return 100 registers * 2 bytes = 200 bytes
	if len(data) != 200 {
		t.Errorf("Data length = %d, want 200", len(data))
	}

	// First 2 registers should have data (our test data is at 40001-40002)
	if data[0] != 0x12 || data[1] != 0x34 {
		t.Errorf("First register = %02X%02X, want 1234", data[0], data[1])
	}
}

func TestReadInputRegistersValid(t *testing.T) {
	h, _ := setupTestHandler()

	// Read input registers (30001-30002)
	data, err := h.ReadInputRegisters(1, 0, 2)
	if err != nil {
		t.Fatalf("ReadInputRegisters failed: %v", err)
	}

	if len(data) != 4 {
		t.Errorf("Data length = %d, want 4", len(data))
	}
	// Our test data has AABBCCDD at input register 30001
	if data[0] != 0xAA || data[1] != 0xBB {
		t.Errorf("First input register = %02X%02X, want AABB", data[0], data[1])
	}
}

func TestReadDiscreteInputsValid(t *testing.T) {
	h, _ := setupTestHandler()

	// Read discrete inputs (10001-10008)
	data, err := h.ReadDiscreteInputs(1, 0, 8)
	if err != nil {
		t.Fatalf("ReadDiscreteInputs failed: %v", err)
	}

	if len(data) != 1 {
		t.Errorf("Data length = %d, want 1", len(data))
	}
	// Our test data has 0x5A at discrete input 10001
	// 5A = 0101 1010
	if data[0] != 0x5A {
		t.Errorf("Discrete inputs = %02X, want 5A", data[0])
	}
}

func TestReadCoilsBeyondDataRange(t *testing.T) {
	h, _ := setupTestHandler()

	// Read 16 coils starting at 0, but our test data only has 8 coils (0xA5)
	data, err := h.ReadCoils(1, 0, 16)
	if err != nil {
		t.Fatalf("ReadCoils failed: %v", err)
	}

	// Should return 2 bytes
	if len(data) != 2 {
		t.Errorf("Data length = %d, want 2", len(data))
	}
	// First byte should have our data, second should be zeros
	if data[0] != 0xA5 {
		t.Errorf("First coil byte = %02X, want A5", data[0])
	}
	if data[1] != 0x00 {
		t.Errorf("Second coil byte = %02X, want 00 (beyond mapped range)", data[1])
	}
}

// Write function tests

func TestWriteSingleCoil(t *testing.T) {
	h, s := setupTestHandler()

	// Write coil at address 0 (logical 1) with ON value (0xFF00)
	err := h.WriteSingleCoil(1, 0, []byte{0xFF, 0x00})
	if err != nil {
		t.Fatalf("WriteSingleCoil failed: %v", err)
	}

	// Verify the coil was written
	data, err := h.ReadCoils(1, 0, 8)
	if err != nil {
		t.Fatalf("ReadCoils failed: %v", err)
	}
	// Original was 0xA5 = 10100101, after setting bit 0, should be 10100101 (already set)
	// Actually test with OFF value
	err = h.WriteSingleCoil(1, 0, []byte{0x00, 0x00})
	if err != nil {
		t.Fatalf("WriteSingleCoil OFF failed: %v", err)
	}
	data, err = h.ReadCoils(1, 0, 8)
	if err != nil {
		t.Fatalf("ReadCoils after OFF failed: %v", err)
	}
	// 0xA5 = 10100101, after clearing bit 0, should be 10100100 = 0xA4
	if data[0] != 0xA4 {
		t.Errorf("Coil after clear = %02X, want A4", data[0])
	}

	// Verify store is still accessible
	_ = s
}

func TestWriteSingleCoilInvalidSlave(t *testing.T) {
	h, _ := setupTestHandler()

	err := h.WriteSingleCoil(99, 0, []byte{0xFF, 0x00})
	if err == nil {
		t.Error("Expected error for invalid slave")
	}
}

func TestWriteSingleRegister(t *testing.T) {
	h, _ := setupTestHandler()

	// Write to holding register at address 0 (logical 40001)
	err := h.WriteSingleRegister(1, 0, []byte{0xAB, 0xCD})
	if err != nil {
		t.Fatalf("WriteSingleRegister failed: %v", err)
	}

	// Verify the register was written
	data, err := h.ReadHoldingRegisters(1, 0, 1)
	if err != nil {
		t.Fatalf("ReadHoldingRegisters failed: %v", err)
	}
	if data[0] != 0xAB || data[1] != 0xCD {
		t.Errorf("Register = %02X%02X, want ABCD", data[0], data[1])
	}
}

func TestWriteSingleRegisterInvalidSlave(t *testing.T) {
	h, _ := setupTestHandler()

	err := h.WriteSingleRegister(99, 0, []byte{0xAB, 0xCD})
	if err == nil {
		t.Error("Expected error for invalid slave")
	}
}

func TestWriteMultipleCoils(t *testing.T) {
	h, _ := setupTestHandler()

	// Write 8 coils at address 0 (logical 1) with 0xFF (all ON)
	err := h.WriteMultipleCoils(1, 0, 8, []byte{0xFF})
	if err != nil {
		t.Fatalf("WriteMultipleCoils failed: %v", err)
	}

	// Verify the coils were written
	data, err := h.ReadCoils(1, 0, 8)
	if err != nil {
		t.Fatalf("ReadCoils failed: %v", err)
	}
	if data[0] != 0xFF {
		t.Errorf("Coils = %02X, want FF", data[0])
	}
}

func TestWriteMultipleCoilsInvalidSlave(t *testing.T) {
	h, _ := setupTestHandler()

	err := h.WriteMultipleCoils(99, 0, 8, []byte{0xFF})
	if err == nil {
		t.Error("Expected error for invalid slave")
	}
}

func TestWriteMultipleRegisters(t *testing.T) {
	h, _ := setupTestHandler()

	// Write 2 registers at address 0 (logical 40001)
	err := h.WriteMultipleRegisters(1, 0, 2, []byte{0xDE, 0xAD, 0xBE, 0xEF})
	if err != nil {
		t.Fatalf("WriteMultipleRegisters failed: %v", err)
	}

	// Verify the registers were written
	data, err := h.ReadHoldingRegisters(1, 0, 2)
	if err != nil {
		t.Fatalf("ReadHoldingRegisters failed: %v", err)
	}
	if data[0] != 0xDE || data[1] != 0xAD || data[2] != 0xBE || data[3] != 0xEF {
		t.Errorf("Registers = %02X%02X%02X%02X, want DEADBEEF", data[0], data[1], data[2], data[3])
	}
}

func TestWriteMultipleRegistersInvalidSlave(t *testing.T) {
	h, _ := setupTestHandler()

	err := h.WriteMultipleRegisters(99, 0, 2, []byte{0xDE, 0xAD, 0xBE, 0xEF})
	if err == nil {
		t.Error("Expected error for invalid slave")
	}
}

func TestWriteMultipleRegistersPartialData(t *testing.T) {
	h, _ := setupTestHandler()

	// Write with less data than quantity specifies (should handle gracefully)
	err := h.WriteMultipleRegisters(1, 0, 3, []byte{0xDE, 0xAD})
	if err != nil {
		t.Fatalf("WriteMultipleRegisters with partial data failed: %v", err)
	}
}

func TestWriteBitsUnmappedAddress(t *testing.T) {
	h, _ := setupTestHandler()

	// Write to unmapped coil address - should succeed (no error on unmapped)
	err := h.WriteSingleCoil(1, 100, []byte{0xFF, 0x00})
	if err != nil {
		t.Fatalf("WriteSingleCoil to unmapped address failed: %v", err)
	}
}

func TestWriteRegistersUnmappedAddress(t *testing.T) {
	h, _ := setupTestHandler()

	// Write to unmapped register address - should succeed (no error on unmapped)
	err := h.WriteSingleRegister(1, 100, []byte{0xAB, 0xCD})
	if err != nil {
		t.Fatalf("WriteSingleRegister to unmapped address failed: %v", err)
	}
}
