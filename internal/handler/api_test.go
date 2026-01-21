package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/whysmx/modbus-simulator-go/internal/model"
	"github.com/whysmx/modbus-simulator-go/internal/store"
)

// Mock TCP server for testing
type mockTCPServer struct {
	startedPorts []int
	stoppedPorts []int
}

func (m *mockTCPServer) StartListener(conn *model.Connection) error {
	m.startedPorts = append(m.startedPorts, conn.Port)
	return nil
}

func (m *mockTCPServer) StopListener(port int) error {
	m.stoppedPorts = append(m.stoppedPorts, port)
	return nil
}

func (m *mockTCPServer) UpdateListener(oldPort int, conn *model.Connection) error {
	m.stoppedPorts = append(m.stoppedPorts, oldPort)
	m.startedPorts = append(m.startedPorts, conn.Port)
	return nil
}

func TestGetConnectionsTree(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	// Create test data
	conn := &model.Connection{ID: "testconnidtestconnidtestconnidtes", Name: "Test", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "testslaveidtestslaveidtestslaveid", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	req := httptest.NewRequest("GET", "/api/connections/tree", nil)
	w := httptest.NewRecorder()

	h.GetConnectionsTree(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var tree []TreeNode
	json.NewDecoder(w.Body).Decode(&tree)

	if len(tree) != 1 {
		t.Errorf("Tree length = %d, want 1", len(tree))
	}
	if len(tree[0].Slaves) != 1 {
		t.Errorf("Slaves length = %d, want 1", len(tree[0].Slaves))
	}
}

func TestCreateConnection(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	body := bytes.NewBufferString(`{"name":"New Connection"}`)
	req := httptest.NewRequest("POST", "/api/connections", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateConnection(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var conn model.Connection
	json.NewDecoder(w.Body).Decode(&conn)

	if conn.Name != "New Connection" {
		t.Errorf("Name = %s, want 'New Connection'", conn.Name)
	}
	if conn.Port != 1502 {
		t.Errorf("Port = %d, want 1502", conn.Port)
	}
	if len(conn.ID) != 32 {
		t.Errorf("ID length = %d, want 32", len(conn.ID))
	}

	// Verify TCP listener started
	if len(tcp.startedPorts) != 1 || tcp.startedPorts[0] != 1502 {
		t.Error("TCP listener not started")
	}
}

func TestCreateSlave(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	// Create connection first
	conn := &model.Connection{ID: "connforslavecreateconnforslave", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	body := bytes.NewBufferString(`{"name":"Slave 1","slaveAddr":5}`)
	req := httptest.NewRequest("POST", "/api/connections/"+conn.ID+"/slaves", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateSlave(w, req, conn.ID)

	if w.Code != http.StatusCreated {
		t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var slave model.Slave
	json.NewDecoder(w.Body).Decode(&slave)

	if slave.Name != "Slave 1" {
		t.Errorf("Name = %s, want 'Slave 1'", slave.Name)
	}
	if slave.SlaveAddr != 5 {
		t.Errorf("SlaveAddr = %d, want 5", slave.SlaveAddr)
	}
}

func TestCreateSlaveInvalidAddr(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforinvalidslaveconnforinvali", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	body := bytes.NewBufferString(`{"name":"Bad Slave","slaveAddr":300}`)
	req := httptest.NewRequest("POST", "/api/connections/"+conn.ID+"/slaves", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateSlave(w, req, conn.ID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateRegister(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforregisterconnforregisterco", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slaveforregisterslaveforregisters", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	body := bytes.NewBufferString(`{"startAddr":40001,"hexData":"1234ABCD"}`)
	req := httptest.NewRequest("POST", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateRegister(w, req, conn.ID, slave.ID)

	if w.Code != http.StatusCreated {
		t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var reg model.Register
	json.NewDecoder(w.Body).Decode(&reg)

	if reg.StartAddr != 40001 {
		t.Errorf("StartAddr = %d, want 40001", reg.StartAddr)
	}
	if reg.HexData != "1234ABCD" {
		t.Errorf("HexData = %s, want '1234ABCD'", reg.HexData)
	}
}

func TestCreateRegisterInvalidHex(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforinvalidhexconnforinvalidh", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slaveforinvalidhexslaveforinvalid", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	// Invalid hex - not multiple of 4 for holding register
	body := bytes.NewBufferString(`{"startAddr":40001,"hexData":"123"}`)
	req := httptest.NewRequest("POST", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateRegister(w, req, conn.ID, slave.ID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDeleteConnection(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "conntodeleteconntodeletconntodel", Name: "ToDelete", Port: 1502}
	s.CreateConnection(conn)

	req := httptest.NewRequest("DELETE", "/api/connections/"+conn.ID, nil)
	w := httptest.NewRecorder()

	h.DeleteConnection(w, req, conn.ID)

	if w.Code != http.StatusNoContent {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNoContent)
	}

	// Verify deleted
	_, err := s.GetConnection(conn.ID)
	if err != store.ErrNotFound {
		t.Error("Connection should be deleted")
	}

	// Verify TCP listener stopped
	if len(tcp.stoppedPorts) != 1 || tcp.stoppedPorts[0] != 1502 {
		t.Error("TCP listener not stopped")
	}
}

func TestGetRegisters(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforgetregsconnforgetregsconn", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slaveforgetregsslaveforgetregssl", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "regforgetregforgetregforgetregfo", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"}
	s.CreateRegister(reg)

	req := httptest.NewRequest("GET", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers", nil)
	w := httptest.NewRecorder()

	h.GetRegisters(w, req, conn.ID, slave.ID)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var regs []*model.Register
	json.NewDecoder(w.Body).Decode(&regs)

	if len(regs) != 1 {
		t.Errorf("Register count = %d, want 1", len(regs))
	}
}
