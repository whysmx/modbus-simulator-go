package server

import (
	"net"
	"testing"
	"time"

	"github.com/whysmx/modbus-simulator-go/internal/handler"
	"github.com/whysmx/modbus-simulator-go/internal/model"
	"github.com/whysmx/modbus-simulator-go/internal/protocol"
	"github.com/whysmx/modbus-simulator-go/internal/store"
)

func TestNewTCPServer(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	if tcp == nil {
		t.Fatal("NewTCPServer returned nil")
	}
	if tcp.store != s {
		t.Error("store not set correctly")
	}
	if tcp.listeners == nil {
		t.Error("listeners map not initialized")
	}
}

func TestStartListener(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn := &model.Connection{
		ID:           "testconnidtestconnidtestconnidtes",
		Name:         "Test",
		Port:         19502, // Use high port to avoid conflicts
		ProtocolType: model.ModbusTcp,
	}

	err := tcp.StartListener(conn)
	if err != nil {
		t.Fatalf("StartListener failed: %v", err)
	}

	// Cleanup
	defer tcp.StopAll()

	// Verify listener is running
	tcp.mu.RLock()
	_, exists := tcp.listeners[conn.Port]
	tcp.mu.RUnlock()

	if !exists {
		t.Error("Listener not registered")
	}
}

func TestStartListenerPortInUse(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)
	defer tcp.StopAll()

	conn1 := &model.Connection{
		ID:   "conn1conn1conn1conn1conn1conn1co",
		Name: "Conn1",
		Port: 19503,
	}
	conn2 := &model.Connection{
		ID:   "conn2conn2conn2conn2conn2conn2co",
		Name: "Conn2",
		Port: 19503, // Same port
	}

	tcp.StartListener(conn1)
	err := tcp.StartListener(conn2)

	if err == nil {
		t.Error("Expected error for duplicate port")
	}
}

func TestStopListener(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn := &model.Connection{
		ID:   "stoplistenerstoplistenerstoplist",
		Name: "Test",
		Port: 19504,
	}

	tcp.StartListener(conn)
	err := tcp.StopListener(conn.Port)

	if err != nil {
		t.Fatalf("StopListener failed: %v", err)
	}

	tcp.mu.RLock()
	_, exists := tcp.listeners[conn.Port]
	tcp.mu.RUnlock()

	if exists {
		t.Error("Listener still registered after stop")
	}
}

func TestStopListenerNotExists(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	err := tcp.StopListener(99999)
	if err == nil {
		t.Error("Expected error for non-existent listener")
	}
}

func TestStopAll(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn1 := &model.Connection{ID: "stopall1stopall1stopall1stopall1", Name: "C1", Port: 19505}
	conn2 := &model.Connection{ID: "stopall2stopall2stopall2stopall2", Name: "C2", Port: 19506}

	tcp.StartListener(conn1)
	tcp.StartListener(conn2)

	tcp.StopAll()

	tcp.mu.RLock()
	count := len(tcp.listeners)
	tcp.mu.RUnlock()

	if count != 0 {
		t.Errorf("Expected 0 listeners after StopAll, got %d", count)
	}
}

func TestUpdateListener(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)
	defer tcp.StopAll()

	conn := &model.Connection{
		ID:           "updatelistenerupdatelistenerupda",
		Name:         "Test",
		Port:         19507,
		ProtocolType: model.ModbusRtuOverTcp,
	}

	tcp.StartListener(conn)

	// Update to new port
	newConn := &model.Connection{
		ID:           "updatelistenerupdatelistenerupda",
		Name:         "Test",
		Port:         19508,
		ProtocolType: model.ModbusTcp,
	}

	err := tcp.UpdateListener(conn.Port, newConn)
	if err != nil {
		t.Fatalf("UpdateListener failed: %v", err)
	}

	tcp.mu.RLock()
	_, oldExists := tcp.listeners[conn.Port]
	_, newExists := tcp.listeners[newConn.Port]
	tcp.mu.RUnlock()

	if oldExists {
		t.Error("Old listener still exists")
	}
	if !newExists {
		t.Error("New listener not created")
	}
}

