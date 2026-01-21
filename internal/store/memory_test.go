package store

import (
	"testing"

	"github.com/whysmx/modbus-simulator-go/internal/model"
)

func TestCreateConnection(t *testing.T) {
	s := New()

	conn := &model.Connection{
		ID:           "abc123def456abc123def456abc12345",
		Name:         "Test Connection",
		Port:         1502,
		ProtocolType: model.ModbusRtuOverTcp,
	}

	err := s.CreateConnection(conn)
	if err != nil {
		t.Fatalf("CreateConnection failed: %v", err)
	}

	// Verify retrieval
	got, err := s.GetConnection(conn.ID)
	if err != nil {
		t.Fatalf("GetConnection failed: %v", err)
	}
	if got.Name != conn.Name {
		t.Errorf("Name = %s, want %s", got.Name, conn.Name)
	}
}

func TestPortInUse(t *testing.T) {
	s := New()

	conn1 := &model.Connection{
		ID:   "id1id1id1id1id1id1id1id1id1id1id",
		Name: "Connection 1",
		Port: 1502,
	}
	conn2 := &model.Connection{
		ID:   "id2id2id2id2id2id2id2id2id2id2id",
		Name: "Connection 2",
		Port: 1502,
	}

	s.CreateConnection(conn1)
	err := s.CreateConnection(conn2)
	if err != ErrPortInUse {
		t.Errorf("Expected ErrPortInUse, got %v", err)
	}
}

func TestDeleteConnectionCascade(t *testing.T) {
	s := New()

	// Create connection
	conn := &model.Connection{
		ID:   "connidconnidconnidconnidconnidco",
		Name: "Test",
		Port: 1502,
	}
	s.CreateConnection(conn)

	// Create slave
	slave := &model.Slave{
		ID:        "slaveidslaveidslaveidslaveidsla",
		ConnID:    conn.ID,
		Name:      "Slave 1",
		SlaveAddr: 1,
	}
	s.CreateSlave(slave)

	// Create register
	reg := &model.Register{
		ID:        "regidregidregidregidregidregidr",
		SlaveID:   slave.ID,
		StartAddr: 40001,
		HexData:   "1234",
	}
	s.CreateRegister(reg)

	// Delete connection - should cascade
	err := s.DeleteConnection(conn.ID)
	if err != nil {
		t.Fatalf("DeleteConnection failed: %v", err)
	}

	// Verify slave is gone
	_, err = s.GetSlave(slave.ID)
	if err != ErrNotFound {
		t.Error("Expected slave to be deleted")
	}

	// Verify register is gone
	_, err = s.GetRegister(reg.ID)
	if err != ErrNotFound {
		t.Error("Expected register to be deleted")
	}
}

func TestUpdateConnection(t *testing.T) {
	s := New()

	conn := &model.Connection{
		ID:   "updatetestupdatetestupdatetestup",
		Name: "Original",
		Port: 1502,
	}
	s.CreateConnection(conn)

	// Update
	conn.Name = "Updated"
	conn.Port = 1503
	err := s.UpdateConnection(conn)
	if err != nil {
		t.Fatalf("UpdateConnection failed: %v", err)
	}

	got, _ := s.GetConnection(conn.ID)
	if got.Name != "Updated" {
		t.Errorf("Name = %s, want Updated", got.Name)
	}
	if got.Port != 1503 {
		t.Errorf("Port = %d, want 1503", got.Port)
	}
}

func TestGetSlaveByAddress(t *testing.T) {
	s := New()

	conn := &model.Connection{
		ID:   "connforslaveconnforslaveconnfors",
		Name: "Conn",
		Port: 1502,
	}
	s.CreateConnection(conn)

	slave := &model.Slave{
		ID:        "slave1slave1slave1slave1slave1sl",
		ConnID:    conn.ID,
		Name:      "Slave",
		SlaveAddr: 5,
	}
	s.CreateSlave(slave)

	// Find by address
	got, err := s.GetSlaveByAddress(conn.ID, 5)
	if err != nil {
		t.Fatalf("GetSlaveByAddress failed: %v", err)
	}
	if got.ID != slave.ID {
		t.Errorf("ID = %s, want %s", got.ID, slave.ID)
	}

	// Not found
	_, err = s.GetSlaveByAddress(conn.ID, 99)
	if err != ErrNotFound {
		t.Error("Expected ErrNotFound for non-existent address")
	}
}

func TestGetAllConnections(t *testing.T) {
	s := New()

	conn1 := &model.Connection{ID: "conn1conn1conn1conn1conn1conn1co", Name: "C1", Port: 1502}
	conn2 := &model.Connection{ID: "conn2conn2conn2conn2conn2conn2co", Name: "C2", Port: 1503}

	s.CreateConnection(conn1)
	s.CreateConnection(conn2)

	all := s.GetAllConnections()
	if len(all) != 2 {
		t.Errorf("Expected 2 connections, got %d", len(all))
	}
}

func TestGetSlavesByConnection(t *testing.T) {
	s := New()

	conn := &model.Connection{ID: "parentconnparentconnparentconnpa", Name: "Parent", Port: 1502}
	s.CreateConnection(conn)

	slave1 := &model.Slave{ID: "s1s1s1s1s1s1s1s1s1s1s1s1s1s1s1s1", ConnID: conn.ID, Name: "S1", SlaveAddr: 1}
	slave2 := &model.Slave{ID: "s2s2s2s2s2s2s2s2s2s2s2s2s2s2s2s2", ConnID: conn.ID, Name: "S2", SlaveAddr: 2}

	s.CreateSlave(slave1)
	s.CreateSlave(slave2)

	slaves := s.GetSlavesByConnection(conn.ID)
	if len(slaves) != 2 {
		t.Errorf("Expected 2 slaves, got %d", len(slaves))
	}
}

func TestGetRegistersBySlave(t *testing.T) {
	s := New()

	conn := &model.Connection{ID: "regconnregconnregconnregconnregc", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	slave := &model.Slave{ID: "regslaveregslaveregslaveregslave", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	reg1 := &model.Register{ID: "r1r1r1r1r1r1r1r1r1r1r1r1r1r1r1r1", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"}
	reg2 := &model.Register{ID: "r2r2r2r2r2r2r2r2r2r2r2r2r2r2r2r2", SlaveID: slave.ID, StartAddr: 40002, HexData: "5678"}

	s.CreateRegister(reg1)
	s.CreateRegister(reg2)

	regs := s.GetRegistersBySlave(slave.ID)
	if len(regs) != 2 {
		t.Errorf("Expected 2 registers, got %d", len(regs))
	}
}

func TestUpdateRegister(t *testing.T) {
	s := New()

	conn := &model.Connection{ID: "upregconnupregconnupregconnupreg", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	slave := &model.Slave{ID: "upregslaveupregslaveupregslaveup", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	reg := &model.Register{ID: "upregupregupregupregupregupregup", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"}
	s.CreateRegister(reg)

	reg.HexData = "ABCD"
	err := s.UpdateRegister(reg)
	if err != nil {
		t.Fatalf("UpdateRegister failed: %v", err)
	}

	got, _ := s.GetRegister(reg.ID)
	if got.HexData != "ABCD" {
		t.Errorf("HexData = %s, want ABCD", got.HexData)
	}
}
