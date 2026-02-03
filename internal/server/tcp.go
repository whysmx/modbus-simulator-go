package server

import (
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/whysmx/modbus-simulator-go/internal/handler"
	"github.com/whysmx/modbus-simulator-go/internal/model"
	"github.com/whysmx/modbus-simulator-go/internal/protocol"
	"github.com/whysmx/modbus-simulator-go/internal/store"
)

// TCPServer manages multiple TCP listeners for Modbus connections
type TCPServer struct {
	store     *store.Store
	listeners map[int]*portListener
	mu        sync.RWMutex
}

type portListener struct {
	listener     net.Listener
	connID       string
	done         chan struct{}
}

// NewTCPServer creates a new TCP server manager
func NewTCPServer(s *store.Store) *TCPServer {
	return &TCPServer{
		store:     s,
		listeners: make(map[int]*portListener),
	}
}

// StartListener starts a TCP listener for a connection
func (s *TCPServer) StartListener(conn *model.Connection) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.listeners[conn.Port]; exists {
		return fmt.Errorf("port %d already in use", conn.Port)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", conn.Port))
	if err != nil {
		return err
	}

	pl := &portListener{
		listener:     listener,
		connID:       conn.ID,
		done:         make(chan struct{}),
	}

	s.listeners[conn.Port] = pl

	go s.acceptLoop(pl, conn.Port)

	log.Printf("Modbus TCP listener started on port %d (protocol: %s)", conn.Port, protocolName(model.ModbusAuto))
	return nil
}

// StopListener stops a TCP listener for a connection
func (s *TCPServer) StopListener(port int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pl, exists := s.listeners[port]
	if !exists {
		return fmt.Errorf("no listener on port %d", port)
	}

	close(pl.done)
	pl.listener.Close()
	delete(s.listeners, port)

	log.Printf("Modbus TCP listener stopped on port %d", port)
	return nil
}

// StopAll stops all listeners
func (s *TCPServer) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for port, pl := range s.listeners {
		close(pl.done)
		pl.listener.Close()
		log.Printf("Stopped listener on port %d", port)
	}
	s.listeners = make(map[int]*portListener)
}

func (s *TCPServer) acceptLoop(pl *portListener, port int) {
	for {
		select {
		case <-pl.done:
			return
		default:
		}

		// Set accept timeout to allow checking done channel
		if tcpListener, ok := pl.listener.(*net.TCPListener); ok {
			tcpListener.SetDeadline(time.Now().Add(time.Second))
		}

		conn, err := pl.listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case <-pl.done:
				return
			default:
				log.Printf("Accept error on port %d: %v", port, err)
				continue
			}
		}

		go s.handleConnection(conn, pl)
	}
}

