package model

import "testing"

func TestGetRegisterType(t *testing.T) {
	tests := []struct {
		addr     int
		expected RegisterType
	}{
		{1, Coil},
		{100, Coil},
		{9999, Coil},
		{10001, DiscreteInput},
		{15000, DiscreteInput},
		{19999, DiscreteInput},
		{30001, InputRegister},
		{35000, InputRegister},
		{39999, InputRegister},
		{40001, HoldingRegister},
		{45000, HoldingRegister},
		{49999, HoldingRegister},
		{50000, HoldingRegister},
		{54501, HoldingRegister},
		{105536, HoldingRegister},
	}

	for _, tt := range tests {
		result := GetRegisterType(tt.addr)
		if result != tt.expected {
			t.Errorf("GetRegisterType(%d) = %d, want %d", tt.addr, result, tt.expected)
		}
	}
}

func TestIsValidAddress(t *testing.T) {
	tests := []struct {
		addr     int
		expected bool
	}{
		{0, false},
		{1, true},
		{9999, true},
		{10000, false},
		{10001, true},
		{19999, true},
		{20000, false},
		{30001, true},
		{39999, true},
		{40000, false},
		{40001, true},
		{49999, true},
		{50000, true},
		{105536, true},
		{105537, false},
	}

	for _, tt := range tests {
		result := IsValidAddress(tt.addr)
		if result != tt.expected {
			t.Errorf("IsValidAddress(%d) = %v, want %v", tt.addr, result, tt.expected)
		}
	}
}

func TestGetAddressOffset(t *testing.T) {
	tests := []struct {
		addr     int
		expected uint16
	}{
		{1, 0},
		{100, 99},
		{10001, 0},
		{10100, 99},
		{30001, 0},
		{30100, 99},
		{40001, 0},
		{40100, 99},
		{54501, 14500},
	}

	for _, tt := range tests {
		result := GetAddressOffset(tt.addr)
		if result != tt.expected {
			t.Errorf("GetAddressOffset(%d) = %d, want %d", tt.addr, result, tt.expected)
		}
	}
}

func TestRegisterFunctionCode(t *testing.T) {
	tests := []struct {
		regType  RegisterType
		expected byte
	}{
		{Coil, 0x01},
		{DiscreteInput, 0x02},
		{HoldingRegister, 0x03},
		{InputRegister, 0x04},
	}

	for _, tt := range tests {
		result := tt.regType.FunctionCode()
		if result != tt.expected {
			t.Errorf("FunctionCode() for %d = %02x, want %02x", tt.regType, result, tt.expected)
		}
	}
}

func TestRegisterCount(t *testing.T) {
	tests := []struct {
		reg      Register
		expected int
	}{
		// Holding register: 4 hex = 1 register
		{Register{StartAddr: 40001, HexData: "1234"}, 1},
		{Register{StartAddr: 40001, HexData: "12345678"}, 2},
		// Coil: 2 hex = 8 bits
		{Register{StartAddr: 1, HexData: "FF"}, 8},
		{Register{StartAddr: 1, HexData: "FFFF"}, 16},
	}

	for _, tt := range tests {
		result := tt.reg.RegisterCount()
		if result != tt.expected {
			t.Errorf("RegisterCount() for addr=%d hex=%s = %d, want %d",
				tt.reg.StartAddr, tt.reg.HexData, result, tt.expected)
		}
	}
}

func TestRegisterEndAddr(t *testing.T) {
	tests := []struct {
		reg      Register
		expected int
	}{
		{Register{StartAddr: 40001, HexData: "1234"}, 40001},
		{Register{StartAddr: 40001, HexData: "12345678"}, 40002},
		{Register{StartAddr: 1, HexData: "FF"}, 8},
		{Register{StartAddr: 1, HexData: "FFFF"}, 16},
	}

	for _, tt := range tests {
		result := tt.reg.EndAddr()
		if result != tt.expected {
			t.Errorf("EndAddr() for addr=%d hex=%s = %d, want %d",
				tt.reg.StartAddr, tt.reg.HexData, result, tt.expected)
		}
	}
}