func TestTCPServerConnection(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)
	defer tcp.StopAll()

	// Setup test data
	conn := &model.Connection{
		ID:           "tcpconnectiontesttcpconnectionte",
		Name:         "Test",
		Port:         19509,
		ProtocolType: model.ModbusTcp,
	}
	s.CreateConnection(conn)

	slave := &model.Slave{
		ID:        "tcpslavetcpslavetcpslavetcpslav",
		ConnID:    conn.ID,
		Name:      "Slave",
		SlaveAddr: 1,
	}
	s.CreateSlave(slave)

	reg := &model.Register{
		ID:        "tcpregtcpregtcpregtcpregtcpregt",
		SlaveID:   slave.ID,
		StartAddr: 40001,
		HexData:   "00640065", // Values 100, 101
	}
	s.CreateRegister(reg)

	// Start listener
	err := tcp.StartListener(conn)
	if err != nil {
		t.Fatalf("StartListener failed: %v", err)
	}

	// Give listener time to start
	time.Sleep(100 * time.Millisecond)

	// Connect as client
	client, err := net.DialTimeout("tcp", "127.0.0.1:19509", time.Second)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Send Modbus TCP read holding registers request
	// Transaction ID: 1, Protocol: 0, Length: 6, Unit: 1, FC: 3, Addr: 0, Qty: 2
	request := []byte{
		0x00, 0x01, // Transaction ID
		0x00, 0x00, // Protocol ID
		0x00, 0x06, // Length
		0x01,       // Unit ID
		0x03,       // Function Code (Read Holding Registers)
		0x00, 0x00, // Start Address (0 = 40001)
		0x00, 0x02, // Quantity (2 registers)
	}

	client.SetWriteDeadline(time.Now().Add(time.Second))
	_, err = client.Write(request)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read response
	client.SetReadDeadline(time.Now().Add(time.Second))
	response := make([]byte, 256)
	n, err := client.Read(response)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if n < 9 {
		t.Fatalf("Response too short: %d bytes", n)
	}

	// Verify response
	// MBAP header (7) + FC (1) + Byte count (1) + Data (4) = 13 bytes
	if response[7] != 0x03 {
		t.Errorf("Function code = %02X, want 03", response[7])
	}
	if response[8] != 0x04 {
		t.Errorf("Byte count = %d, want 4", response[8])
	}
	// Data should be 00 64 00 65 (100, 101)
	if response[9] != 0x00 || response[10] != 0x64 {
		t.Errorf("First register = %02X%02X, want 0064", response[9], response[10])
	}
}

func TestProcessRequest(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn := &model.Connection{
		ID:   "processreqprocessreqprocessreqpr",
		Name: "Test",
		Port: 19510,
	}
	s.CreateConnection(conn)

	slave := &model.Slave{
		ID:        "processslaveprocessslaveprocesss",
		ConnID:    conn.ID,
		Name:      "Slave",
		SlaveAddr: 1,
	}
	s.CreateSlave(slave)

	h := handler.NewModbusHandler(s, conn.ID)

	// Test illegal function code
	req := &protocol.Request{
		TransactionID: 1,
		UnitID:        1,
		FunctionCode:  0x99, // Invalid
		StartAddress:  0,
		Quantity:      1,
	}

	resp := tcp.processRequest(req, h)

	if !resp.IsError {
		t.Error("Expected error response")
	}
	if resp.ErrorCode != protocol.ExceptionIllegalFunction {
		t.Errorf("ErrorCode = %02X, want %02X", resp.ErrorCode, protocol.ExceptionIllegalFunction)
	}
}

