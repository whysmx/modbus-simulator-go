package store

import (
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"

	"github.com/whysmx/modbus-simulator-go/internal/model"
)

const sqliteSchema = `
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS connections (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	port INTEGER NOT NULL UNIQUE,
	protocol_type INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS slaves (
	id TEXT PRIMARY KEY,
	conn_id TEXT NOT NULL,
	name TEXT NOT NULL,
	slave_addr INTEGER NOT NULL,
	FOREIGN KEY(conn_id) REFERENCES connections(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_slaves_conn_addr ON slaves(conn_id, slave_addr);
CREATE INDEX IF NOT EXISTS idx_slaves_conn ON slaves(conn_id);

CREATE TABLE IF NOT EXISTS registers (
	id TEXT PRIMARY KEY,
	slave_id TEXT NOT NULL,
	start_addr INTEGER NOT NULL,
	hex_data TEXT NOT NULL,
	names TEXT,
	coefficients TEXT,
	FOREIGN KEY(slave_id) REFERENCES slaves(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_registers_slave ON registers(slave_id);
`

// NewSQLite creates a new store backed by sqlite and loads existing data.
func NewSQLite(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(sqliteSchema); err != nil {
		db.Close()
		return nil, err
	}

	s := &Store{
		db:             db,
		connections:    make(map[string]*model.Connection),
		slaves:         make(map[string]*model.Slave),
		registers:      make(map[string]*model.Register),
		portToConnID:   make(map[int]string),
		connIDToSlaves: make(map[string][]string),
		slaveToRegs:    make(map[string][]string),
	}

	if err := s.loadFromSQLite(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

// Close closes the underlying sqlite database if present.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) loadFromSQLite() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	connRows, err := s.db.Query(`SELECT id, name, port, protocol_type FROM connections`)
	if err != nil {
		return err
	}
	defer connRows.Close()

	for connRows.Next() {
		conn := &model.Connection{}
		if err := connRows.Scan(&conn.ID, &conn.Name, &conn.Port, &conn.ProtocolType); err != nil {
			return err
		}
		s.connections[conn.ID] = conn
		s.portToConnID[conn.Port] = conn.ID
	}
	if err := connRows.Err(); err != nil {
		return err
	}

	slaveRows, err := s.db.Query(`SELECT id, conn_id, name, slave_addr FROM slaves`)
	if err != nil {
		return err
	}
	defer slaveRows.Close()

	for slaveRows.Next() {
		slave := &model.Slave{}
		if err := slaveRows.Scan(&slave.ID, &slave.ConnID, &slave.Name, &slave.SlaveAddr); err != nil {
			return err
		}
		s.slaves[slave.ID] = slave
		s.connIDToSlaves[slave.ConnID] = append(s.connIDToSlaves[slave.ConnID], slave.ID)
	}
	if err := slaveRows.Err(); err != nil {
		return err
	}

	regRows, err := s.db.Query(`SELECT id, slave_id, start_addr, hex_data, names, coefficients FROM registers`)
	if err != nil {
		return err
	}
	defer regRows.Close()

	for regRows.Next() {
		reg := &model.Register{}
		if err := regRows.Scan(&reg.ID, &reg.SlaveID, &reg.StartAddr, &reg.HexData, &reg.Names, &reg.Coefficients); err != nil {
			return err
		}
		s.registers[reg.ID] = reg
		s.slaveToRegs[reg.SlaveID] = append(s.slaveToRegs[reg.SlaveID], reg.ID)
	}
	if err := regRows.Err(); err != nil {
		return err
	}

	return nil
}

func (s *Store) insertConnection(conn *model.Connection) error {
	if s.db == nil {
		return errors.New("sqlite not initialized")
	}
	_, err := s.db.Exec(
		`INSERT INTO connections (id, name, port, protocol_type) VALUES (?, ?, ?, ?)`,
		conn.ID, conn.Name, conn.Port, conn.ProtocolType,
	)
	return err
}

func (s *Store) updateConnection(conn *model.Connection) error {
	if s.db == nil {
		return errors.New("sqlite not initialized")
	}
	res, err := s.db.Exec(
		`UPDATE connections SET name = ?, port = ?, protocol_type = ? WHERE id = ?`,
		conn.Name, conn.Port, conn.ProtocolType, conn.ID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) deleteConnection(id string) error {
	if s.db == nil {
		return errors.New("sqlite not initialized")
	}
	res, err := s.db.Exec(`DELETE FROM connections WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) insertSlave(slave *model.Slave) error {
	if s.db == nil {
		return errors.New("sqlite not initialized")
	}
	_, err := s.db.Exec(
		`INSERT INTO slaves (id, conn_id, name, slave_addr) VALUES (?, ?, ?, ?)`,
		slave.ID, slave.ConnID, slave.Name, slave.SlaveAddr,
	)
	return err
}

func (s *Store) updateSlave(slave *model.Slave) error {
	if s.db == nil {
		return errors.New("sqlite not initialized")
	}
	res, err := s.db.Exec(
		`UPDATE slaves SET name = ?, slave_addr = ? WHERE id = ?`,
		slave.Name, slave.SlaveAddr, slave.ID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) deleteSlave(id string) error {
	if s.db == nil {
		return errors.New("sqlite not initialized")
	}
	res, err := s.db.Exec(`DELETE FROM slaves WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) insertRegister(reg *model.Register) error {
	if s.db == nil {
		return errors.New("sqlite not initialized")
	}
	_, err := s.db.Exec(
		`INSERT INTO registers (id, slave_id, start_addr, hex_data, names, coefficients)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		reg.ID, reg.SlaveID, reg.StartAddr, reg.HexData, reg.Names, reg.Coefficients,
	)
	return err
}

func (s *Store) updateRegister(reg *model.Register) error {
	if s.db == nil {
		return errors.New("sqlite not initialized")
	}
	res, err := s.db.Exec(
		`UPDATE registers SET start_addr = ?, hex_data = ?, names = ?, coefficients = ? WHERE id = ?`,
		reg.StartAddr, reg.HexData, reg.Names, reg.Coefficients, reg.ID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) deleteRegister(id string) error {
	if s.db == nil {
		return errors.New("sqlite not initialized")
	}
	res, err := s.db.Exec(`DELETE FROM registers WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) debugSQLiteCounts() (string, error) {
	if s.db == nil {
		return "", errors.New("sqlite not initialized")
	}
	var connCount, slaveCount, regCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM connections`).Scan(&connCount); err != nil {
		return "", err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM slaves`).Scan(&slaveCount); err != nil {
		return "", err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM registers`).Scan(&regCount); err != nil {
		return "", err
	}
	return fmt.Sprintf("connections=%d slaves=%d registers=%d", connCount, slaveCount, regCount), nil
}