func TestRegisterOverlaps(t *testing.T) {
	tests := []struct {
		reg1     Register
		reg2     Register
		expected bool
	}{
		// Same address - overlaps
		{
			Register{StartAddr: 40001, HexData: "1234"},
			Register{StartAddr: 40001, HexData: "5678"},
			true,
		},
		// Adjacent - no overlap
		{
			Register{StartAddr: 40001, HexData: "1234"},
			Register{StartAddr: 40002, HexData: "5678"},
			false,
		},
		// Overlapping range
		{
			Register{StartAddr: 40001, HexData: "12345678"}, // 40001-40002
			Register{StartAddr: 40002, HexData: "ABCD"},     // 40002
			true,
		},
		// Different register types - no overlap
		{
			Register{StartAddr: 40001, HexData: "1234"}, // Holding
			Register{StartAddr: 30001, HexData: "5678"}, // Input
			false,
		},
	}

	for i, tt := range tests {
		result := tt.reg1.Overlaps(&tt.reg2)
		if result != tt.expected {
			t.Errorf("Test %d: Overlaps() = %v, want %v", i, result, tt.expected)
		}
	}
}

// Additional tests for edge cases

func TestFunctionCodeDefault(t *testing.T) {
	// Test an invalid register type to cover default case
	invalidType := RegisterType(99)
	result := invalidType.FunctionCode()
	if result != 0x00 {
		t.Errorf("FunctionCode for invalid type = %02x, want 0x00", result)
	}
}

func TestGetRegisterTypeDefault(t *testing.T) {
	// Test addresses outside valid ranges
	tests := []struct {
		addr     int
		expected RegisterType
	}{
		{0, Coil},      // Below valid range, default fallback
		{10000, Coil},  // Gap between Coil and DiscreteInput
		{20000, Coil},  // Gap between DiscreteInput and InputRegister
		{25000, Coil},  // Between gaps
		{105537, Coil}, // Above all valid ranges
		{-1, Coil},     // Negative address
	}

	for _, tt := range tests {
		result := GetRegisterType(tt.addr)
		if result != tt.expected {
			t.Errorf("GetRegisterType(%d) = %d, want %d", tt.addr, result, tt.expected)
		}
	}
}

func TestGetAddressOffsetDefault(t *testing.T) {
	// Test addresses outside valid ranges - should return 0
	tests := []struct {
		addr     int
		expected uint16
	}{
		{0, 0},     // Below valid range
		{10000, 0}, // Gap
		{-1, 0},    // Negative
		{105537, 0}, // Above range
	}

	for _, tt := range tests {
		result := GetAddressOffset(tt.addr)
		if result != tt.expected {
			t.Errorf("GetAddressOffset(%d) = %d, want %d", tt.addr, result, tt.expected)
		}
	}
}

func TestRegisterCountDefault(t *testing.T) {
	// Test with an address outside valid ranges - hits default case
	reg := Register{StartAddr: 0, HexData: "1234"}
	count := reg.RegisterCount()
	// Address 0 falls to default Coil (via GetRegisterType), but 0 is not in range so uses Coil logic
	// For coil: len("1234")/2 * 8 = 16
	if count != 16 {
		t.Errorf("RegisterCount for addr=0 = %d, want 16", count)
	}
}

func TestEndAddrZeroCount(t *testing.T) {
	// Test with empty hex data (count = 0)
	reg := Register{StartAddr: 40001, HexData: ""}
	end := reg.EndAddr()
	if end != 40001 {
		t.Errorf("EndAddr for empty hex = %d, want 40001", end)
	}
}

func TestRegisterCountInputRegister(t *testing.T) {
	// Test input register
	reg := Register{StartAddr: 30001, HexData: "12345678"}
	count := reg.RegisterCount()
	if count != 2 {
		t.Errorf("RegisterCount for input register = %d, want 2", count)
	}
}

func TestRegisterCountDiscreteInput(t *testing.T) {
	// Test discrete input
	reg := Register{StartAddr: 10001, HexData: "FF"}
	count := reg.RegisterCount()
	if count != 8 {
		t.Errorf("RegisterCount for discrete input = %d, want 8", count)
	}
}
