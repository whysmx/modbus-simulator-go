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
		{50000, false},
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
