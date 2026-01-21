package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/whysmx/modbus-simulator-go/internal/handler"
	"github.com/whysmx/modbus-simulator-go/internal/model"
	"github.com/whysmx/modbus-simulator-go/internal/store"
)

// mockTCPServer implements handler.TCPServerInterface for testing
type mockTCPServer struct{}

func (m *mockTCPServer) StartListener(conn *model.Connection) error { return nil }
func (m *mockTCPServer) StopListener(port int) error                { return nil }
func (m *mockTCPServer) UpdateListener(oldPort int, conn *model.Connection) error {
	return nil
}

func setupRouter() *Router {
	s := store.New()
	tcp := &mockTCPServer{}
	api := handler.NewAPIHandler(s, tcp, 1502)
	return NewRouter(api, nil)
}

func TestNewRouter(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	api := handler.NewAPIHandler(s, tcp, 1502)

	router := NewRouter(api, nil)
	if router == nil {
		t.Fatal("NewRouter returned nil")
	}
	if router.apiHandler != api {
		t.Error("apiHandler not set correctly")
	}
}

func TestRouterCORS(t *testing.T) {
	router := setupRouter()

	req := httptest.NewRequest("OPTIONS", "/api/connections", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("OPTIONS status = %d, want %d", w.Code, http.StatusOK)
	}

	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("CORS origin = %s, want *", origin)
	}

	if methods := w.Header().Get("Access-Control-Allow-Methods"); methods == "" {
		t.Error("CORS methods header not set")
	}
}

func TestRouterGetConnectionsTree(t *testing.T) {
	router := setupRouter()

	req := httptest.NewRequest("GET", "/api/connections/tree", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRouterCreateConnection(t *testing.T) {
	router := setupRouter()

	body := bytes.NewBufferString(`{"name":"Test","port":1502}`)
	req := httptest.NewRequest("POST", "/api/connections", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestRouterUpdateConnection(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	api := handler.NewAPIHandler(s, tcp, 1502)
	router := NewRouter(api, nil)

	// Create connection first - use hex chars only (a-f, 0-9)
	conn := &model.Connection{ID: "aaaabbbbccccdddd1111222233334444", Name: "Test", Port: 1502}
	s.CreateConnection(conn)

	body := bytes.NewBufferString(`{"name":"Updated","port":1503}`)
	req := httptest.NewRequest("PUT", "/api/connections/aaaabbbbccccdddd1111222233334444", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestRouterDeleteConnection(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	api := handler.NewAPIHandler(s, tcp, 1502)
	router := NewRouter(api, nil)

	conn := &model.Connection{ID: "dddd1111222233334444aaaabbbbcccc", Name: "Test", Port: 1502}
	s.CreateConnection(conn)

	req := httptest.NewRequest("DELETE", "/api/connections/dddd1111222233334444aaaabbbbcccc", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestRouterCreateSlave(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	api := handler.NewAPIHandler(s, tcp, 1502)
	router := NewRouter(api, nil)

	conn := &model.Connection{ID: "1111222233334444aaaabbbbccccdddd", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	body := bytes.NewBufferString(`{"name":"Slave1","slaveAddr":1}`)
	req := httptest.NewRequest("POST", "/api/connections/1111222233334444aaaabbbbccccdddd/slaves", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestRouterUpdateSlave(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	api := handler.NewAPIHandler(s, tcp, 1502)
	router := NewRouter(api, nil)

	conn := &model.Connection{ID: "2222333344445555aaaabbbbccccdddd", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "5555444433332222ddddccccbbbbaaaa", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	body := bytes.NewBufferString(`{"name":"Updated","slaveAddr":2}`)
	req := httptest.NewRequest("PUT", "/api/connections/2222333344445555aaaabbbbccccdddd/slaves/5555444433332222ddddccccbbbbaaaa", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestRouterDeleteSlave(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	api := handler.NewAPIHandler(s, tcp, 1502)
	router := NewRouter(api, nil)

	conn := &model.Connection{ID: "3333444455556666aaaabbbbccccdddd", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "6666555544443333ddddccccbbbbaaaa", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	req := httptest.NewRequest("DELETE", "/api/connections/3333444455556666aaaabbbbccccdddd/slaves/6666555544443333ddddccccbbbbaaaa", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestRouterGetRegisters(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	api := handler.NewAPIHandler(s, tcp, 1502)
	router := NewRouter(api, nil)

	conn := &model.Connection{ID: "4444555566667777aaaabbbbccccdddd", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "7777666655554444ddddccccbbbbaaaa", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	req := httptest.NewRequest("GET", "/api/connections/4444555566667777aaaabbbbccccdddd/slaves/7777666655554444ddddccccbbbbaaaa/registers", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRouterCreateRegister(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	api := handler.NewAPIHandler(s, tcp, 1502)
	router := NewRouter(api, nil)

	conn := &model.Connection{ID: "aabbccdd11223344aabbccdd11223344", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "11223344aabbccdd11223344aabbccdd", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	body := bytes.NewBufferString(`{"startAddr":40001,"hexData":"1234"}`)
	req := httptest.NewRequest("POST", "/api/connections/aabbccdd11223344aabbccdd11223344/slaves/11223344aabbccdd11223344aabbccdd/registers", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestRouterUpdateRegister(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	api := handler.NewAPIHandler(s, tcp, 1502)
	router := NewRouter(api, nil)

	conn := &model.Connection{ID: "aabbccdd11223344aabbccdd11223355", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "11223344aabbccdd11223344aabbcc55", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "55443322aabbccdd55443322aabbccdd", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"}
	s.CreateRegister(reg)

	body := bytes.NewBufferString(`{"startAddr":40001,"hexData":"ABCD"}`)
	req := httptest.NewRequest("PUT", "/api/connections/aabbccdd11223344aabbccdd11223355/slaves/11223344aabbccdd11223344aabbcc55/registers/55443322aabbccdd55443322aabbccdd", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestRouterDeleteRegister(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	api := handler.NewAPIHandler(s, tcp, 1502)
	router := NewRouter(api, nil)

	conn := &model.Connection{ID: "aabbccdd11223344aabbccdd11223366", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)
	slave := &model.Slave{ID: "11223344aabbccdd11223344aabbcc66", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)
	reg := &model.Register{ID: "66443322aabbccdd66443322aabbccdd", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"}
	s.CreateRegister(reg)

	req := httptest.NewRequest("DELETE", "/api/connections/aabbccdd11223344aabbccdd11223366/slaves/11223344aabbccdd11223344aabbcc66/registers/66443322aabbccdd66443322aabbccdd", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestRouterNotFound(t *testing.T) {
	router := setupRouter()

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestRouterInvalidMethod(t *testing.T) {
	router := setupRouter()

	// GET on /api/connections should not match (only POST is supported)
	req := httptest.NewRequest("GET", "/api/connections", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should fall through to 404
	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
