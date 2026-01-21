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
