package integration

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"net"
	"testing"
	"time"

	"github.com/whysmx/modbus-simulator-go/internal/handler"
	"github.com/whysmx/modbus-simulator-go/internal/model"
	"github.com/whysmx/modbus-simulator-go/internal/protocol"
	"github.com/whysmx/modbus-simulator-go/internal/store"
)

// generateUUID generates a random UUID for testing
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// TestModbusTCPCommunication Modbus TCP通信集成测试
func TestModbusTCPCommunication(t *testing.T) {
	// 创建存储
	s := store.New()

	// 创建测试连接
	conn := &model.Connection{
		ID:           generateUUID(),
		Name:         "IntegrationTest",
		Port:         15021,
		ProtocolType: 1, // Modbus TCP
	}
	if err := s.CreateConnection(conn); err != nil {
		t.Fatalf("Failed to create connection: %v", err)
	}

	// 创建从站
	slave := &model.Slave{
		ID:        generateUUID(),
		ConnID:    conn.ID,
		Name:      "TestSlave",
		SlaveAddr: 1,
	}
	if err := s.CreateSlave(slave); err != nil {
		t.Fatalf("Failed to create slave: %v", err)
	}

	// 创建Modbus处理器
	mbHandler := handler.NewModbusHandler(s, conn.ID)

	// 启动测试服务器
	listener, err := net.Listen("tcp", "127.0.0.1:15021")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- runModbusServer(listener, mbHandler)
	}()

	defer func() {
		listener.Close()
		select {
		case <-serverDone:
		case <-time.After(time.Second):
		}
	}()

	time.Sleep(300 * time.Millisecond)

	t.Run("CreateAndReadHoldingRegisters", func(t *testing.T) {
		// 创建寄存器
		reg := &model.Register{
			ID:        generateUUID(),
			SlaveID:   slave.ID,
			StartAddr: 40001,
			HexData:   "000A0014", // 10, 20
		}
		if err := s.CreateRegister(reg); err != nil {
			t.Fatalf("Failed to create register: %v", err)
		}

		t.Logf("📝 Created register: ID=%s, SlaveID=%s, Addr=%d, Data=%s",
			reg.ID, reg.SlaveID, reg.StartAddr, reg.HexData)

		// 测试读取
		data, err := mbHandler.ReadHoldingRegisters(1, 0, 2)
		if err != nil {
			t.Fatalf("Failed to read: %v", err)
		}

		if len(data) != 4 {
			t.Fatalf("Expected 4 bytes, got %d", len(data))
		}

		val1 := binary.BigEndian.Uint16(data[0:2])
		val2 := binary.BigEndian.Uint16(data[2:4])

		if val1 != 10 || val2 != 20 {
			t.Errorf("Values mismatch: got %d, %d, want 10, 20", val1, val2)
		} else {
			t.Logf("✅ Read holding registers: %d, %d", val1, val2)
		}
	})

	t.Run("ReadCoils", func(t *testing.T) {
		reg := &model.Register{
			ID:        generateUUID(),
			SlaveID:   slave.ID,
			StartAddr: 1,
			HexData:   "A5",
		}
		if err := s.CreateRegister(reg); err != nil {
			t.Fatalf("Failed to create coil: %v", err)
		}

		data, err := mbHandler.ReadCoils(1, 0, 8)
		if err != nil {
			t.Fatalf("Failed to read coils: %v", err)
		}

		if len(data) != 1 {
			t.Fatalf("Expected 1 byte, got %d", len(data))
		}

		t.Logf("✅ Read coils: 0x%02X", data[0])
	})

	t.Run("FullTCPCommunication", func(t *testing.T) {
		// 使用真实的TCP客户端测试
		client, err := net.DialTimeout("tcp", "127.0.0.1:15021", 5*time.Second)
		if err != nil {
			t.Fatalf("Failed to dial: %v", err)
		}
		defer client.Close()

		// 构建Modbus TCP请求
		request := []byte{
			0x00, 0x01, // Transaction ID
			0x00, 0x00, // Protocol ID
			0x00, 0x06, // Length
			0x01,       // Unit ID
			0x03,       // Function Code: Read Holding Registers
			0x00, 0x00, // Start Address (PDU)
			0x00, 0x02, // Quantity
		}

		// 发送请求
		if _, err := client.Write(request); err != nil {
			t.Fatalf("Failed to write: %v", err)
		}

		// 读取响应
		response := make([]byte, 256)
		client.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := client.Read(response)
		if err != nil {
			t.Fatalf("Failed to read: %v", err)
		}

		t.Logf("Received %d bytes: % X", n, response[:n])

		if n < 9 {
			t.Fatalf("Response too short: %d bytes. Expected at least 9 bytes (MBAP header: 7 + Function code: 1 + Byte count: 1 + Data: n)", n)
		}

		// 验证MBAP头
		if response[0] != 0x00 || response[1] != 0x01 {
			t.Errorf("Transaction ID mismatch: got 0x%02X, want 0x0001", binary.BigEndian.Uint16(response[0:2]))
		}

		if response[7] != 0x03 {
			t.Errorf("Function code mismatch: got 0x%02X, want 0x03", response[7])
		}

		// 验证数据长度
		dataLen := int(response[8])
		if n < 9+dataLen {
			t.Fatalf("Incomplete data: expected %d, got %d", dataLen, n-9)
		}

		// 验证数据内容
		if dataLen == 4 {
			val1 := binary.BigEndian.Uint16(response[9:11])
			val2 := binary.BigEndian.Uint16(response[11:13])
			if val1 == 10 && val2 == 20 {
				t.Logf("✅ Full TCP communication successful! Read: %d, %d", val1, val2)
			} else {
				t.Errorf("Data mismatch: got %d, %d, want 10, 20", val1, val2)
			}
		}
	})
}

