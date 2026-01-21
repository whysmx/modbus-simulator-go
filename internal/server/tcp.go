package server

import (
	"fmt"
	"io"
	"log"
	"net"
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
	protocolType model.ProtocolType
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
		protocolType: conn.ProtocolType,
		done:         make(chan struct{}),
	}

	s.listeners[conn.Port] = pl

	go s.acceptLoop(pl, conn.Port)

	log.Printf("Modbus TCP listener started on port %d (protocol: %d)", conn.Port, conn.ProtocolType)
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

	for {
		select {
		case <-pl.done:
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
					log.Printf("Read error: %v", err)
				}
			}
			return
		}

		if n == 0 {
			continue
		}

		var req *protocol.Request
		var parseErr error

		if pl.protocolType == model.ModbusTcp {
			req, parseErr = protocol.ParseTCPRequest(buf[:n])
		} else {
			req, parseErr = protocol.ParseRTURequest(buf[:n])
		}

		if parseErr != nil {
			log.Printf("Parse error: %v", parseErr)
			continue
		}

		resp := s.processRequest(req, h)

		var respBytes []byte
		if pl.protocolType == model.ModbusTcp {
			respBytes = protocol.BuildTCPResponse(resp)
		} else {
			respBytes = protocol.BuildRTUResponse(resp)
		}

		conn.Write(respBytes)
	}
}

func (s *TCPServer) processRequest(req *protocol.Request, h *handler.ModbusHandler) *protocol.Response {
	resp := &protocol.Response{
		TransactionID: req.TransactionID,
		ProtocolID:    req.ProtocolID,
		UnitID:        req.UnitID,
		FunctionCode:  req.FunctionCode,
	}

	var data []byte
	var err error

	switch req.FunctionCode {
	case 0x01:
		data, err = h.ReadCoils(req.UnitID, req.StartAddress, req.Quantity)
	case 0x02:
		data, err = h.ReadDiscreteInputs(req.UnitID, req.StartAddress, req.Quantity)
	case 0x03:
		data, err = h.ReadHoldingRegisters(req.UnitID, req.StartAddress, req.Quantity)
	case 0x04:
		data, err = h.ReadInputRegisters(req.UnitID, req.StartAddress, req.Quantity)
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

// UpdateListener updates a listener's protocol type (requires restart)
func (s *TCPServer) UpdateListener(oldPort int, conn *model.Connection) error {
	// Stop old listener
	if err := s.StopListener(oldPort); err != nil {
		// Ignore if not running
	}
	// Start new listener
	return s.StartListener(conn)
}