func (s *TCPServer) handleConnection(conn net.Conn, pl *portListener) {
	defer conn.Close()

	h := handler.NewModbusHandler(s.store, pl.connID)
	buf := make([]byte, 256)
	var rbuf []byte
	debugEnabled := os.Getenv("MODBUS_DEBUG") == "1"
	rtuGap := time.Duration(20) * time.Millisecond
	if gapEnv := os.Getenv("MODBUS_RTU_GAP_MS"); gapEnv != "" {
		if ms, err := strconv.Atoi(gapEnv); err == nil && ms >= 0 {
			rtuGap = time.Duration(ms) * time.Millisecond
		}
	}
	keepAlive := true
	keepAlivePeriod := 60 * time.Second
	if kaEnv := os.Getenv("MODBUS_KEEPALIVE_SEC"); kaEnv != "" {
		if sec, err := strconv.Atoi(kaEnv); err == nil {
			if sec <= 0 {
				keepAlive = false
			} else {
				keepAlivePeriod = time.Duration(sec) * time.Second
			}
		}
	}
	activeProto := model.ModbusAuto
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
		if keepAlive {
			_ = tcpConn.SetKeepAlive(true)
			_ = tcpConn.SetKeepAlivePeriod(keepAlivePeriod)
		} else {
			_ = tcpConn.SetKeepAlive(false)
		}
	}
	if debugEnabled {
		log.Printf("CONN %s proto=%s open", conn.RemoteAddr(), protocolName(activeProto))
		if keepAlive {
			log.Printf("CONN %s proto=%s keepalive=on period=%s", conn.RemoteAddr(), protocolName(activeProto), keepAlivePeriod)
		} else {
			log.Printf("CONN %s proto=%s keepalive=off", conn.RemoteAddr(), protocolName(activeProto))
		}
	}

	for {
		select {
		case <-pl.done:
			if debugEnabled {
				log.Printf("CONN %s proto=%s closed by server", conn.RemoteAddr(), protocolName(activeProto))
			}
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		n, err := conn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if debugEnabled {
					log.Printf("CONN %s proto=%s read timeout", conn.RemoteAddr(), protocolName(activeProto))
				}
				continue
			}
			if err != io.EOF {
				log.Printf("Read error from %s: %v", conn.RemoteAddr(), err)
			}
			if debugEnabled {
				log.Printf("CONN %s proto=%s closed by peer", conn.RemoteAddr(), protocolName(activeProto))
			}
			return
		}

		if n == 0 {
			continue
		}

		rbuf = append(rbuf, buf[:n]...)
		if len(rbuf) > 4096 {
			if debugEnabled {
				log.Printf("CONN %s proto=%s buffer overflow, dropping %d bytes", conn.RemoteAddr(), protocolName(activeProto), len(rbuf)-2048)
			}
			rbuf = rbuf[len(rbuf)-2048:]
		}

		if activeProto == model.ModbusAuto {
			_, rtuStatus := sniffRTUFrame(rbuf)
			_, tcpStatus := sniffTCPFrame(rbuf)
			switch {
			case rtuStatus == sniffOK:
				activeProto = model.ModbusRtuOverTcp
				if debugEnabled {
					log.Printf("CONN %s proto=auto->rtu detected", conn.RemoteAddr())
				}
			case tcpStatus == sniffOK:
				activeProto = model.ModbusTcp
				if debugEnabled {
					log.Printf("CONN %s proto=auto->tcp detected", conn.RemoteAddr())
				}
			default:
				if rtuStatus == sniffNeedMore || tcpStatus == sniffNeedMore {
					continue
				}
				if len(rbuf) > 0 {
					if debugEnabled {
						sample := rbuf
						if len(sample) > 32 {
							sample = sample[:32]
						}
						log.Printf("CONN %s proto=auto resync drop 1 byte data=%s", conn.RemoteAddr(), formatHex(sample))
					}
					rbuf = rbuf[1:]
				}
				continue
			}
		}

		var frames [][]byte
		var dropped int
		if activeProto == model.ModbusTcp {
			frames, rbuf = extractTCPFrames(rbuf)
		} else {
			frames, rbuf, dropped = extractRTUFrames(rbuf, func(sample []byte) {
				if !debugEnabled {
					return
				}
				log.Printf("CONN %s proto=rtu bad_crc bytes=%d data=%s", conn.RemoteAddr(), len(sample), formatHex(sample))
			})
		}

		if debugEnabled && dropped > 0 {
			log.Printf("CONN %s proto=%s dropped=%d bytes due to CRC mismatch", conn.RemoteAddr(), protocolName(activeProto), dropped)
		}

		if len(frames) == 0 {
			continue
		}

		if debugEnabled {
			for _, frame := range frames {
				log.Printf("RX %s proto=%s bytes=%d data=%s", conn.RemoteAddr(), protocolName(activeProto), len(frame), formatHex(frame))
			}
		}

		for _, frame := range frames {
			var req *protocol.Request
			var parseErr error

			if activeProto == model.ModbusTcp {
				req, parseErr = protocol.ParseTCPRequest(frame)
			} else {
				req, parseErr = protocol.ParseRTURequest(frame)
			}

			if parseErr != nil {
				log.Printf("Parse error from %s proto=%s bytes=%d err=%v data=%s", conn.RemoteAddr(), protocolName(activeProto), len(frame), parseErr, formatHex(frame))
				continue
			}

			if debugEnabled {
				log.Printf("REQ %s proto=%s unit=%d fc=0x%02X addr=%d qty=%d", conn.RemoteAddr(), protocolName(activeProto), req.UnitID, req.FunctionCode, req.StartAddress, req.Quantity)
			}

			// Skip response for broadcast address (per Modbus spec)
			if protocol.IsBroadcast(req.UnitID) {
				log.Printf("Broadcast request received, no response sent")
				continue
			}

			// Reject reserved UnitID range (248-255 per Modbus spec)
			if !protocol.IsValidUnitID(req.UnitID) {
				log.Printf("Reserved UnitID %d received, returning exception", req.UnitID)
				resp := &protocol.Response{
					TransactionID: req.TransactionID,
					ProtocolID:    req.ProtocolID,
					UnitID:        req.UnitID,
					FunctionCode:  req.FunctionCode,
					IsError:       true,
					ErrorCode:     protocol.ExceptionIllegalDataAddr,
				}
				var respBytes []byte
				if activeProto == model.ModbusTcp {
					respBytes = protocol.BuildTCPResponse(resp)
				} else {
					respBytes = protocol.BuildRTUResponse(resp)
				}
				if _, err := conn.Write(respBytes); err != nil {
					log.Printf("Write error to %s: %v", conn.RemoteAddr(), err)
					return
				}
				continue
			}

			resp := s.processRequest(req, h)

			var respBytes []byte
			if activeProto == model.ModbusTcp {
				respBytes = protocol.BuildTCPResponse(resp)
			} else {
				respBytes = protocol.BuildRTUResponse(resp)
			}

			if debugEnabled {
				log.Printf("TX %s proto=%s bytes=%d data=%s error=%v code=%d", conn.RemoteAddr(), protocolName(activeProto), len(respBytes), formatHex(respBytes), resp.IsError, resp.ErrorCode)
			}

			if _, err := conn.Write(respBytes); err != nil {
				log.Printf("Write error to %s: %v", conn.RemoteAddr(), err)
				return
			}

			if activeProto == model.ModbusRtuOverTcp && rtuGap > 0 {
				time.Sleep(rtuGap)
			}
		}
	}
}

