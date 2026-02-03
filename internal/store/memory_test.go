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

func TestDeleteSlave(t *testing.T) {
	s := New()

	conn := &model.Connection{ID: "delslavconndelslaveconndelslav01", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	slave := &model.Slave{ID: "slavetodelslavetodelslavetodel01", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	// Add register to slave
	reg := &model.Register{ID: "regindelregindelregindelreg0101", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"}
	s.CreateRegister(reg)

	// Delete slave
	err := s.DeleteSlave(slave.ID)
	if err != nil {
		t.Fatalf("DeleteSlave failed: %v", err)
	}

	// Verify slave is gone
	_, err = s.GetSlave(slave.ID)
	if err != ErrNotFound {
		t.Error("Expected slave to be deleted")
	}

	// Verify register is gone (cascade delete)
	_, err = s.GetRegister(reg.ID)
	if err != ErrNotFound {
		t.Error("Expected register to be deleted")
	}
}

func TestDeleteSlaveNotFound(t *testing.T) {
	s := New()

	err := s.DeleteSlave("nonexistentidnonexistentidnone1")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestDeleteRegister(t *testing.T) {
	s := New()

	conn := &model.Connection{ID: "delregconndelregconndelregconn01", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	slave := &model.Slave{ID: "delregslvdelregslvdelregslv01234", ConnID: conn.ID, Name: "Slave", SlaveAddr: 1}
	s.CreateSlave(slave)

	reg := &model.Register{ID: "regtodelregtodelregtodelreg01234", SlaveID: slave.ID, StartAddr: 40001, HexData: "1234"}
	s.CreateRegister(reg)

	err := s.DeleteRegister(reg.ID)
	if err != nil {
		t.Fatalf("DeleteRegister failed: %v", err)
	}

	_, err = s.GetRegister(reg.ID)
	if err != ErrNotFound {
		t.Error("Expected register to be deleted")
	}
}

func TestDeleteRegisterNotFound(t *testing.T) {
	s := New()

	err := s.DeleteRegister("nonexistentidnonexistentidnone1")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestGetConnectionByPort(t *testing.T) {
	s := New()

	conn := &model.Connection{ID: "connbyportconnbyportconnbyport01", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	// Found
	got, err := s.GetConnectionByPort(1502)
	if err != nil {
		t.Fatalf("GetConnectionByPort failed: %v", err)
	}
	if got.ID != conn.ID {
		t.Errorf("ID = %s, want %s", got.ID, conn.ID)
	}

	// Not found
	_, err = s.GetConnectionByPort(9999)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestUpdateSlave(t *testing.T) {
	s := New()

	conn := &model.Connection{ID: "upslaveconnupslaveconnupslavecon", Name: "Conn", Port: 1502}
	s.CreateConnection(conn)

	slave := &model.Slave{ID: "slavetoupdaslavetoupdaslavetoup", ConnID: conn.ID, Name: "Original", SlaveAddr: 1}
	s.CreateSlave(slave)

	// Use a NEW struct to update (not the same pointer)
	updateReq := &model.Slave{
		ID:        slave.ID,
		ConnID:    conn.ID,
		Name:      "Updated",
		SlaveAddr: 5,
	}
	err := s.UpdateSlave(updateReq)
	if err != nil {
		t.Fatalf("UpdateSlave failed: %v", err)
	}

	got, _ := s.GetSlave(slave.ID)
	if got.Name != "Updated" {
		t.Errorf("Name = %s, want Updated", got.Name)
	}
	if got.SlaveAddr != 5 {
		t.Errorf("SlaveAddr = %d, want 5", got.SlaveAddr)
	}
}

func TestUpdateSlaveNotFound(t *testing.T) {
	s := New()

	slave := &model.Slave{ID: "nonexistentidnonexistentidnonex", ConnID: "connid", Name: "Slave", SlaveAddr: 1}
	err := s.UpdateSlave(slave)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestUpdateRegisterNotFound(t *testing.T) {
	s := New()

	reg := &model.Register{ID: "nonexistentidnonexistentidnonex", SlaveID: "slaveid", StartAddr: 40001, HexData: "1234"}
	err := s.UpdateRegister(reg)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestUpdateConnectionNotFound(t *testing.T) {
	s := New()

	conn := &model.Connection{ID: "nonexistentidnonexistentidnonex", Name: "Conn", Port: 1502}
	err := s.UpdateConnection(conn)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestDeleteConnectionNotFound(t *testing.T) {
	s := New()

	err := s.DeleteConnection("nonexistentidnonexistentidnonex")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestGetSlaveNotFound(t *testing.T) {
	s := New()

	_, err := s.GetSlave("nonexistentidnonexistentidnonex")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestGetConnectionNotFound(t *testing.T) {
	s := New()

	_, err := s.GetConnection("nonexistentidnonexistentidnonex")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestCreateSlaveWithoutConnection(t *testing.T) {
	s := New()

	slave := &model.Slave{ID: "orphanslaveorphanslaveorphansla1", ConnID: "nonexistent", Name: "Slave", SlaveAddr: 1}
	err := s.CreateSlave(slave)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestCreateRegisterWithoutSlave(t *testing.T) {
	s := New()

	reg := &model.Register{ID: "orphanregorphanregorphanreg0001", SlaveID: "nonexistent", StartAddr: 40001, HexData: "1234"}
	err := s.CreateRegister(reg)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestUpdateConnectionPortConflict(t *testing.T) {
	s := New()

	conn1 := &model.Connection{ID: "portconflict1portconflict1portc1", Name: "Conn1", Port: 1502}
	conn2 := &model.Connection{ID: "portconflict2portconflict2portc2", Name: "Conn2", Port: 1503}

	s.CreateConnection(conn1)
	s.CreateConnection(conn2)

	// Try to update conn2 to use conn1's port using a new object (don't modify the original)
	updateReq := &model.Connection{ID: conn2.ID, Name: "Conn2", Port: 1502}
	err := s.UpdateConnection(updateReq)
	if err != ErrPortInUse {
		t.Errorf("Expected ErrPortInUse, got %v", err)
	}

	// Verify conn2 still has old port in store
	got, _ := s.GetConnection(conn2.ID)
	if got.Port != 1503 {
		t.Errorf("conn2 port = %d, want 1503 (unchanged)", got.Port)
	}
}

func TestUpdateConnectionPortChange(t *testing.T) {
	s := New()

	conn := &model.Connection{ID: "portchangeportchangeportchange01", Name: "Test", Port: 1502}
	s.CreateConnection(conn)

	// Update port using a new object with the same ID
	updateReq := &model.Connection{ID: conn.ID, Name: "Test", Port: 1600}
	err := s.UpdateConnection(updateReq)
	if err != nil {
		t.Fatalf("UpdateConnection failed: %v", err)
	}

	// Verify old port is freed
	_, err = s.GetConnectionByPort(1502)
	if err != ErrNotFound {
		t.Error("Old port should be freed after update")
	}

	// Verify new port is indexed
	got, err := s.GetConnectionByPort(1600)
	if err != nil {
		t.Fatalf("GetConnectionByPort(1600) failed: %v", err)
	}
	if got.ID != conn.ID {
		t.Errorf("GetConnectionByPort(1600) ID = %s, want %s", got.ID, conn.ID)
	}
}

func TestUpdateConnectionSamePort(t *testing.T) {
	s := New()

	conn := &model.Connection{ID: "sameportsameportsameportsamepo01", Name: "Test", Port: 1502}
	s.CreateConnection(conn)

	// Use a NEW struct to update (not the same pointer)
	updateReq := &model.Connection{
		ID:   conn.ID,
		Name: "Updated Name",
		Port: 1502,
	}
	err := s.UpdateConnection(updateReq)
	if err != nil {
		t.Fatalf("UpdateConnection failed: %v", err)
	}

	// Verify port is still indexed
	got, err := s.GetConnectionByPort(1502)
	if err != nil {
		t.Fatalf("GetConnectionByPort failed: %v", err)
	}
	if got.Name != "Updated Name" {
		t.Errorf("Name = %s, want Updated Name", got.Name)
	}
}
