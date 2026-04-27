package store

import (
	"database/sql"
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/whysmx/modbus-simulator-go/internal/model"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrPortInUse     = errors.New("port already in use")
	ErrNameInUse     = errors.New("connection name already exists")
)

// Store provides thread-safe in-memory storage for Modbus and private protocol entities
type Store struct {
	mu               sync.RWMutex
	db               *sql.DB
	connections      map[string]*model.Connection
	slaves           map[string]*model.Slave
	registers        map[string]*model.Register
	privateProtocols map[string]*model.PrivateProtocol // keyed by connID

	// Index for quick lookups
	portToConnID   map[int]string
	connIDToSlaves map[string][]string
	slaveToRegs    map[string][]string
}

// New creates a new in-memory store
func New() *Store {
	return &Store{
		connections:      make(map[string]*model.Connection),
		slaves:           make(map[string]*model.Slave),
		registers:        make(map[string]*model.Register),
		privateProtocols: make(map[string]*model.PrivateProtocol),
		portToConnID:     make(map[int]string),
		connIDToSlaves:   make(map[string][]string),
		slaveToRegs:      make(map[string][]string),
	}
}

// Connection operations

func (s *Store) CreateConnection(conn *model.Connection) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isDuplicateConnectionNameLocked(conn.Name, "") {
		return ErrNameInUse
	}

	if _, exists := s.portToConnID[conn.Port]; exists {
		return ErrPortInUse
	}

	if s.db != nil {
		if err := s.insertConnection(conn); err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed: connections.port") {
				return ErrPortInUse
			}
			return err
		}
	}

	s.connections[conn.ID] = conn
	s.portToConnID[conn.Port] = conn.ID
	return nil
}

func (s *Store) GetConnection(id string) (*model.Connection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conn, ok := s.connections[id]
	if !ok {
		return nil, ErrNotFound
	}
	return conn, nil
}

func (s *Store) GetAllConnections() []*model.Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*model.Connection, 0, len(s.connections))
	for _, conn := range s.connections {
		result = append(result, conn)
	}
	sort.Slice(result, func(i, j int) bool {
		if cmp := compareDisplayName(result[i].Name, result[j].Name); cmp != 0 {
			return cmp < 0
		}
		if result[i].Port != result[j].Port {
			return result[i].Port < result[j].Port
		}
		return result[i].ID < result[j].ID
	})
	return result
}

func (s *Store) UpdateConnection(conn *model.Connection) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	old, ok := s.connections[conn.ID]
	if !ok {
		return ErrNotFound
	}

	if s.isDuplicateConnectionNameLocked(conn.Name, conn.ID) {
		return ErrNameInUse
	}

	// Check if new port is available (if changed)
	if old.Port != conn.Port {
		if existingID, exists := s.portToConnID[conn.Port]; exists && existingID != conn.ID {
			return ErrPortInUse
		}
		delete(s.portToConnID, old.Port)
		s.portToConnID[conn.Port] = conn.ID
	}

	if s.db != nil {
		if err := s.updateConnection(conn); err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed: connections.port") {
				return ErrPortInUse
			}
			return err
		}
	}

	s.connections[conn.ID] = conn
	return nil
}

func (s *Store) isDuplicateConnectionNameLocked(name, excludeConnID string) bool {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return false
	}
	for _, conn := range s.connections {
		if conn.ID == excludeConnID {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(conn.Name), normalized) {
			return true
		}
	}
	return false
}

func (s *Store) DeleteConnection(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn, ok := s.connections[id]
	if !ok {
		return ErrNotFound
	}

	if s.db != nil {
		if err := s.deleteConnection(id); err != nil {
			return err
		}
	}

	// Delete all slaves and their registers
	for _, slaveID := range s.connIDToSlaves[id] {
		for _, regID := range s.slaveToRegs[slaveID] {
			delete(s.registers, regID)
		}
		delete(s.slaveToRegs, slaveID)
		delete(s.slaves, slaveID)
	}
	delete(s.connIDToSlaves, id)
	delete(s.privateProtocols, id)

	delete(s.portToConnID, conn.Port)
	delete(s.connections, id)
	return nil
}

// Private protocol operations

func (s *Store) UpsertPrivateProtocol(pp *model.PrivateProtocol) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.connections[pp.ConnID]; !ok {
		return ErrNotFound
	}

	if s.db != nil {
		if err := s.upsertPrivateProtocol(pp); err != nil {
			return err
		}
	}

	s.privateProtocols[pp.ConnID] = pp
	return nil
}

func (s *Store) GetPrivateProtocol(connID string) (*model.PrivateProtocol, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pp, ok := s.privateProtocols[connID]
	if !ok {
		return nil, ErrNotFound
	}
	return pp, nil
}

func (s *Store) DeletePrivateProtocol(connID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.privateProtocols[connID]; !ok {
		return ErrNotFound
	}

	if s.db != nil {
		if err := s.deletePrivateProtocol(connID); err != nil {
			return err
		}
	}

	delete(s.privateProtocols, connID)
	return nil
}

