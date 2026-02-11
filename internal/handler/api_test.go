package handler

import (
	"bytes"
	"encoding/json"
	"errors"
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

	body := bytes.NewBufferString(`{"startAddr":40001,"hexData":"1234ABCD","jitterAmp":5}`)
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
	if reg.JitterAmp != 5 {
		t.Errorf("JitterAmp = %d, want 5", reg.JitterAmp)
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

// Additional tests for error cases and edge cases

func TestCreateConnectionInvalidJSON(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest("POST", "/api/connections", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateConnection(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateConnectionPortInUse(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	// Create first connection
	conn1 := &model.Connection{ID: "conn1portinuseconn1portinusecon1", Name: "Conn1", Port: 1502}
	s.CreateConnection(conn1)

	// Try to create connection with same port
	body := bytes.NewBufferString(`{"name":"New Connection","port":1502}`)
	req := httptest.NewRequest("POST", "/api/connections", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateConnection(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

type failingTCPServer struct {
	mockTCPServer
}

func (m *failingTCPServer) StartListener(conn *model.Connection) error {
	return errors.New("failed to start listener")
}

func TestCreateConnectionTCPFailed(t *testing.T) {
	s := store.New()
	tcp := &failingTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	body := bytes.NewBufferString(`{"name":"New Connection"}`)
	req := httptest.NewRequest("POST", "/api/connections", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateConnection(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateConnection(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "conntoupdateconntoupdateconntoup", Name: "Original", Port: 1502}
	s.CreateConnection(conn)

	body := bytes.NewBufferString(`{"name":"Updated","port":1503}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID, body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateConnection(w, req, conn.ID)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var updated model.Connection
	json.NewDecoder(w.Body).Decode(&updated)

	if updated.Name != "Updated" {
		t.Errorf("Name = %s, want Updated", updated.Name)
	}
	if updated.Port != 1503 {
		t.Errorf("Port = %d, want 1503", updated.Port)
	}
}

func TestUpdateConnectionNotFound(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	body := bytes.NewBufferString(`{"name":"Updated"}`)
	req := httptest.NewRequest("PUT", "/api/connections/nonexistent", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateConnection(w, req, "nonexistentidnonexistentidnonex")

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUpdateConnectionInvalidJSON(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "conninvalidjsonconninvalidjsonco", Name: "Original", Port: 1502}
	s.CreateConnection(conn)

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID, body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateConnection(w, req, conn.ID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateConnectionPortInUse(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn1 := &model.Connection{ID: "conn1forupdateconn1forupdatecon1", Name: "Conn1", Port: 1502}
	conn2 := &model.Connection{ID: "conn2forupdateconn2forupdatecon2", Name: "Conn2", Port: 1503}
	s.CreateConnection(conn1)
	s.CreateConnection(conn2)

	// Try to update conn2 to use conn1's port
	body := bytes.NewBufferString(`{"name":"Updated","port":1502}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn2.ID, body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateConnection(w, req, conn2.ID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDeleteConnectionNotFound(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	req := httptest.NewRequest("DELETE", "/api/connections/nonexistent", nil)
	w := httptest.NewRecorder()

	h.DeleteConnection(w, req, "nonexistentidnonexistentidnonex")

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestCreateSlaveNotFoundConnection(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	body := bytes.NewBufferString(`{"name":"Slave","slaveAddr":1}`)
	req := httptest.NewRequest("POST", "/api/connections/nonexistent/slaves", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateSlave(w, req, "nonexistentidnonexistentidnonex")

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestCreateSlaveInvalidJSON(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforslavejsonconnforslavejson", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest("POST", "/api/connections/"+conn.ID+"/slaves", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateSlave(w, req, conn.ID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateSlaveZeroAddr(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforslavezeroconnforslavezero", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	body := bytes.NewBufferString(`{"name":"Slave","slaveAddr":0}`)
	req := httptest.NewRequest("POST", "/api/connections/"+conn.ID+"/slaves", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateSlave(w, req, conn.ID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateSlave(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforupdateslavconnforupdatesla", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slavetoupdateslavetoupdateslav01", ConnID: conn.ID, Name: "Original", SlaveAddr: 1}
	s.CreateSlave(slave)

	body := bytes.NewBufferString(`{"name":"Updated","slaveAddr":5}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID+"/slaves/"+slave.ID, body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateSlave(w, req, conn.ID, slave.ID)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestUpdateSlaveNotFoundConnection(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	body := bytes.NewBufferString(`{"name":"Updated","slaveAddr":1}`)
	req := httptest.NewRequest("PUT", "/api/connections/nonexistent/slaves/xxx", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateSlave(w, req, "nonexistentidnonexistentidnonex", "slaveidxslaveidxslaveidxslaveid")

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUpdateSlaveNotFound(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforupslavenotfconnforupslave", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	body := bytes.NewBufferString(`{"name":"Updated","slaveAddr":1}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID+"/slaves/nonexistent", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateSlave(w, req, conn.ID, "nonexistentidnonexistentidnonex")

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUpdateSlaveInvalidJSON(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforupslavejsonconnforupslave", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slavejsoninvalslavejsoninvalslav", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID+"/slaves/"+slave.ID, body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateSlave(w, req, conn.ID, slave.ID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateSlaveInvalidAddr(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforupslaveaddrconnforupslave", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slaveaddrinvalslaveaddrinvalsla1", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	body := bytes.NewBufferString(`{"name":"Updated","slaveAddr":300}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID+"/slaves/"+slave.ID, body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateSlave(w, req, conn.ID, slave.ID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDeleteSlave(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connfordelslavconfordelslavconfo", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slavetodeleteslavetodeleteslav01", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	req := httptest.NewRequest("DELETE", "/api/connections/"+conn.ID+"/slaves/"+slave.ID, nil)
	w := httptest.NewRecorder()

	h.DeleteSlave(w, req, conn.ID, slave.ID)

	if w.Code != http.StatusNoContent {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestDeleteSlaveNotFound(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connfordelslavnfconnfordelslavnf", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	req := httptest.NewRequest("DELETE", "/api/connections/"+conn.ID+"/slaves/nonexistent", nil)
	w := httptest.NewRecorder()

	h.DeleteSlave(w, req, conn.ID, "nonexistentidnonexistentidnonex")

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestGetRegistersNotFoundSlave(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforgetregsnfconnforgetregsnf", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	req := httptest.NewRequest("GET", "/api/connections/"+conn.ID+"/slaves/nonexistent/registers", nil)
	w := httptest.NewRecorder()

	h.GetRegisters(w, req, conn.ID, "nonexistentidnonexistentidnonex")

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestCreateRegisterNotFoundSlave(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforcreatregsnfconnforcreatre", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	body := bytes.NewBufferString(`{"startAddr":40001,"hexData":"1234"}`)
	req := httptest.NewRequest("POST", "/api/connections/"+conn.ID+"/slaves/nonexistent/registers", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateRegister(w, req, conn.ID, "nonexistentidnonexistentidnonex")

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestCreateRegisterInvalidJSON(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforcregregjsonconnforcreregj", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slavecregregjsonslavecregregjson", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest("POST", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateRegister(w, req, conn.ID, slave.ID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateRegisterInvalidAddr(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforcreregaddrconnforcreregad", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slavecreregaddrslavecreregaddrs1", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	body := bytes.NewBufferString(`{"startAddr":105537,"hexData":"1234"}`)
	req := httptest.NewRequest("POST", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateRegister(w, req, conn.ID, slave.ID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateRegisterOverlap(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforcregregoverlconnforcregre", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slavecreregoverslavecreregoversl", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	// Create first register
	reg := &model.Register{ID: "reg1overlapreg1overlapreg1overl", SlaveID: slave.ID, StartAddr: 40001, HexData: "12345678"}
	s.CreateRegister(reg)

	// Try to create overlapping register
	body := bytes.NewBufferString(`{"startAddr":40002,"hexData":"ABCD"}`)
	req := httptest.NewRequest("POST", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateRegister(w, req, conn.ID, slave.ID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateRegister(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforupdateregconnforupdatereg", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slaveupdateregslaveupdateregsla1", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "regtoupdateregtoupdateregtoupda1", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"}
	s.CreateRegister(reg)

	body := bytes.NewBufferString(`{"startAddr":40001,"hexData":"ABCD","jitterAmp":11}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers/"+reg.ID, body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateRegister(w, req, conn.ID, slave.ID, reg.ID)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	updated, err := s.GetRegister(reg.ID)
	if err != nil {
		t.Fatalf("GetRegister failed: %v", err)
	}
	if updated.JitterAmp != 11 {
		t.Errorf("Updated JitterAmp = %d, want 11", updated.JitterAmp)
	}
}

func TestUpdateRegisterNotFoundSlave(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforupregnfslavconnforupregns", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	body := bytes.NewBufferString(`{"startAddr":40001,"hexData":"1234"}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID+"/slaves/nonexistent/registers/xxx", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateRegister(w, req, conn.ID, "nonexistentidnonexistentidnonex", "regididregidid")

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUpdateRegisterNotFound(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforupregnfregconnforupregnfr", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slaveupregnotfslaveupregnotfslav", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	body := bytes.NewBufferString(`{"startAddr":40001,"hexData":"1234"}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers/nonexistent", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateRegister(w, req, conn.ID, slave.ID, "nonexistentidnonexistentidnonex")

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUpdateRegisterInvalidJSON(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforupregjsonconnforupregjson", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slaveupregjsonslaveupregjsonslav", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "regupregjsonregupregjsonregupre1", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"}
	s.CreateRegister(reg)

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers/"+reg.ID, body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateRegister(w, req, conn.ID, slave.ID, reg.ID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateRegisterInvalidAddr(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforupregaddrconnforupregaddr", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slaveupregaddrslaveupregaddrsla1", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "regupaddrregupaddrregupaddrreg01", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"}
	s.CreateRegister(reg)

	body := bytes.NewBufferString(`{"startAddr":105537,"hexData":"1234"}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers/"+reg.ID, body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateRegister(w, req, conn.ID, slave.ID, reg.ID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateRegisterInvalidHex(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforupreghexconnforupreghexco", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slaveupreghexslaveupreghexslav01", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "reguphexreguphexreguphexreguph01", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"}
	s.CreateRegister(reg)

	body := bytes.NewBufferString(`{"startAddr":40001,"hexData":"123"}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers/"+reg.ID, body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateRegister(w, req, conn.ID, slave.ID, reg.ID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateRegisterOverlap(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforupregoverlconnforupregove", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slaveupregoverslaveupregoverslav", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg1 := &model.Register{ID: "reg1upoverlapreg1upoverlapreg101", SlaveID: slave.ID, StartAddr: 40001, HexData: "12345678"}
	reg2 := &model.Register{ID: "reg2upoverlapreg2upoverlapreg201", SlaveID: slave.ID, StartAddr: 40005, HexData: "ABCD"}
	s.CreateRegister(reg1)
	s.CreateRegister(reg2)

	// Try to update reg2 to overlap with reg1
	body := bytes.NewBufferString(`{"startAddr":40002,"hexData":"ABCD"}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers/"+reg2.ID, body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateRegister(w, req, conn.ID, slave.ID, reg2.ID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDeleteRegister(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connfordelregconnfordelregconnfo", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slavedelregslavedelivregslavedr1", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "regtodeleteregtodeleteregtodelr1", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"}
	s.CreateRegister(reg)

	req := httptest.NewRequest("DELETE", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers/"+reg.ID, nil)
	w := httptest.NewRecorder()

	h.DeleteRegister(w, req, conn.ID, slave.ID, reg.ID)

	if w.Code != http.StatusNoContent {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestDeleteRegisterNotFoundSlave(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connfordelregnfslaconnfordelregn", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	req := httptest.NewRequest("DELETE", "/api/connections/"+conn.ID+"/slaves/nonexistent/registers/xxx", nil)
	w := httptest.NewRecorder()

	h.DeleteRegister(w, req, conn.ID, "nonexistentidnonexistentidnonex", "regididregidid")

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestDeleteRegisterNotFound(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connfordelregnfregconnfordelregn", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slavedelregnfslavedelregnfslav01", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	req := httptest.NewRequest("DELETE", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers/nonexistent", nil)
	w := httptest.NewRecorder()

	h.DeleteRegister(w, req, conn.ID, slave.ID, "nonexistentidnonexistentidnonex")

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestIsValidHexData(t *testing.T) {
	tests := []struct {
		hexData   string
		startAddr int
		expected  bool
	}{
		{"1234", 40001, true},       // Holding register, 4 hex chars
		{"12345678", 40001, true},   // Holding register, 8 hex chars
		{"123", 40001, false},       // Holding register, wrong length
		{"AB", 1, true},             // Coil, 2 hex chars
		{"ABCD", 1, true},           // Coil, 4 hex chars
		{"A", 1, false},             // Coil, wrong length
		{"", 40001, false},          // Empty
		{"GHIJ", 40001, false},      // Invalid hex characters
		{"abcd", 40001, false},      // Lowercase (not uppercase)
		{"1234", 30001, true},       // Input register
		{"12", 10001, true},         // Discrete input
	}

	for _, tt := range tests {
		result := isValidHexData(tt.hexData, tt.startAddr)
		if result != tt.expected {
			t.Errorf("isValidHexData(%q, %d) = %v, want %v", tt.hexData, tt.startAddr, result, tt.expected)
		}
	}
}

func TestGenerateUUID(t *testing.T) {
	id1 := generateUUID()
	id2 := generateUUID()

	if len(id1) != 32 {
		t.Errorf("UUID length = %d, want 32", len(id1))
	}
	if id1 == id2 {
		t.Error("Two generated UUIDs should not be equal")
	}
}

// Test CreateConnection rollback when TCP listener fails
func TestCreateConnectionRollbackOnTCPFail(t *testing.T) {
	s := store.New()
	tcp := &failingTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	body := bytes.NewBufferString(`{"name":"New Connection","port":1505}`)
	req := httptest.NewRequest("POST", "/api/connections", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateConnection(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	// Verify connection was rolled back from store
	conns := s.GetAllConnections()
	if len(conns) != 0 {
		t.Errorf("Expected 0 connections after rollback, got %d", len(conns))
	}

	// Verify port is also freed
	_, err := s.GetConnectionByPort(1505)
	if err != store.ErrNotFound {
		t.Error("Port should be freed after rollback")
	}
}

// Mock that tracks UpdateListener calls
type trackingTCPServer struct {
	mockTCPServer
	updateCalls []struct {
		oldPort int
		newPort int
	}
}

func (m *trackingTCPServer) UpdateListener(oldPort int, conn *model.Connection) error {
	m.updateCalls = append(m.updateCalls, struct {
		oldPort int
		newPort int
	}{oldPort, conn.Port})
	return nil
}

func TestUpdateConnectionTriggersUpdateListener(t *testing.T) {
	s := store.New()
	tcp := &trackingTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "updatelistenerconnupdatelistener", Name: "Original", Port: 1502, ProtocolType: 0}
	s.CreateConnection(conn)

	// Update port
	body := bytes.NewBufferString(`{"name":"Updated","port":1600}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID, body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateConnection(w, req, conn.ID)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Verify UpdateListener was called with correct ports
	if len(tcp.updateCalls) != 1 {
		t.Fatalf("UpdateListener called %d times, want 1", len(tcp.updateCalls))
	}
	if tcp.updateCalls[0].oldPort != 1502 {
		t.Errorf("oldPort = %d, want 1502", tcp.updateCalls[0].oldPort)
	}
	if tcp.updateCalls[0].newPort != 1600 {
		t.Errorf("newPort = %d, want 1600", tcp.updateCalls[0].newPort)
	}
}

func TestUpdateConnectionProtocolChangeIgnored(t *testing.T) {
	s := store.New()
	tcp := &trackingTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "protocolchangeconnprotocolchange", Name: "Test", Port: 1502, ProtocolType: 0}
	s.CreateConnection(conn)

	// Update protocol type only (same port) should be ignored
	body := bytes.NewBufferString(`{"name":"Test","port":1502,"protocolType":1}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID, body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateConnection(w, req, conn.ID)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify UpdateListener was NOT called (protocol ignored)
	if len(tcp.updateCalls) != 0 {
		t.Errorf("UpdateListener called %d times, want 0", len(tcp.updateCalls))
	}
}

func TestUpdateConnectionNoChangeNoUpdateListener(t *testing.T) {
	s := store.New()
	tcp := &trackingTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "nochangeconnnochangeconnnochange", Name: "Test", Port: 1502, ProtocolType: 0}
	s.CreateConnection(conn)

	// Update name only (no port change)
	body := bytes.NewBufferString(`{"name":"Updated Name","port":1502}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID, body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateConnection(w, req, conn.ID)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify UpdateListener was NOT called
	if len(tcp.updateCalls) != 0 {
		t.Errorf("UpdateListener called %d times, want 0 (no port/protocol change)", len(tcp.updateCalls))
	}
}

// TestGetRegister tests getting a single register by ID
func TestGetRegister(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connforgetregconnforgetregconnf", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slaveforgetregslaveforgetregsla", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "regidforgetregregidforgetregregi", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"}
	s.CreateRegister(reg)

	req := httptest.NewRequest("GET", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers/"+reg.ID, nil)
	w := httptest.NewRecorder()

	h.GetRegister(w, req, conn.ID, slave.ID, reg.ID)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var returnedReg model.Register
	json.NewDecoder(w.Body).Decode(&returnedReg)

	if returnedReg.ID != reg.ID {
		t.Errorf("Register ID = %s, want %s", returnedReg.ID, reg.ID)
	}
}

// TestGetRegisterNotFoundSlave tests getting register with non-existent slave
func TestGetRegisterNotFoundSlave(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connidnotfoundslaveconnidnotfound", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	req := httptest.NewRequest("GET", "/api/connections/"+conn.ID+"/slaves/nonexistent/registers/regid", nil)
	w := httptest.NewRecorder()

	h.GetRegister(w, req, conn.ID, "nonexistent", "regid")

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// TestGetRegisterWrongConn tests getting register with wrong connection ID
func TestGetRegisterWrongConn(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn1 := &model.Connection{ID: "conn1wrongconnconn1wrongconnconn1w", Name: "Conn1", Port: 1502}
	s.CreateConnection(conn1)
	conn2 := &model.Connection{ID: "conn2wrongconnconn2wrongconnconn2w", Name: "Conn2", Port: 1503}
	s.CreateConnection(conn2)

	slave := &model.Slave{ID: "slavewrongconnslavewrongconnslave", ConnID: conn1.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "regwrongconnregwrongconnregwrongc", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"}
	s.CreateRegister(reg)

	// Try to get register using wrong connection ID
	req := httptest.NewRequest("GET", "/api/connections/"+conn2.ID+"/slaves/"+slave.ID+"/registers/"+reg.ID, nil)
	w := httptest.NewRecorder()

	h.GetRegister(w, req, conn2.ID, slave.ID, reg.ID)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// TestGetRegisterNotFound tests getting non-existent register
func TestGetRegisterNotFound(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connnotfoundregconnnotfoundregcon", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slavenotfoundregslavenotfoundreg", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	req := httptest.NewRequest("GET", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers/nonexistent", nil)
	w := httptest.NewRecorder()

	h.GetRegister(w, req, conn.ID, slave.ID, "nonexistent")

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// TestGetRegisterWrongSlave tests getting register with wrong slave ID
func TestGetRegisterWrongSlave(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connwrongsconnconnwrongsconnconnw", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	slave1 := &model.Slave{ID: "slave1wrongsslave1wrongs_slave1wr", ConnID: conn.ID, Name: "Slave1", SlaveAddr: 1}
	s.CreateSlave(slave1)
	slave2 := &model.Slave{ID: "slave2wrongs_slave2wrongs_slave2wr", ConnID: conn.ID, Name: "Slave2", SlaveAddr: 2}
	s.CreateSlave(slave2)

	reg := &model.Register{ID: "regwrongsregregwrongsregregwron", SlaveID: slave1.ID, StartAddr: 40001, HexData: "1234"}
	s.CreateRegister(reg)

	// Try to get register using wrong slave ID
	req := httptest.NewRequest("GET", "/api/connections/"+conn.ID+"/slaves/"+slave2.ID+"/registers/"+reg.ID, nil)
	w := httptest.NewRecorder()

	h.GetRegister(w, req, conn.ID, slave2.ID, reg.ID)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// TestUpdateRegisterInvalidOddHex tests updating register with odd-length hex data
func TestUpdateRegisterInvalidOddHex(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "conninvalidhexconninvalidhexconni", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slaveinvalidhexslaveinvalidhexsl", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "reginvalidhexreginvalidhexreginv", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"}
	s.CreateRegister(reg)

	// Invalid hex data (odd length)
	body := bytes.NewBufferString(`{"hexData":"ABC"}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers/"+reg.ID, body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateRegister(w, req, conn.ID, slave.ID, reg.ID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// TestCreateRegisterEmptyHex tests creating register with empty hex data
func TestCreateRegisterEmptyHex(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connemptyhexconnemptyhexconnempty", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slaveemptyhexslaveemptyhexslaveem", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	// Empty hex data
	body := bytes.NewBufferString(`{"startAddr":40001,"hexData":""}`)
	req := httptest.NewRequest("POST", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateRegister(w, req, conn.ID, slave.ID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// TestCreateRegisterNonHex tests creating register with non-hex characters
func TestCreateRegisterNonHex(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	conn := &model.Connection{ID: "connnonhexconnnonhexconnnonhexco", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slavenonhexslavenonhexslavenonhe", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	// Non-hex characters
	body := bytes.NewBufferString(`{"startAddr":40001,"hexData":"GHXY"}`)
	req := httptest.NewRequest("POST", "/api/connections/"+conn.ID+"/slaves/"+slave.ID+"/registers", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateRegister(w, req, conn.ID, slave.ID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// TestReadBitsEdgeCases tests edge cases in readBits
func TestReadBitsEdgeCases(t *testing.T) {
	s := store.New()
	conn := &model.Connection{ID: "conedgecaseconedgecaseconedgecase", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slaveedgecaseslaveedgecaseslave", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	mbHandler := NewModbusHandler(s, conn.ID)

	// Create a coil with only some bits set
	reg := &model.Register{ID: "regedgecaseregedgecaseregedgeca", SlaveID: slave.ID, StartAddr: 1, HexData: "01"} // Only first bit set
	s.CreateRegister(reg)

	// Read more bits than available (should return zeros for unmapped)
	data, err := mbHandler.ReadCoils(1, 0, 16)
	if err != nil {
		t.Fatalf("ReadCoils failed: %v", err)
	}

	// Should have 2 bytes (16 bits = 2 bytes)
	if len(data) != 2 {
		t.Errorf("Data length = %d, want 2", len(data))
	}

	// First byte should be 0x01, second byte should be 0x00
	if data[0] != 0x01 {
		t.Errorf("First byte = 0x%02X, want 0x01", data[0])
	}
	if data[1] != 0x00 {
		t.Errorf("Second byte = 0x%02X, want 0x00", data[1])
	}
}

// TestReadRegistersEdgeCases tests edge cases in readRegisters
func TestReadRegistersEdgeCases(t *testing.T) {
	s := store.New()
	conn := &model.Connection{ID: "connregedgeconnregedgeconnregedgec", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "slaveregedgeslaveregedgeslavereg", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	mbHandler := NewModbusHandler(s, conn.ID)

	// Create a register with only one register value
	reg := &model.Register{ID: "regregedgeregregedgeregregedger", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"} // Only 1 register
	s.CreateRegister(reg)

	// Read more registers than available (should return zeros for unmapped)
	data, err := mbHandler.ReadHoldingRegisters(1, 0, 3)
	if err != nil {
		t.Fatalf("ReadHoldingRegisters failed: %v", err)
	}

	// Should have 6 bytes (3 registers * 2 bytes)
	if len(data) != 6 {
		t.Errorf("Data length = %d, want 6", len(data))
	}

	// First register should be 0x1234
	if data[0] != 0x12 || data[1] != 0x34 {
		t.Errorf("First register = 0x%02X%02X, want 0x1234", data[0], data[1])
	}

	// Second and third registers should be 0x0000 (unmapped)
	if data[2] != 0x00 || data[3] != 0x00 {
		t.Errorf("Second register = 0x%02X%02X, want 0x0000", data[2], data[3])
	}
	if data[4] != 0x00 || data[5] != 0x00 {
		t.Errorf("Third register = 0x%02X%02X, want 0x0000", data[4], data[5])
	}
}
