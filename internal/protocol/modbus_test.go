package protocol

import "testing"

func TestCRC16(t *testing.T) {
	// Test with known Modbus CRC values
	// CRC16 returns the CRC as uint16 in native byte order
	// For data {0x01, 0x03, 0x00, 0x00, 0x00, 0x01}, the CRC is 0x0A84
	// where low byte = 0x84, high byte = 0x0A
	tests := []struct {
		data     []byte
		expected uint16
	}{
		// Modbus request: Read Holding Registers, slave 1, addr 0, count 1
		{[]byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x01}, 0x0A84},
	}

	for i, tt := range tests {
		result := CRC16(tt.data)
		if result != tt.expected {
			t.Errorf("Test %d: CRC16 = %04X, want %04X", i, result, tt.expected)
		}
	}
}

func TestCRC16Bytes(t *testing.T) {
	data := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x01}
	result := CRC16Bytes(data)
	// CRC is 0x0A84, returned as [low, high]: 0x84, 0x0A
	if result[0] != 0x84 || result[1] != 0x0A {
		t.Errorf("CRC16Bytes = [%02X %02X], want [84 0A]", result[0], result[1])
	}
}

func TestValidateCRC(t *testing.T) {
	// Valid frame with CRC (CRC bytes are [low, high]: 0x84, 0x0A)
	valid := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x01, 0x84, 0x0A}
	if !ValidateCRC(valid) {
		t.Error("ValidateCRC returned false for valid frame")
	}

	// Invalid CRC
	invalid := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00}
	if ValidateCRC(invalid) {
		t.Error("ValidateCRC returned true for invalid frame")
	}

	// Too short
	short := []byte{0x01, 0x02}
	if ValidateCRC(short) {
		t.Error("ValidateCRC returned true for too short frame")
	}
}

func TestParseTCPRequest(t *testing.T) {
	// Modbus TCP request: Read Holding Registers
	// Transaction ID: 0x0001, Protocol: 0x0000, Length: 0x0006
	// Unit ID: 0x01, Function: 0x03, Start: 0x0000, Quantity: 0x0001
	data := []byte{
		0x00, 0x01, // Transaction ID
		0x00, 0x00, // Protocol ID
		0x00, 0x06, // Length
		0x01,       // Unit ID
		0x03,       // Function Code
		0x00, 0x00, // Start Address
		0x00, 0x01, // Quantity
	}

	req, err := ParseTCPRequest(data)
	if err != nil {
		t.Fatalf("ParseTCPRequest failed: %v", err)
	}

	if req.TransactionID != 1 {
		t.Errorf("TransactionID = %d, want 1", req.TransactionID)
	}
	if req.UnitID != 1 {
		t.Errorf("UnitID = %d, want 1", req.UnitID)
	}
	if req.FunctionCode != 0x03 {
		t.Errorf("FunctionCode = %02X, want 03", req.FunctionCode)
	}
	if req.StartAddress != 0 {
		t.Errorf("StartAddress = %d, want 0", req.StartAddress)
	}
	if req.Quantity != 1 {
		t.Errorf("Quantity = %d, want 1", req.Quantity)
	}
}

func TestParseRTURequest(t *testing.T) {
	// Modbus RTU request: Read Holding Registers with CRC
	// CRC bytes are [low, high]: 0x84, 0x0A
	data := []byte{
		0x01,       // Slave Address
		0x03,       // Function Code
		0x00, 0x00, // Start Address
		0x00, 0x01, // Quantity
		0x84, 0x0A, // CRC (low, high)
	}

	req, err := ParseRTURequest(data)
	if err != nil {
		t.Fatalf("ParseRTURequest failed: %v", err)
	}

	if req.UnitID != 1 {
		t.Errorf("UnitID = %d, want 1", req.UnitID)
	}
	if req.FunctionCode != 0x03 {
		t.Errorf("FunctionCode = %02X, want 03", req.FunctionCode)
	}
}