func TestProcessRequestFC01(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn := &model.Connection{ID: "fc01connfc01connfc01connfc01conn", Name: "Test", Port: 19511}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "fc01slavefc01slavefc01slavefc01s", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "fc01regfc01regfc01regfc01regfc01", SlaveID: slave.ID, StartAddr: 1, HexData: "FF"}
	s.CreateRegister(reg)

	h := handler.NewModbusHandler(s, conn.ID)

	req := &protocol.Request{
		UnitID:       1,
		FunctionCode: 0x01, // Read Coils
		StartAddress: 0,
		Quantity:     8,
	}

	resp := tcp.processRequest(req, h)

	if resp.IsError {
		t.Errorf("Unexpected error: %02X", resp.ErrorCode)
	}
	if resp.FunctionCode != 0x01 {
		t.Errorf("FunctionCode = %02X, want 01", resp.FunctionCode)
	}
}

func TestProcessRequestFC02(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn := &model.Connection{ID: "fc02connfc02connfc02connfc02conn", Name: "Test", Port: 19512}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "fc02slavefc02slavefc02slavefc02s", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "fc02regfc02regfc02regfc02regfc02", SlaveID: slave.ID, StartAddr: 10001, HexData: "AA"}
	s.CreateRegister(reg)

	h := handler.NewModbusHandler(s, conn.ID)

	req := &protocol.Request{
		UnitID:       1,
		FunctionCode: 0x02, // Read Discrete Inputs
		StartAddress: 0,
		Quantity:     8,
	}

	resp := tcp.processRequest(req, h)

	if resp.IsError {
		t.Errorf("Unexpected error: %02X", resp.ErrorCode)
	}
	if resp.FunctionCode != 0x02 {
		t.Errorf("FunctionCode = %02X, want 02", resp.FunctionCode)
	}
}

func TestProcessRequestFC03(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn := &model.Connection{ID: "fc03connfc03connfc03connfc03conn", Name: "Test", Port: 19513}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "fc03slavefc03slavefc03slavefc03s", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "fc03regfc03regfc03regfc03regfc03", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"}
	s.CreateRegister(reg)

	h := handler.NewModbusHandler(s, conn.ID)

	req := &protocol.Request{
		UnitID:       1,
		FunctionCode: 0x03, // Read Holding Registers
		StartAddress: 0,
		Quantity:     1,
	}

	resp := tcp.processRequest(req, h)

	if resp.IsError {
		t.Errorf("Unexpected error: %02X", resp.ErrorCode)
	}
	if len(resp.Data) != 2 {
		t.Errorf("Data length = %d, want 2", len(resp.Data))
	}
}

func TestProcessRequestFC04(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn := &model.Connection{ID: "fc04connfc04connfc04connfc04conn", Name: "Test", Port: 19514}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "fc04slavefc04slavefc04slavefc04s", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "fc04regfc04regfc04regfc04regfc04", SlaveID: slave.ID, StartAddr: 30001, HexData: "5678"}
	s.CreateRegister(reg)

	h := handler.NewModbusHandler(s, conn.ID)

	req := &protocol.Request{
		UnitID:       1,
		FunctionCode: 0x04, // Read Input Registers
		StartAddress: 0,
		Quantity:     1,
	}

	resp := tcp.processRequest(req, h)

	if resp.IsError {
		t.Errorf("Unexpected error: %02X", resp.ErrorCode)
	}
	if len(resp.Data) != 2 {
		t.Errorf("Data length = %d, want 2", len(resp.Data))
	}
}

func TestProcessRequestSlaveNotFound(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn := &model.Connection{ID: "notslavefoundnotslavefoundnotsla", Name: "Test", Port: 19515}
	s.CreateConnection(conn)
	// No slave created

	h := handler.NewModbusHandler(s, conn.ID)

	req := &protocol.Request{
		UnitID:       99, // Non-existent slave
		FunctionCode: 0x03,
		StartAddress: 0,
		Quantity:     1,
	}

	resp := tcp.processRequest(req, h)

	if !resp.IsError {
		t.Error("Expected error response for non-existent slave")
	}
	if resp.ErrorCode != protocol.ExceptionIllegalDataAddr {
		t.Errorf("ErrorCode = %02X, want %02X", resp.ErrorCode, protocol.ExceptionIllegalDataAddr)
	}
}
