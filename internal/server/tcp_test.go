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

func TestRTUProtocol(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)
	defer tcp.StopAll()

	// Setup test data
	conn := &model.Connection{
		ID:           "rtuconnectiontestrtuconnectionte",
		Name:         "Test",
		Port:         19516,
		ProtocolType: model.ModbusRtuOverTcp,
	}
	s.CreateConnection(conn)

	slave := &model.Slave{
		ID:        "rtuslavirtuslavirtuslavirtusla01",
		ConnID:    conn.ID,
		Name:      "Slave",
		SlaveAddr: 1,
	}
	s.CreateSlave(slave)

	reg := &model.Register{
		ID:        "rturegirturegirturegirturegiir01",
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
	client, err := net.DialTimeout("tcp", "127.0.0.1:19516", time.Second)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Send Modbus RTU read holding registers request with CRC
	// SlaveID: 1, FC: 3, Addr: 0, Qty: 2
	request := []byte{
		0x01,       // Slave ID
		0x03,       // Function Code (Read Holding Registers)
		0x00, 0x00, // Start Address (0 = 40001)
		0x00, 0x02, // Quantity (2 registers)
	}
	// Calculate CRC
	crc := protocol.CRC16Bytes(request)
	request = append(request, crc[0], crc[1])

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

	if n < 7 {
		t.Fatalf("Response too short: %d bytes", n)
	}

	// Verify response: SlaveID(1) + FC(1) + ByteCount(1) + Data(4) + CRC(2) = 9 bytes
	if response[0] != 0x01 {
		t.Errorf("Slave ID = %02X, want 01", response[0])
	}
	if response[1] != 0x03 {
		t.Errorf("Function code = %02X, want 03", response[1])
	}
	if response[2] != 0x04 {
		t.Errorf("Byte count = %d, want 4", response[2])
	}
}

func TestConnectionCloseOnDone(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn := &model.Connection{
		ID:           "closeondoneconncloseondoneconnab",
		Name:         "Test",
		Port:         19517,
		ProtocolType: model.ModbusTcp,
	}
	s.CreateConnection(conn)

	// Start listener
	err := tcp.StartListener(conn)
	if err != nil {
		t.Fatalf("StartListener failed: %v", err)
	}

	// Give listener time to start
	time.Sleep(100 * time.Millisecond)

	// Connect as client
	client, err := net.DialTimeout("tcp", "127.0.0.1:19517", time.Second)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Stop all to close the listener
	tcp.StopAll()

	// Verify listener is removed from internal map
	tcp.mu.RLock()
	_, exists := tcp.listeners[conn.Port]
	tcp.mu.RUnlock()
	if exists {
		t.Error("Listener should be removed after StopAll")
	}

	// Verify new connections are rejected
	time.Sleep(50 * time.Millisecond)
	_, err = net.DialTimeout("tcp", "127.0.0.1:19517", 200*time.Millisecond)
	if err == nil {
		t.Error("Expected connection to be refused after StopAll")
	}
}

func TestInvalidRTURequest(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)
	defer tcp.StopAll()

	conn := &model.Connection{
		ID:           "invalidrtuconnsinvalidrtuconns01",
		Name:         "Test",
		Port:         19518,
		ProtocolType: model.ModbusRtuOverTcp,
	}
	s.CreateConnection(conn)

	// Start listener
	err := tcp.StartListener(conn)
	if err != nil {
		t.Fatalf("StartListener failed: %v", err)
	}

	// Give listener time to start
	time.Sleep(100 * time.Millisecond)

	// Connect as client
	client, err := net.DialTimeout("tcp", "127.0.0.1:19518", time.Second)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Send invalid RTU request (bad CRC)
	request := []byte{
		0x01,       // Slave ID
		0x03,       // Function Code
		0x00, 0x00, // Start Address
		0x00, 0x02, // Quantity
		0xFF, 0xFF, // Invalid CRC
	}

	client.SetWriteDeadline(time.Now().Add(time.Second))
	_, err = client.Write(request)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Server should NOT respond to invalid CRC - expect timeout
	client.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	response := make([]byte, 256)
	n, err := client.Read(response)

	// Fail if any data was received, regardless of error
	if n > 0 {
		t.Errorf("Server responded to invalid CRC request with %d bytes: %02X", n, response[:n])
	}
}