func (s *TCPServer) processRequest(req *protocol.Request, h *handler.ModbusHandler) *protocol.Response {
	resp := &protocol.Response{
		TransactionID: req.TransactionID,
		ProtocolID:    req.ProtocolID,
		UnitID:        req.UnitID,
		FunctionCode:  req.FunctionCode,
	}

	// Validate quantity per Modbus spec (before processing)
	if err := protocol.ValidateQuantity(req.FunctionCode, req.Quantity); err != nil {
		resp.IsError = true
		resp.ErrorCode = protocol.ExceptionIllegalDataValue
		return resp
	}

	// Validate address range (startAddress + quantity must not overflow)
	if err := protocol.ValidateAddressRange(req.StartAddress, req.Quantity); err != nil {
		resp.IsError = true
		resp.ErrorCode = protocol.ExceptionIllegalDataAddr
		return resp
	}

	var data []byte
	var err error

	switch req.FunctionCode {
	case protocol.FCReadCoils:
		data, err = h.ReadCoils(req.UnitID, req.StartAddress, req.Quantity)
	case protocol.FCReadDiscreteInputs:
		data, err = h.ReadDiscreteInputs(req.UnitID, req.StartAddress, req.Quantity)
	case protocol.FCReadHoldingRegs:
		data, err = h.ReadHoldingRegisters(req.UnitID, req.StartAddress, req.Quantity)
	case protocol.FCReadInputRegs:
		data, err = h.ReadInputRegisters(req.UnitID, req.StartAddress, req.Quantity)
	case protocol.FCWriteSingleCoil:
		err = h.WriteSingleCoil(req.UnitID, req.StartAddress, req.WriteData)
		if err == nil {
			// Echo response: Address(2) + Value(2)
			resp.IsEcho = true
			resp.Data = make([]byte, 4)
			resp.Data[0] = byte(req.StartAddress >> 8)
			resp.Data[1] = byte(req.StartAddress)
			copy(resp.Data[2:4], req.WriteData)
			return resp
		}
	case protocol.FCWriteSingleReg:
		err = h.WriteSingleRegister(req.UnitID, req.StartAddress, req.WriteData)
		if err == nil {
			// Echo response: Address(2) + Value(2)
			resp.IsEcho = true
			resp.Data = make([]byte, 4)
			resp.Data[0] = byte(req.StartAddress >> 8)
			resp.Data[1] = byte(req.StartAddress)
			copy(resp.Data[2:4], req.WriteData)
			return resp
		}
	case protocol.FCWriteMultipleCoils:
		err = h.WriteMultipleCoils(req.UnitID, req.StartAddress, req.Quantity, req.WriteData)
		if err == nil {
			// Echo response: Address(2) + Quantity(2)
			resp.IsEcho = true
			resp.Data = make([]byte, 4)
			resp.Data[0] = byte(req.StartAddress >> 8)
			resp.Data[1] = byte(req.StartAddress)
			resp.Data[2] = byte(req.Quantity >> 8)
			resp.Data[3] = byte(req.Quantity)
			return resp
		}
	case protocol.FCWriteMultipleRegs:
		err = h.WriteMultipleRegisters(req.UnitID, req.StartAddress, req.Quantity, req.WriteData)
		if err == nil {
			// Echo response: Address(2) + Quantity(2)
			resp.IsEcho = true
			resp.Data = make([]byte, 4)
			resp.Data[0] = byte(req.StartAddress >> 8)
			resp.Data[1] = byte(req.StartAddress)
			resp.Data[2] = byte(req.Quantity >> 8)
			resp.Data[3] = byte(req.Quantity)
			return resp
		}
	default:
		resp.IsError = true
		resp.ErrorCode = protocol.ExceptionIllegalFunction
		return resp
	}

	if err != nil {
		resp.IsError = true
		switch err {
		case protocol.ErrIllegalDataAddr:
			resp.ErrorCode = protocol.ExceptionIllegalDataAddr
		case protocol.ErrIllegalDataValue:
			resp.ErrorCode = protocol.ExceptionIllegalDataValue
		default:
			resp.ErrorCode = protocol.ExceptionIllegalFunction
		}
		return resp
	}

	resp.Data = data
	return resp
}

