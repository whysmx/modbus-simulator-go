package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/whysmx/modbus-simulator-go/internal/model"
	"github.com/whysmx/modbus-simulator-go/internal/store"
)

// TCPServerInterface defines the methods needed from TCP server
type TCPServerInterface interface {
	StartListener(conn *model.Connection) error
	StopListener(port int) error
	UpdateListener(oldPort int, conn *model.Connection) error
}

// APIHandler handles HTTP API requests
type APIHandler struct {
	store     *store.Store
	tcpServer TCPServerInterface
	nextPort  int
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(s *store.Store, tcp TCPServerInterface, startPort int) *APIHandler {
	return &APIHandler{
		store:     s,
		tcpServer: tcp,
		nextPort:  startPort,
	}
}

// generateUUID generates a 32-character UUID without hyphens
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// TreeNode represents a node in the device tree
type TreeNode struct {
	Connection *model.Connection `json:"connection"`
	Slaves     []*model.Slave    `json:"slaves"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}

func (h *APIHandler) isDuplicateSlaveName(name, excludeSlaveID string) bool {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return false
	}
	for _, conn := range h.store.GetAllConnections() {
		slaves := h.store.GetSlavesByConnection(conn.ID)
		for _, slave := range slaves {
			if slave.ID == excludeSlaveID {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(slave.Name), normalized) {
				return true
			}
		}
	}
	return false
}

// GetConnectionsTree returns all connections with their slaves
func (h *APIHandler) GetConnectionsTree(w http.ResponseWriter, r *http.Request) {
	connections := h.store.GetAllConnections()
	tree := make([]TreeNode, 0, len(connections))

	for _, conn := range connections {
		slaves := h.store.GetSlavesByConnection(conn.ID)
		tree = append(tree, TreeNode{
			Connection: conn,
			Slaves:     slaves,
		})
	}

	writeJSON(w, http.StatusOK, tree)
}

// CreateConnection creates a new connection
func (h *APIHandler) CreateConnection(w http.ResponseWriter, r *http.Request) {
	var conn model.Connection
	if err := json.NewDecoder(r.Body).Decode(&conn); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	conn.ID = generateUUID()
	if conn.Port == 0 {
		conn.Port = h.nextPort
		h.nextPort++
	}
	conn.ProtocolType = model.ModbusAuto

	if err := h.store.CreateConnection(&conn); err != nil {
		if err == store.ErrPortInUse {
			writeError(w, http.StatusBadRequest, "port already in use")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Start TCP listener
	if err := h.tcpServer.StartListener(&conn); err != nil {
		// Rollback
		h.store.DeleteConnection(conn.ID)
		writeError(w, http.StatusBadRequest, "failed to start listener: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, conn)
}

// UpdateConnection updates an existing connection
func (h *APIHandler) UpdateConnection(w http.ResponseWriter, r *http.Request, id string) {
	oldConn, err := h.store.GetConnection(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}

	var conn model.Connection
	if err := json.NewDecoder(r.Body).Decode(&conn); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	conn.ID = id
	oldPort := oldConn.Port
	conn.ProtocolType = model.ModbusAuto

	if err := h.store.UpdateConnection(&conn); err != nil {
		if err == store.ErrPortInUse {
			writeError(w, http.StatusBadRequest, "port already in use")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Restart listener if port changed
	if oldPort != conn.Port {
		h.tcpServer.UpdateListener(oldPort, &conn)
	}

	writeJSON(w, http.StatusOK, conn)
}

// DeleteConnection deletes a connection
func (h *APIHandler) DeleteConnection(w http.ResponseWriter, r *http.Request, id string) {
	conn, err := h.store.GetConnection(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}

	h.tcpServer.StopListener(conn.Port)

	if err := h.store.DeleteConnection(id); err != nil {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateSlave creates a new slave under a connection
func (h *APIHandler) CreateSlave(w http.ResponseWriter, r *http.Request, connID string) {
	if _, err := h.store.GetConnection(connID); err != nil {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}

	var slave model.Slave
	if err := json.NewDecoder(r.Body).Decode(&slave); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	slave.ID = generateUUID()
	slave.ConnID = connID

	if slave.SlaveAddr < 1 || slave.SlaveAddr > 247 {
		writeError(w, http.StatusBadRequest, "slaveAddr must be 1-247")
		return
	}

	if h.isDuplicateSlaveName(slave.Name, "") {
		writeError(w, http.StatusBadRequest, "设备名称不能重复")
		return
	}

	if err := h.store.CreateSlave(&slave); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, slave)
}

// UpdateSlave updates an existing slave
func (h *APIHandler) UpdateSlave(w http.ResponseWriter, r *http.Request, connID, slaveID string) {
	if _, err := h.store.GetConnection(connID); err != nil {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}

	existing, err := h.store.GetSlave(slaveID)
	if err != nil || existing.ConnID != connID {
		writeError(w, http.StatusNotFound, "slave not found")
		return
	}

	var slave model.Slave
	if err := json.NewDecoder(r.Body).Decode(&slave); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	slave.ID = slaveID
	slave.ConnID = connID

	if slave.SlaveAddr < 1 || slave.SlaveAddr > 247 {
		writeError(w, http.StatusBadRequest, "slaveAddr must be 1-247")
		return
	}

	if h.isDuplicateSlaveName(slave.Name, slaveID) {
		writeError(w, http.StatusBadRequest, "设备名称不能重复")
		return
	}

	if err := h.store.UpdateSlave(&slave); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, slave)
}

// DeleteSlave deletes a slave
func (h *APIHandler) DeleteSlave(w http.ResponseWriter, r *http.Request, connID, slaveID string) {
	existing, err := h.store.GetSlave(slaveID)
	if err != nil || existing.ConnID != connID {
		writeError(w, http.StatusNotFound, "slave not found")
		return
	}

	if err := h.store.DeleteSlave(slaveID); err != nil {
		writeError(w, http.StatusNotFound, "slave not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetRegisters returns all registers for a slave
func (h *APIHandler) GetRegisters(w http.ResponseWriter, r *http.Request, connID, slaveID string) {
	existing, err := h.store.GetSlave(slaveID)
	if err != nil || existing.ConnID != connID {
		writeError(w, http.StatusNotFound, "slave not found")
		return
	}

	registers := h.store.GetRegistersBySlave(slaveID)
	writeJSON(w, http.StatusOK, registers)
}

// GetRegister returns a single register
func (h *APIHandler) GetRegister(w http.ResponseWriter, r *http.Request, connID, slaveID, regID string) {
	slave, err := h.store.GetSlave(slaveID)
	if err != nil || slave.ConnID != connID {
		writeError(w, http.StatusNotFound, "slave not found")
		return
	}

	reg, err := h.store.GetRegister(regID)
	if err != nil || reg.SlaveID != slaveID {
		writeError(w, http.StatusNotFound, "register not found")
		return
	}

	writeJSON(w, http.StatusOK, reg)
}

// isValidHexData checks if hexData is valid
func isValidHexData(hexData string, startAddr int) bool {
	if len(hexData) == 0 {
		return false
	}
	// Must be uppercase hex only
	matched, _ := regexp.MatchString("^[0-9A-F]+$", hexData)
	if !matched {
		return false
	}
	// Check length based on register type
	regType := model.GetRegisterType(startAddr)
	switch regType {
	case model.Coil, model.DiscreteInput:
		return len(hexData)%2 == 0
	case model.InputRegister, model.HoldingRegister:
		return len(hexData)%4 == 0
	}
	return false
}

func normalizeJitterAmp(amp int) int {
	if amp < 0 {
		return 0
	}
	if amp > model.MaxJitterAmp {
		return model.MaxJitterAmp
	}
	return amp
}

// CreateRegister creates a new register
func (h *APIHandler) CreateRegister(w http.ResponseWriter, r *http.Request, connID, slaveID string) {
	existing, err := h.store.GetSlave(slaveID)
	if err != nil || existing.ConnID != connID {
		writeError(w, http.StatusNotFound, "slave not found")
		return
	}

	var reg model.Register
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	reg.ID = generateUUID()
	reg.SlaveID = slaveID
	reg.HexData = strings.ToUpper(reg.HexData)
	reg.JitterAmp = normalizeJitterAmp(reg.JitterAmp)

	if !model.IsValidAddress(reg.StartAddr) {
		writeError(w, http.StatusBadRequest, "invalid startAddr")
		return
	}

	if !isValidHexData(reg.HexData, reg.StartAddr) {
		writeError(w, http.StatusBadRequest, "invalid hexData")
		return
	}

	// Check for address overlap
	existingRegs := h.store.GetRegistersBySlave(slaveID)
	for _, existing := range existingRegs {
		if reg.Overlaps(existing) {
			writeError(w, http.StatusBadRequest, "address overlap with existing register")
			return
		}
	}

	if err := h.store.CreateRegister(&reg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, reg)
}

// UpdateRegister updates an existing register
func (h *APIHandler) UpdateRegister(w http.ResponseWriter, r *http.Request, connID, slaveID, regID string) {
	slave, err := h.store.GetSlave(slaveID)
	if err != nil || slave.ConnID != connID {
		writeError(w, http.StatusNotFound, "slave not found")
		return
	}

	existingReg, err := h.store.GetRegister(regID)
	if err != nil || existingReg.SlaveID != slaveID {
		writeError(w, http.StatusNotFound, "register not found")
		return
	}

	var reg model.Register
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	reg.ID = regID
	reg.SlaveID = slaveID
	reg.HexData = strings.ToUpper(reg.HexData)
	reg.JitterAmp = normalizeJitterAmp(reg.JitterAmp)

	if !model.IsValidAddress(reg.StartAddr) {
		writeError(w, http.StatusBadRequest, "invalid startAddr")
		return
	}

	if !isValidHexData(reg.HexData, reg.StartAddr) {
		writeError(w, http.StatusBadRequest, "invalid hexData")
		return
	}

	// Check for address overlap (excluding self)
	existingRegs := h.store.GetRegistersBySlave(slaveID)
	for _, existing := range existingRegs {
		if existing.ID != regID && reg.Overlaps(existing) {
			writeError(w, http.StatusBadRequest, "address overlap with existing register")
			return
		}
	}

	if err := h.store.UpdateRegister(&reg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, reg)
}

// DeleteRegister deletes a register
func (h *APIHandler) DeleteRegister(w http.ResponseWriter, r *http.Request, connID, slaveID, regID string) {
	slave, err := h.store.GetSlave(slaveID)
	if err != nil || slave.ConnID != connID {
		writeError(w, http.StatusNotFound, "slave not found")
		return
	}

	existingReg, err := h.store.GetRegister(regID)
	if err != nil || existingReg.SlaveID != slaveID {
		writeError(w, http.StatusNotFound, "register not found")
		return
	}

	if err := h.store.DeleteRegister(regID); err != nil {
		writeError(w, http.StatusNotFound, "register not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