// runModbusServer 运行Modbus服务器
func runModbusServer(listener net.Listener, h *handler.ModbusHandler) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		go handleConnection(conn, h)
	}
}

// handleConnection 处理Modbus连接
func handleConnection(conn net.Conn, h *handler.ModbusHandler) {
	defer conn.Close()

	buf := make([]byte, 256)

	for {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			return
		}

		if n == 0 {
			continue
		}

		// 解析TCP请求
		req, err := protocol.ParseTCPRequest(buf[:n])
		if err != nil {
			continue
		}

		// Debug logging
		// fmt.Printf("📨 Recv: FC=0x%02X, UnitID=%d, StartAddr=%d, Qty=%d\n",
		// 	req.FunctionCode, req.UnitID, req.StartAddress, req.Quantity)

		// 处理请求
		var respData []byte
		var errCode byte

		switch req.FunctionCode {
		case 0x01: // Read Coils
			var dataErr error
			respData, dataErr = h.ReadCoils(req.UnitID, req.StartAddress, req.Quantity)
			if dataErr != nil {
				errCode = 0x02 // Illegal data address
			}

		case 0x03: // Read Holding Registers
			var dataErr error
			respData, dataErr = h.ReadHoldingRegisters(req.UnitID, req.StartAddress, req.Quantity)
			if dataErr != nil {
				errCode = 0x02
			}

		case 0x02: // Read Discrete Inputs
			var dataErr error
			respData, dataErr = h.ReadDiscreteInputs(req.UnitID, req.StartAddress, req.Quantity)
			if dataErr != nil {
				errCode = 0x02
			}

		case 0x04: // Read Input Registers
			var dataErr error
			respData, dataErr = h.ReadInputRegisters(req.UnitID, req.StartAddress, req.Quantity)
			if dataErr != nil {
				errCode = 0x02
			}

		default:
			errCode = 0x01 // Illegal function
		}

		// 构建响应
		var response []byte

		if errCode != 0 {
			// 异常响应
			response = append([]byte{
				byte(req.TransactionID >> 8), byte(req.TransactionID),
				0, 0,
				0x00, 0x03,
				req.UnitID,
				0x80 | req.FunctionCode,
				errCode,
			}, 0x00)
		} else {
			// 正常响应: MBAP头(7字节) + 功能码(1字节) + 字节计数(1字节) + 数据
			// Length字段 = 功能码(1) + 字节计数(1) + 数据长度 = 2 + len(respData)
			length := uint16(2 + len(respData))
			response = append([]byte{
				byte(req.TransactionID >> 8), byte(req.TransactionID),
				0, 0,
				byte(length >> 8), byte(length), // Length
				req.UnitID,
				req.FunctionCode,
				byte(len(respData)), // Byte count
			}, respData...)
		}

		// 发送响应
		if _, err := conn.Write(response); err != nil {
			return
		}
	}
}