func TestTCPRequestTooShort(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)
	defer tcp.StopAll()

	conn := &model.Connection{
		ID:           "shortrequestshortrequestshortreq",
		Name:         "Test",
		Port:         19519,
		ProtocolType: model.ModbusTcp,
	}
	s.CreateConnection(conn)

	err := tcp.StartListener(conn)
	if err != nil {
		t.Fatalf("StartListener failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	client, err := net.DialTimeout("tcp", "127.0.0.1:19519", time.Second)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Send too-short request (less than 12 bytes for TCP)
	client.SetWriteDeadline(time.Now().Add(time.Second))
	_, err = client.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x02})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Server should NOT respond to invalid frame
	client.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	response := make([]byte, 256)
	n, err := client.Read(response)
	// Fail if any data was received, regardless of error
	if n > 0 {
		t.Errorf("Server responded to too-short request with %d bytes: %02X", n, response[:n])
	}
	_ = err // err is expected (timeout or EOF)

	// Verify server still accepts new connections (didn't crash)
	client2, err := net.DialTimeout("tcp", "127.0.0.1:19519", time.Second)
	if err != nil {
		t.Errorf("Server stopped accepting connections after invalid request: %v", err)
	} else {
		client2.Close()
	}
}

func TestClientDisconnect(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)
	defer tcp.StopAll()

	conn := &model.Connection{
		ID:           "clientdisconnclientdisconnclienx",
		Name:         "Test",
		Port:         19520,
		ProtocolType: model.ModbusTcp,
	}
	s.CreateConnection(conn)

	err := tcp.StartListener(conn)
	if err != nil {
		t.Fatalf("StartListener failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Connect and immediately disconnect multiple times
	for i := 0; i < 3; i++ {
		client, err := net.DialTimeout("tcp", "127.0.0.1:19520", time.Second)
		if err != nil {
			t.Fatalf("Connection %d failed: %v", i, err)
		}
		client.Close()
	}

	// Give server time to handle disconnects
	time.Sleep(100 * time.Millisecond)

	// Verify server still works after multiple disconnects
	client, err := net.DialTimeout("tcp", "127.0.0.1:19520", time.Second)
	if err != nil {
		t.Fatalf("Server stopped accepting connections after client disconnects: %v", err)
	}
	defer client.Close()

	// Verify listener is still registered
	tcp.mu.RLock()
	_, exists := tcp.listeners[conn.Port]
	tcp.mu.RUnlock()
	if !exists {
		t.Error("Listener should still be registered after client disconnects")
	}
}

// Tests for Modbus specification validation (added per V1.1b3 compliance)

func TestProcessRequestQuantityZero(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn := &model.Connection{ID: "qtyzerotestconnqtyzerotestconn01", Name: "Test", Port: 19521}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "qtyzeroslavetestqtyzeroslavetest", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	h := handler.NewModbusHandler(s, conn.ID)

	req := &protocol.Request{
		UnitID:       1,
		FunctionCode: 0x03,
		StartAddress: 0,
		Quantity:     0, // Invalid: zero quantity
	}

	resp := tcp.processRequest(req, h)

	if !resp.IsError {
		t.Error("Expected error response for zero quantity")
	}
	if resp.ErrorCode != protocol.ExceptionIllegalDataValue {
		t.Errorf("ErrorCode = %02X, want %02X", resp.ErrorCode, protocol.ExceptionIllegalDataValue)
	}
}

func TestProcessRequestQuantityExceedsMax(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn := &model.Connection{ID: "qtymaxtestconnqtymaxtestconnqtym", Name: "Test", Port: 19522}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "qtymaxslaveqtymaxslaveqtymaxslav", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	h := handler.NewModbusHandler(s, conn.ID)

	// Test FC03 with quantity > 125 (max for registers)
	req := &protocol.Request{
		UnitID:       1,
		FunctionCode: 0x03,
		StartAddress: 0,
		Quantity:     126, // Invalid: exceeds max 125 for registers
	}

	resp := tcp.processRequest(req, h)

	if !resp.IsError {
		t.Error("Expected error response for quantity > 125")
	}
	if resp.ErrorCode != protocol.ExceptionIllegalDataValue {
		t.Errorf("ErrorCode = %02X, want %02X", resp.ErrorCode, protocol.ExceptionIllegalDataValue)
	}
}

func TestProcessRequestAddressOverflow(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn := &model.Connection{ID: "addroverflowtestaddroverflowtestx", Name: "Test", Port: 19523}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "addroverflowtestslaveovertestslav", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	h := handler.NewModbusHandler(s, conn.ID)

	// Test address exceeds PDU range: 65535 + 2 overflows 16-bit
	req := &protocol.Request{
		UnitID:       1,
		FunctionCode: 0x03,
		StartAddress: 65535, // Max PDU address
		Quantity:     2,     // Overflow
	}

	resp := tcp.processRequest(req, h)

	if !resp.IsError {
		t.Error("Expected error response for address overflow")
	}
	if resp.ErrorCode != protocol.ExceptionIllegalDataAddr {
		t.Errorf("ErrorCode = %02X, want %02X", resp.ErrorCode, protocol.ExceptionIllegalDataAddr)
	}
}

func TestBroadcastNoResponse(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)
	defer tcp.StopAll()

	conn := &model.Connection{
		ID:           "broadcasttestconnbroadcasttestco",
		Name:         "Test",
		Port:         19524,
		ProtocolType: model.ModbusTcp,
	}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "broadcastslavtestbroadcastslavte", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "broadcastregtestbroadcastregtestx", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"}
	s.CreateRegister(reg)

	err := tcp.StartListener(conn)
	if err != nil {
		t.Fatalf("StartListener failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	client, err := net.DialTimeout("tcp", "127.0.0.1:19524", time.Second)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Send broadcast request (UnitID = 0)
	request := []byte{
		0x00, 0x01, // Transaction ID
		0x00, 0x00, // Protocol ID
		0x00, 0x06, // Length
		0x00,       // Unit ID = 0 (BROADCAST)
		0x03,       // Function Code
		0x00, 0x00, // Start Address
		0x00, 0x01, // Quantity
	}

	client.SetWriteDeadline(time.Now().Add(time.Second))
	_, err = client.Write(request)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Server should NOT respond to broadcast - expect timeout
	client.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	response := make([]byte, 256)
	n, err := client.Read(response)

	// Fail if any data was received, regardless of error
	if n > 0 {
		t.Errorf("Server responded to broadcast request with %d bytes: %02X", n, response[:n])
	}
	_ = err // err is expected (timeout or EOF)
}

func TestProcessRequestWriteSingleReg(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn := &model.Connection{ID: "writesingleregtestwritesinglereg", Name: "Test", Port: 19525}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "writesingleregslavesingleregslave", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "writesingleregrwritesingleregreg", SlaveID: slave.ID, StartAddr: 40001, HexData: "0000"}
	s.CreateRegister(reg)

	h := handler.NewModbusHandler(s, conn.ID)

	req := &protocol.Request{
		UnitID:       1,
		FunctionCode: protocol.FCWriteSingleReg,
		StartAddress: 0,
		Quantity:     1,
		WriteData:    []byte{0x12, 0x34},
	}

	resp := tcp.processRequest(req, h)

	if resp.IsError {
		t.Errorf("Unexpected error: %02X", resp.ErrorCode)
	}
	if !resp.IsEcho {
		t.Error("Expected echo response for write single register")
	}
	if len(resp.Data) != 4 {
		t.Errorf("Response data length = %d, want 4", len(resp.Data))
	}
}

func TestProcessRequestWriteMultipleRegs(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn := &model.Connection{ID: "writemultiregtestwritemultiregte", Name: "Test", Port: 19526}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "writemultiregslavemultiregslavex", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "writemultiregregmultiregregmulit", SlaveID: slave.ID, StartAddr: 40001, HexData: "00000000"}
	s.CreateRegister(reg)

	h := handler.NewModbusHandler(s, conn.ID)

	req := &protocol.Request{
		UnitID:       1,
		FunctionCode: protocol.FCWriteMultipleRegs,
		StartAddress: 0,
		Quantity:     2,
		WriteData:    []byte{0x00, 0x0A, 0x01, 0x02},
	}

	resp := tcp.processRequest(req, h)

	if resp.IsError {
		t.Errorf("Unexpected error: %02X", resp.ErrorCode)
	}
	if !resp.IsEcho {
		t.Error("Expected echo response for write multiple registers")
	}
}