// Slave operations

func (s *Store) CreateSlave(slave *model.Slave) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.connections[slave.ConnID]; !ok {
		return ErrNotFound
	}

	if s.db != nil {
		if err := s.insertSlave(slave); err != nil {
			return err
		}
	}

	s.slaves[slave.ID] = slave
	s.connIDToSlaves[slave.ConnID] = append(s.connIDToSlaves[slave.ConnID], slave.ID)
	return nil
}

func (s *Store) GetSlave(id string) (*model.Slave, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	slave, ok := s.slaves[id]
	if !ok {
		return nil, ErrNotFound
	}
	return slave, nil
}

func (s *Store) GetSlavesByConnection(connID string) []*model.Slave {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.connIDToSlaves[connID]
	result := make([]*model.Slave, 0, len(ids))
	for _, id := range ids {
		if slave, ok := s.slaves[id]; ok {
			result = append(result, slave)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if cmp := compareDisplayName(result[i].Name, result[j].Name); cmp != 0 {
			return cmp < 0
		}
		if result[i].SlaveAddr != result[j].SlaveAddr {
			return result[i].SlaveAddr < result[j].SlaveAddr
		}
		return result[i].ID < result[j].ID
	})
	return result
}

func (s *Store) DeleteSlave(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	slave, ok := s.slaves[id]
	if !ok {
		return ErrNotFound
	}

	if s.db != nil {
		if err := s.deleteSlave(id); err != nil {
			return err
		}
	}

	// Delete all registers
	for _, regID := range s.slaveToRegs[id] {
		delete(s.registers, regID)
	}
	delete(s.slaveToRegs, id)

	// Remove from connection index
	connSlaves := s.connIDToSlaves[slave.ConnID]
	for i, sid := range connSlaves {
		if sid == id {
			s.connIDToSlaves[slave.ConnID] = append(connSlaves[:i], connSlaves[i+1:]...)
			break
		}
	}

	delete(s.slaves, id)
	return nil
}

// Register operations

func (s *Store) CreateRegister(reg *model.Register) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.slaves[reg.SlaveID]; !ok {
		return ErrNotFound
	}

	if s.db != nil {
		if err := s.insertRegister(reg); err != nil {
			return err
		}
	}

	s.registers[reg.ID] = reg
	s.slaveToRegs[reg.SlaveID] = append(s.slaveToRegs[reg.SlaveID], reg.ID)
	return nil
}

func (s *Store) GetRegister(id string) (*model.Register, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	reg, ok := s.registers[id]
	if !ok {
		return nil, ErrNotFound
	}
	return reg, nil
}

func (s *Store) GetRegistersBySlave(slaveID string) []*model.Register {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.slaveToRegs[slaveID]
	result := make([]*model.Register, 0, len(ids))
	for _, id := range ids {
		if reg, ok := s.registers[id]; ok {
			result = append(result, reg)
		}
	}
	return result
}

func (s *Store) UpdateRegister(reg *model.Register) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.registers[reg.ID]; !ok {
		return ErrNotFound
	}

	if s.db != nil {
		if err := s.updateRegister(reg); err != nil {
			return err
		}
	}

	s.registers[reg.ID] = reg
	return nil
}

func (s *Store) DeleteRegister(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	reg, ok := s.registers[id]
	if !ok {
		return ErrNotFound
	}

	if s.db != nil {
		if err := s.deleteRegister(id); err != nil {
			return err
		}
	}

	// Remove from slave index
	slaveRegs := s.slaveToRegs[reg.SlaveID]
	for i, rid := range slaveRegs {
		if rid == id {
			s.slaveToRegs[reg.SlaveID] = append(slaveRegs[:i], slaveRegs[i+1:]...)
			break
		}
	}

	delete(s.registers, id)
	return nil
}

// GetConnectionByPort returns the connection using a specific port
func (s *Store) GetConnectionByPort(port int) (*model.Connection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	connID, ok := s.portToConnID[port]
	if !ok {
		return nil, ErrNotFound
	}
	return s.connections[connID], nil
}

// GetSlaveByAddress finds a slave by connection ID and slave address
func (s *Store) GetSlaveByAddress(connID string, slaveAddr int) (*model.Slave, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, id := range s.connIDToSlaves[connID] {
		if slave, ok := s.slaves[id]; ok && slave.SlaveAddr == slaveAddr {
			return slave, nil
		}
	}
	return nil, ErrNotFound
}

// UpdateSlave updates an existing slave
func (s *Store) UpdateSlave(slave *model.Slave) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.slaves[slave.ID]; !ok {
		return ErrNotFound
	}

	if s.db != nil {
		if err := s.updateSlave(slave); err != nil {
			return err
		}
	}
	s.slaves[slave.ID] = slave
	return nil
}