func TestBuildTCPResponse(t *testing.T) {
	resp := &Response{
		TransactionID: 1,
		ProtocolID:    0,
		UnitID:        1,
		FunctionCode:  0x03,
		Data:          []byte{0x00, 0x64}, // Value 100
	}

	result := BuildTCPResponse(resp)

	// Check header
	if result[0] != 0x00 || result[1] != 0x01 { // Transaction ID
		t.Errorf("Transaction ID = %02X%02X, want 0001", result[0], result[1])
	}
	if result[6] != 0x01 { // Unit ID
		t.Errorf("Unit ID = %02X, want 01", result[6])
	}
	if result[7] != 0x03 { // Function Code
		t.Errorf("Function Code = %02X, want 03", result[7])
	}
}

func TestBuildTCPErrorResponse(t *testing.T) {
	resp := &Response{
		TransactionID: 1,
		ProtocolID:    0,
		UnitID:        1,
		FunctionCode:  0x03,
		IsError:       true,
		ErrorCode:     ExceptionIllegalDataAddr,
	}

	result := BuildTCPResponse(resp)

	// Function code should have high bit set
	if result[7] != 0x83 {
		t.Errorf("Error function code = %02X, want 83", result[7])
	}
	if result[8] != ExceptionIllegalDataAddr {
		t.Errorf("Error code = %02X, want %02X", result[8], ExceptionIllegalDataAddr)
	}
}

func TestBuildRTUResponse(t *testing.T) {
	resp := &Response{
		UnitID:       1,
		FunctionCode: 0x03,
		Data:         []byte{0x00, 0x64},
	}

	result := BuildRTUResponse(resp)

	if result[0] != 0x01 { // Unit ID
		t.Errorf("Unit ID = %02X, want 01", result[0])
	}
	if result[1] != 0x03 { // Function Code
		t.Errorf("Function Code = %02X, want 03", result[1])
	}

	// Last 2 bytes should be CRC
	if len(result) < 4 {
		t.Fatalf("Response too short: %d bytes", len(result))
	}

	// Validate CRC
	if !ValidateCRC(result) {
		t.Error("Response has invalid CRC")
	}
}

// Error path tests for ParseTCPRequest and ParseRTURequest

func TestParseTCPRequestTooShort(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"1 byte", []byte{0x00}},
		{"6 bytes", []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x06}},
		{"11 bytes", []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x01, 0x03, 0x00, 0x00, 0x00}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTCPRequest(tt.data)
			if err != ErrInvalidFrame {
				t.Errorf("ParseTCPRequest(%d bytes) error = %v, want ErrInvalidFrame", len(tt.data), err)
			}
		})
	}
}

func TestParseRTURequestTooShort(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"1 byte", []byte{0x01}},
		{"7 bytes", []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x01, 0x84}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseRTURequest(tt.data)
			if err != ErrInvalidFrame {
				t.Errorf("ParseRTURequest(%d bytes) error = %v, want ErrInvalidFrame", len(tt.data), err)
			}
		})
	}
}

func TestParseRTURequestInvalidCRC(t *testing.T) {
	// Valid length but bad CRC
	data := []byte{
		0x01,       // Slave Address
		0x03,       // Function Code
		0x00, 0x00, // Start Address
		0x00, 0x01, // Quantity
		0xFF, 0xFF, // Invalid CRC
	}

	_, err := ParseRTURequest(data)
	if err != ErrInvalidCRC {
		t.Errorf("ParseRTURequest with bad CRC error = %v, want ErrInvalidCRC", err)
	}
}

func TestBuildRTUErrorResponse(t *testing.T) {
	resp := &Response{
		UnitID:       1,
		FunctionCode: 0x03,
		IsError:      true,
		ErrorCode:    ExceptionIllegalDataAddr,
	}

	result := BuildRTUResponse(resp)

	// Error response: UnitID(1) + FC|0x80(1) + ErrorCode(1) + CRC(2) = 5 bytes
	if len(result) != 5 {
		t.Errorf("Error response length = %d, want 5", len(result))
	}
	if result[0] != 0x01 {
		t.Errorf("Unit ID = %02X, want 01", result[0])
	}
	if result[1] != 0x83 { // 0x03 | 0x80
		t.Errorf("Function Code = %02X, want 83", result[1])
	}
	if result[2] != ExceptionIllegalDataAddr {
		t.Errorf("Error code = %02X, want %02X", result[2], ExceptionIllegalDataAddr)
	}
	if !ValidateCRC(result) {
		t.Error("Error response has invalid CRC")
	}
}

// Tests for validation functions per Modbus Application Protocol V1.1b3