func TestProcessRequestReservedUnitID(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)
	defer tcp.StopAll()

	conn := &model.Connection{
		ID:           "reservedunittestconnreservedunit",
		Name:         "Test",
		Port:         19527,
		ProtocolType: model.ModbusTcp,
	}
	s.CreateConnection(conn)

	err := tcp.StartListener(conn)
	if err != nil {
		t.Fatalf("StartListener failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	client, err := net.DialTimeout("tcp", "127.0.0.1:19527", time.Second)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Send request with reserved UnitID (248)
	request := []byte{
		0x00, 0x01, // Transaction ID
		0x00, 0x00, // Protocol ID
		0x00, 0x06, // Length
		0xF8,       // Unit ID = 248 (RESERVED)
		0x03,       // Function Code
		0x00, 0x00, // Start Address
		0x00, 0x01, // Quantity
	}

	client.SetWriteDeadline(time.Now().Add(time.Second))
	_, err = client.Write(request)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Should receive error response (not timeout)
	client.SetReadDeadline(time.Now().Add(time.Second))
	response := make([]byte, 256)
	n, err := client.Read(response)

	if err != nil {
		t.Fatalf("Expected error response, got read error: %v", err)
	}
	if n < 9 {
		t.Fatalf("Response too short: %d bytes", n)
	}
	// Check for exception response (function code | 0x80)
	if response[7] != 0x83 {
		t.Errorf("Function code = %02X, want 83 (exception)", response[7])
	}
	if response[8] != protocol.ExceptionIllegalDataAddr {
		t.Errorf("Exception code = %02X, want %02X", response[8], protocol.ExceptionIllegalDataAddr)
	}
}

func TestProcessRequestWriteSingleCoil(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn := &model.Connection{ID: "writecoiltestconnwritecoiltestco", Name: "Test", Port: 19528}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "writecoilslavewritecoilslavewrit", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "writecoilregwritecoilregwritecoi", SlaveID: slave.ID, StartAddr: 1, HexData: "00"}
	s.CreateRegister(reg)

	h := handler.NewModbusHandler(s, conn.ID)

	req := &protocol.Request{
		UnitID:       1,
		FunctionCode: protocol.FCWriteSingleCoil,
		StartAddress: 0,
		Quantity:     1,
		WriteData:    []byte{0xFF, 0x00}, // ON
	}

	resp := tcp.processRequest(req, h)

	if resp.IsError {
		t.Errorf("Unexpected error: %02X", resp.ErrorCode)
	}
	if !resp.IsEcho {
		t.Error("Expected echo response for write single coil")
	}
	if len(resp.Data) != 4 {
		t.Errorf("Response data length = %d, want 4", len(resp.Data))
	}
}