// UpdateListener updates a listener (requires restart)
func (s *TCPServer) UpdateListener(oldPort int, conn *model.Connection) error {
	// Stop old listener
	if err := s.StopListener(oldPort); err != nil {
		// Ignore if not running
	}
	// Start new listener
	return s.StartListener(conn)
}

func protocolName(p model.ProtocolType) string {
	switch p {
	case model.ModbusTcp:
		return "tcp"
	case model.ModbusRtuOverTcp:
		return "rtu"
	case model.ModbusAuto:
		return "auto"
	default:
		return "unknown"
	}
}

func formatHex(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	hexStr := strings.ToUpper(hex.EncodeToString(data))
	var b strings.Builder
	for i := 0; i < len(hexStr); i += 2 {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(hexStr[i : i+2])
	}
	return b.String()
}

func extractTCPFrames(buf []byte) ([][]byte, []byte) {
	var frames [][]byte
	for {
		if len(buf) < 6 {
			break
		}
		length := int(buf[4])<<8 | int(buf[5])
		total := 6 + length
		if total <= 0 || total > 260 {
			// Invalid length, drop one byte to resync
			buf = buf[1:]
			continue
		}
		if len(buf) < total {
			break
		}
		frame := buf[:total]
		frames = append(frames, frame)
		buf = buf[total:]
	}
	return frames, buf
}

type sniffStatus int

const (
	sniffNeedMore sniffStatus = iota
	sniffInvalid
	sniffOK
)

func sniffRTUFrame(buf []byte) ([]byte, sniffStatus) {
	if len(buf) < 8 {
		return nil, sniffNeedMore
	}

	fc := buf[1]
	frameLen := 8
	if fc == protocol.FCWriteMultipleCoils || fc == protocol.FCWriteMultipleRegs {
		if len(buf) < 7 {
			return nil, sniffNeedMore
		}
		byteCount := int(buf[6])
		frameLen = 9 + byteCount
	}

	if len(buf) < frameLen {
		return nil, sniffNeedMore
	}
	frame := buf[:frameLen]
	if !protocol.ValidateCRC(frame) {
		return nil, sniffInvalid
	}
	return frame, sniffOK
}

func sniffTCPFrame(buf []byte) ([]byte, sniffStatus) {
	if len(buf) < 7 {
		return nil, sniffNeedMore
	}
	protocolID := int(buf[2])<<8 | int(buf[3])
	if protocolID != 0 {
		return nil, sniffInvalid
	}
	length := int(buf[4])<<8 | int(buf[5])
	if length < 3 || length > 260 {
		return nil, sniffInvalid
	}
	total := 6 + length
	if len(buf) < total {
		return nil, sniffNeedMore
	}
	frame := buf[:total]
	if _, err := protocol.ParseTCPRequest(frame); err != nil {
		return nil, sniffInvalid
	}
	return frame, sniffOK
}

func extractRTUFrames(buf []byte, onBadCRC func(sample []byte)) ([][]byte, []byte, int) {
	var frames [][]byte
	dropped := 0
	for {
		if len(buf) < 8 {
			break
		}

		fc := buf[1]
		frameLen := 8
		if fc == protocol.FCWriteMultipleCoils || fc == protocol.FCWriteMultipleRegs {
			if len(buf) < 7 {
				break
			}
			byteCount := int(buf[6])
			frameLen = 9 + byteCount
		}

		if len(buf) < frameLen {
			break
		}

		frame := buf[:frameLen]
		if !protocol.ValidateCRC(frame) {
			if onBadCRC != nil {
				sample := frame
				if len(sample) > 32 {
					sample = sample[:32]
				}
				onBadCRC(sample)
			}
			buf = buf[1:]
			dropped++
			continue
		}

		frames = append(frames, frame)
		buf = buf[frameLen:]
	}
	return frames, buf, dropped
}