func TestValidateQuantity(t *testing.T) {
	tests := []struct {
		name         string
		functionCode byte
		quantity     uint16
		wantErr      error
	}{
		// Zero quantity is always invalid
		{"FC01 quantity 0", 0x01, 0, ErrIllegalDataValue},
		{"FC02 quantity 0", 0x02, 0, ErrIllegalDataValue},
		{"FC03 quantity 0", 0x03, 0, ErrIllegalDataValue},
		{"FC04 quantity 0", 0x04, 0, ErrIllegalDataValue},
		// Valid quantities
		{"FC01 quantity 1", 0x01, 1, nil},
		{"FC01 quantity 2000", 0x01, 2000, nil},
		{"FC02 quantity 2000", 0x02, 2000, nil},
		{"FC03 quantity 1", 0x03, 1, nil},
		{"FC03 quantity 125", 0x03, 125, nil},
		{"FC04 quantity 125", 0x04, 125, nil},
		// Exceeds max for coils/discrete inputs (max 2000)
		{"FC01 quantity 2001", 0x01, 2001, ErrIllegalDataValue},
		{"FC02 quantity 2001", 0x02, 2001, ErrIllegalDataValue},
		// Exceeds max for registers (max 125)
		{"FC03 quantity 126", 0x03, 126, ErrIllegalDataValue},
		{"FC04 quantity 126", 0x04, 126, ErrIllegalDataValue},
		// Unknown function code (no limit enforced)
		{"FC05 quantity 1", 0x05, 1, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateQuantity(tt.functionCode, tt.quantity)
			if err != tt.wantErr {
				t.Errorf("ValidateQuantity(0x%02X, %d) = %v, want %v",
					tt.functionCode, tt.quantity, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAddressRange(t *testing.T) {
	tests := []struct {
		name         string
		startAddress uint16
		quantity     uint16
		wantErr      error
	}{
		// Valid ranges (0-65535 in PDU)
		{"start 0 qty 1", 0, 1, nil},
		{"start 0 qty 9999", 0, 9999, nil},
		{"start 5000 qty 4999", 5000, 4999, nil},
		{"start 65535 qty 1", 65535, 1, nil},
		{"start 65534 qty 2", 65534, 2, nil},
		// Exceeds PDU range
		{"start 65535 qty 2", 65535, 2, ErrIllegalDataAddr},
		{"start 65530 qty 10", 65530, 10, ErrIllegalDataAddr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAddressRange(tt.startAddress, tt.quantity)
			if err != tt.wantErr {
				t.Errorf("ValidateAddressRange(%d, %d) = %v, want %v",
					tt.startAddress, tt.quantity, err, tt.wantErr)
			}
		})
	}
}

func TestIsBroadcast(t *testing.T) {
	tests := []struct {
		unitID   byte
		expected bool
	}{
		{0x00, true},  // Broadcast address
		{0x01, false}, // Valid slave 1
		{0xFF, false}, // Valid slave 255
		{0x7F, false}, // Valid slave 127
	}

	for _, tt := range tests {
		result := IsBroadcast(tt.unitID)
		if result != tt.expected {
			t.Errorf("IsBroadcast(0x%02X) = %v, want %v", tt.unitID, result, tt.expected)
		}
	}
}

func TestParseTCPRequestInvalidProtocolID(t *testing.T) {
	// Protocol ID must be 0x0000 for Modbus TCP
	data := []byte{
		0x00, 0x01, // Transaction ID
		0x00, 0x01, // Invalid Protocol ID (should be 0x0000)
		0x00, 0x06, // Length
		0x01,       // Unit ID
		0x03,       // Function Code
		0x00, 0x00, // Start Address
		0x00, 0x01, // Quantity
	}

	_, err := ParseTCPRequest(data)
	if err != ErrInvalidProtocol {
		t.Errorf("ParseTCPRequest with invalid Protocol ID error = %v, want ErrInvalidProtocol", err)
	}
}

func TestParseTCPRequestInvalidLength(t *testing.T) {
	// Length field doesn't match actual data length
	data := []byte{
		0x00, 0x01, // Transaction ID
		0x00, 0x00, // Protocol ID
		0x00, 0x10, // Invalid Length (claims 16 bytes follow, but only 6)
		0x01,       // Unit ID
		0x03,       // Function Code
		0x00, 0x00, // Start Address
		0x00, 0x01, // Quantity
	}

	_, err := ParseTCPRequest(data)
	if err != ErrInvalidLength {
		t.Errorf("ParseTCPRequest with invalid Length error = %v, want ErrInvalidLength", err)
	}
}

func TestParseTCPRequestWriteSingleCoil(t *testing.T) {
	// FC05: Write Single Coil - Address(2) + Value(2)
	data := []byte{
		0x00, 0x01, // Transaction ID
		0x00, 0x00, // Protocol ID
		0x00, 0x06, // Length
		0x01,       // Unit ID
		0x05,       // Function Code (Write Single Coil)
		0x00, 0x0A, // Address (10)
		0xFF, 0x00, // Value (ON)
	}

	req, err := ParseTCPRequest(data)
	if err != nil {
		t.Fatalf("ParseTCPRequest failed: %v", err)
	}

	if req.FunctionCode != FCWriteSingleCoil {
		t.Errorf("FunctionCode = %02X, want %02X", req.FunctionCode, FCWriteSingleCoil)
	}
	if req.StartAddress != 10 {
		t.Errorf("StartAddress = %d, want 10", req.StartAddress)
	}
	if req.Quantity != 1 {
		t.Errorf("Quantity = %d, want 1", req.Quantity)
	}
	if len(req.WriteData) != 2 || req.WriteData[0] != 0xFF || req.WriteData[1] != 0x00 {
		t.Errorf("WriteData = %02X, want FF00", req.WriteData)
	}
}

func TestParseTCPRequestWriteSingleReg(t *testing.T) {
	// FC06: Write Single Register - Address(2) + Value(2)
	data := []byte{
		0x00, 0x01, // Transaction ID
		0x00, 0x00, // Protocol ID
		0x00, 0x06, // Length
		0x01,       // Unit ID
		0x06,       // Function Code (Write Single Register)
		0x00, 0x05, // Address (5)
		0x12, 0x34, // Value
	}

	req, err := ParseTCPRequest(data)
	if err != nil {
		t.Fatalf("ParseTCPRequest failed: %v", err)
	}

	if req.FunctionCode != FCWriteSingleReg {
		t.Errorf("FunctionCode = %02X, want %02X", req.FunctionCode, FCWriteSingleReg)
	}
	if req.StartAddress != 5 {
		t.Errorf("StartAddress = %d, want 5", req.StartAddress)
	}
	if len(req.WriteData) != 2 || req.WriteData[0] != 0x12 || req.WriteData[1] != 0x34 {
		t.Errorf("WriteData = %02X, want 1234", req.WriteData)
	}
}

func TestParseTCPRequestWriteMultipleRegs(t *testing.T) {
	// FC10: Write Multiple Registers - Address(2) + Quantity(2) + ByteCount(1) + Data(n)
	data := []byte{
		0x00, 0x01, // Transaction ID
		0x00, 0x00, // Protocol ID
		0x00, 0x0B, // Length (11 bytes: UnitID + FC + Addr + Qty + ByteCount + Data)
		0x01,       // Unit ID
		0x10,       // Function Code (Write Multiple Registers)
		0x00, 0x00, // Start Address
		0x00, 0x02, // Quantity (2 registers)
		0x04,       // Byte Count
		0x00, 0x0A, // Register 1 value (10)
		0x01, 0x02, // Register 2 value (258)
	}

	req, err := ParseTCPRequest(data)
	if err != nil {
		t.Fatalf("ParseTCPRequest failed: %v", err)
	}

	if req.FunctionCode != FCWriteMultipleRegs {
		t.Errorf("FunctionCode = %02X, want %02X", req.FunctionCode, FCWriteMultipleRegs)
	}
	if req.Quantity != 2 {
		t.Errorf("Quantity = %d, want 2", req.Quantity)
	}
	if len(req.WriteData) != 4 {
		t.Errorf("WriteData length = %d, want 4", len(req.WriteData))
	}
}

func TestIsValidUnitID(t *testing.T) {
	tests := []struct {
		unitID   byte
		expected bool
	}{
		{0x00, false}, // Broadcast - not valid slave
		{0x01, true},  // Min valid
		{0x7F, true},  // Mid range
		{0xF7, true},  // 247 - Max valid
		{0xF8, false}, // 248 - Reserved
		{0xFF, false}, // 255 - Reserved
	}

	for _, tt := range tests {
		result := IsValidUnitID(tt.unitID)
		if result != tt.expected {
			t.Errorf("IsValidUnitID(0x%02X) = %v, want %v", tt.unitID, result, tt.expected)
		}
	}
}

func TestBuildTCPResponseEcho(t *testing.T) {
	resp := &Response{
		TransactionID: 1,
		ProtocolID:    0,
		UnitID:        1,
		FunctionCode:  FCWriteSingleReg,
		Data:          []byte{0x00, 0x05, 0x12, 0x34}, // Address + Value
		IsEcho:        true,
	}

	result := BuildTCPResponse(resp)

	// Echo format: FC(1) + Data(4) = 5 bytes PDU
	// Total: MBAP(7) + PDU(5) = 12 bytes
	if len(result) != 12 {
		t.Errorf("Response length = %d, want 12", len(result))
	}
	if result[7] != FCWriteSingleReg {
		t.Errorf("Function code = %02X, want %02X", result[7], FCWriteSingleReg)
	}
	// No byte count in echo format
	if result[8] != 0x00 || result[9] != 0x05 {
		t.Errorf("Address in response = %02X%02X, want 0005", result[8], result[9])
	}
}

func TestBuildRTUResponseEcho(t *testing.T) {
	resp := &Response{
		UnitID:       1,
		FunctionCode: FCWriteSingleCoil,
		Data:         []byte{0x00, 0x0A, 0xFF, 0x00}, // Address + Value
		IsEcho:       true,
	}

	result := BuildRTUResponse(resp)

	// Echo format: UnitID(1) + FC(1) + Data(4) + CRC(2) = 8 bytes
	if len(result) != 8 {
		t.Errorf("Response length = %d, want 8", len(result))
	}
	if result[0] != 0x01 {
		t.Errorf("UnitID = %02X, want 01", result[0])
	}
	if result[1] != FCWriteSingleCoil {
		t.Errorf("Function code = %02X, want %02X", result[1], FCWriteSingleCoil)
	}
	if !ValidateCRC(result) {
		t.Error("Response has invalid CRC")
	}
}

func TestValidateQuantityWriteFunctions(t *testing.T) {
	tests := []struct {
		name         string
		functionCode byte
		quantity     uint16
		wantErr      error
	}{
		// FC05/FC06 always pass (quantity=1 implicit)
		{"FC05 qty 1", FCWriteSingleCoil, 1, nil},
		{"FC06 qty 1", FCWriteSingleReg, 1, nil},
		// FC0F: max 1968 coils
		{"FC0F qty 1", FCWriteMultipleCoils, 1, nil},
		{"FC0F qty 1968", FCWriteMultipleCoils, 1968, nil},
		{"FC0F qty 1969", FCWriteMultipleCoils, 1969, ErrIllegalDataValue},
		// FC10: max 123 registers
		{"FC10 qty 1", FCWriteMultipleRegs, 1, nil},
		{"FC10 qty 123", FCWriteMultipleRegs, 123, nil},
		{"FC10 qty 124", FCWriteMultipleRegs, 124, ErrIllegalDataValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateQuantity(tt.functionCode, tt.quantity)
			if err != tt.wantErr {
				t.Errorf("ValidateQuantity(0x%02X, %d) = %v, want %v",
					tt.functionCode, tt.quantity, err, tt.wantErr)
			}
		})
	}
}

// RTU parsing tests for write operations

func TestParseRTURequestWriteSingleCoil(t *testing.T) {
	// FC05: Write Single Coil - SlaveID(1) + FC(1) + Address(2) + Value(2) + CRC(2)
	pdu := []byte{
		0x01,       // Slave Address
		0x05,       // Function Code
		0x00, 0x0A, // Address (10)
		0xFF, 0x00, // Value (ON)
	}
	crc := CRC16Bytes(pdu)
	data := append(pdu, crc[0], crc[1])

	req, err := ParseRTURequest(data)
	if err != nil {
		t.Fatalf("ParseRTURequest failed: %v", err)
	}

	if req.FunctionCode != FCWriteSingleCoil {
		t.Errorf("FunctionCode = %02X, want %02X", req.FunctionCode, FCWriteSingleCoil)
	}
	if req.StartAddress != 10 {
		t.Errorf("StartAddress = %d, want 10", req.StartAddress)
	}
	if req.Quantity != 1 {
		t.Errorf("Quantity = %d, want 1", req.Quantity)
	}
	if len(req.WriteData) != 2 || req.WriteData[0] != 0xFF || req.WriteData[1] != 0x00 {
		t.Errorf("WriteData = %02X, want FF00", req.WriteData)
	}
}

func TestParseRTURequestWriteSingleReg(t *testing.T) {
	// FC06: Write Single Register
	pdu := []byte{
		0x01,       // Slave Address
		0x06,       // Function Code
		0x00, 0x05, // Address (5)
		0x12, 0x34, // Value
	}
	crc := CRC16Bytes(pdu)
	data := append(pdu, crc[0], crc[1])

	req, err := ParseRTURequest(data)
	if err != nil {
		t.Fatalf("ParseRTURequest failed: %v", err)
	}

	if req.FunctionCode != FCWriteSingleReg {
		t.Errorf("FunctionCode = %02X, want %02X", req.FunctionCode, FCWriteSingleReg)
	}
	if len(req.WriteData) != 2 || req.WriteData[0] != 0x12 || req.WriteData[1] != 0x34 {
		t.Errorf("WriteData = %02X, want 1234", req.WriteData)
	}
}

func TestParseRTURequestWriteMultipleRegs(t *testing.T) {
	// FC10: Write Multiple Registers
	pdu := []byte{
		0x01,       // Slave Address
		0x10,       // Function Code
		0x00, 0x00, // Start Address
		0x00, 0x02, // Quantity (2 registers)
		0x04,       // Byte Count
		0x00, 0x0A, // Register 1 value
		0x01, 0x02, // Register 2 value
	}
	crc := CRC16Bytes(pdu)
	data := append(pdu, crc[0], crc[1])

	req, err := ParseRTURequest(data)
	if err != nil {
		t.Fatalf("ParseRTURequest failed: %v", err)
	}

	if req.FunctionCode != FCWriteMultipleRegs {
		t.Errorf("FunctionCode = %02X, want %02X", req.FunctionCode, FCWriteMultipleRegs)
	}
	if req.Quantity != 2 {
		t.Errorf("Quantity = %d, want 2", req.Quantity)
	}
	if len(req.WriteData) != 4 {
		t.Errorf("WriteData length = %d, want 4", len(req.WriteData))
	}
}

func TestParseRTURequestWriteMultipleCoils(t *testing.T) {
	// FC0F: Write Multiple Coils
	pdu := []byte{
		0x01,       // Slave Address
		0x0F,       // Function Code
		0x00, 0x00, // Start Address
		0x00, 0x08, // Quantity (8 coils)
		0x01,       // Byte Count
		0xFF,       // Coil data (all ON)
	}
	crc := CRC16Bytes(pdu)
	data := append(pdu, crc[0], crc[1])

	req, err := ParseRTURequest(data)
	if err != nil {
		t.Fatalf("ParseRTURequest failed: %v", err)
	}

	if req.FunctionCode != FCWriteMultipleCoils {
		t.Errorf("FunctionCode = %02X, want %02X", req.FunctionCode, FCWriteMultipleCoils)
	}
	if req.Quantity != 8 {
		t.Errorf("Quantity = %d, want 8", req.Quantity)
	}
	if len(req.WriteData) != 1 || req.WriteData[0] != 0xFF {
		t.Errorf("WriteData = %02X, want FF", req.WriteData)
	}
}

func TestParseRTURequestWriteMultipleTooShort(t *testing.T) {
	// FC10 with insufficient data for header
	pdu := []byte{
		0x01,       // Slave Address
		0x10,       // Function Code
		0x00, 0x00, // Start Address
		0x00,       // Missing second byte of quantity
	}
	crc := CRC16Bytes(pdu)
	data := append(pdu, crc[0], crc[1])

	_, err := ParseRTURequest(data)
	if err != ErrInvalidFrame {
		t.Errorf("ParseRTURequest error = %v, want ErrInvalidFrame", err)
	}
}

func TestParseRTURequestWriteMultipleDataTooShort(t *testing.T) {
	// FC10 with byte count but insufficient data bytes
	pdu := []byte{
		0x01,       // Slave Address
		0x10,       // Function Code
		0x00, 0x00, // Start Address
		0x00, 0x02, // Quantity (2 registers)
		0x04,       // Byte Count (claims 4 bytes)
		0x00, 0x0A, // Only 2 bytes of data (should be 4)
	}
	crc := CRC16Bytes(pdu)
	data := append(pdu, crc[0], crc[1])

	_, err := ParseRTURequest(data)
	if err != ErrInvalidFrame {
		t.Errorf("ParseRTURequest error = %v, want ErrInvalidFrame", err)
	}
}

func TestParseRTURequestUnknownFunction(t *testing.T) {
	// Unknown function code (0x99)
	pdu := []byte{
		0x01,       // Slave Address
		0x99,       // Unknown Function Code
		0x00, 0x00, // Start Address
		0x00, 0x01, // Quantity
	}
	crc := CRC16Bytes(pdu)
	data := append(pdu, crc[0], crc[1])

	req, err := ParseRTURequest(data)
	if err != nil {
		t.Fatalf("ParseRTURequest failed: %v", err)
	}

	if req.FunctionCode != 0x99 {
		t.Errorf("FunctionCode = %02X, want 99", req.FunctionCode)
	}
	if req.Quantity != 1 {
		t.Errorf("Quantity = %d, want 1", req.Quantity)
	}
}

func TestParseTCPRequestWriteMultipleCoils(t *testing.T) {
	// FC0F: Write Multiple Coils
	data := []byte{
		0x00, 0x01, // Transaction ID
		0x00, 0x00, // Protocol ID
		0x00, 0x08, // Length
		0x01,       // Unit ID
		0x0F,       // Function Code
		0x00, 0x00, // Start Address
		0x00, 0x08, // Quantity (8 coils)
		0x01,       // Byte Count
		0xFF,       // Coil data
	}

	req, err := ParseTCPRequest(data)
	if err != nil {
		t.Fatalf("ParseTCPRequest failed: %v", err)
	}

	if req.FunctionCode != FCWriteMultipleCoils {
		t.Errorf("FunctionCode = %02X, want %02X", req.FunctionCode, FCWriteMultipleCoils)
	}
	if req.Quantity != 8 {
		t.Errorf("Quantity = %d, want 8", req.Quantity)
	}
}

func TestParseTCPRequestWriteMultipleTooShort(t *testing.T) {
	// FC10 without byte count
	data := []byte{
		0x00, 0x01, // Transaction ID
		0x00, 0x00, // Protocol ID
		0x00, 0x06, // Length
		0x01,       // Unit ID
		0x10,       // Function Code
		0x00, 0x00, // Start Address
		0x00, 0x02, // Quantity
	}

	_, err := ParseTCPRequest(data)
	if err != ErrInvalidFrame {
		t.Errorf("ParseTCPRequest error = %v, want ErrInvalidFrame", err)
	}
}

func TestParseTCPRequestWriteMultipleDataTooShort(t *testing.T) {
	// FC10 with byte count but insufficient data
	data := []byte{
		0x00, 0x01, // Transaction ID
		0x00, 0x00, // Protocol ID
		0x00, 0x09, // Length
		0x01,       // Unit ID
		0x10,       // Function Code
		0x00, 0x00, // Start Address
		0x00, 0x02, // Quantity (2 registers)
		0x04,       // Byte Count (claims 4 bytes)
		0x00, 0x0A, // Only 2 bytes
	}

	_, err := ParseTCPRequest(data)
	if err != ErrInvalidFrame {
		t.Errorf("ParseTCPRequest error = %v, want ErrInvalidFrame", err)
	}
}

func TestParseTCPRequestUnknownFunction(t *testing.T) {
	// Unknown function code
	data := []byte{
		0x00, 0x01, // Transaction ID
		0x00, 0x00, // Protocol ID
		0x00, 0x06, // Length
		0x01,       // Unit ID
		0x99,       // Unknown Function Code
		0x00, 0x00, // Start Address
		0x00, 0x01, // Quantity
	}

	req, err := ParseTCPRequest(data)
	if err != nil {
		t.Fatalf("ParseTCPRequest failed: %v", err)
	}

	if req.FunctionCode != 0x99 {
		t.Errorf("FunctionCode = %02X, want 99", req.FunctionCode)
	}
}