func TestProcessRequestWriteMultipleCoils(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn := &model.Connection{ID: "writemulticoiltestwritemulticoil", Name: "Test", Port: 19529}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "writemulticoilslavemulticoilslav", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "writemulticoilregmulticoilregmul", SlaveID: slave.ID, StartAddr: 1, HexData: "00"}
	s.CreateRegister(reg)

	h := handler.NewModbusHandler(s, conn.ID)

	req := &protocol.Request{
		UnitID:       1,
		FunctionCode: protocol.FCWriteMultipleCoils,
		StartAddress: 0,
		Quantity:     8,
		WriteData:    []byte{0xFF}, // all ON
	}

	resp := tcp.processRequest(req, h)

	if resp.IsError {
		t.Errorf("Unexpected error: %02X", resp.ErrorCode)
	}
	if !resp.IsEcho {
		t.Error("Expected echo response for write multiple coils")
	}
	if len(resp.Data) != 4 {
		t.Errorf("Response data length = %d, want 4", len(resp.Data))
	}
}

func TestProcessRequestWriteWithError(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn := &model.Connection{ID: "writeerrtestconnwriteerrtestconn", Name: "Test", Port: 19530}
	s.CreateConnection(conn)

	h := handler.NewModbusHandler(s, conn.ID)

	// Write to non-existent slave should return error
	req := &protocol.Request{
		UnitID:       99,
		FunctionCode: protocol.FCWriteSingleReg,
		StartAddress: 0,
		Quantity:     1,
		WriteData:    []byte{0x00, 0x01},
	}

	resp := tcp.processRequest(req, h)

	if !resp.IsError {
		t.Error("Expected error for invalid slave")
	}
	if resp.ErrorCode != protocol.ExceptionIllegalDataAddr {
		t.Errorf("ErrorCode = %02X, want %02X", resp.ErrorCode, protocol.ExceptionIllegalDataAddr)
	}
}

func TestProcessRequestUnknownFunctionCode(t *testing.T) {
	s := store.New()
	tcp := NewTCPServer(s)

	conn := &model.Connection{ID: "unknownfctestconnunknownfctestco", Name: "Test", Port: 19531}
	s.CreateConnection(conn)

	h := handler.NewModbusHandler(s, conn.ID)

	req := &protocol.Request{
		UnitID:       1,
		FunctionCode: 0x99, // Unknown function code
		StartAddress: 0,
		Quantity:     1,
	}

	resp := tcp.processRequest(req, h)

	if !resp.IsError {
		t.Error("Expected error for unknown function code")
	}
	if resp.ErrorCode != protocol.ExceptionIllegalFunction {
		t.Errorf("ErrorCode = %02X, want %02X", resp.ErrorCode, protocol.ExceptionIllegalFunction)
	}
}
