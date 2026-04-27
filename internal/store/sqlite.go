package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/whysmx/modbus-simulator-go/internal/model"
)

const sqliteSchema = `
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS connections (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	port INTEGER NOT NULL UNIQUE,
	protocol_type INTEGER NOT NULL,
	service_type INTEGER NOT NULL DEFAULT 0
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
	jitter_amp INTEGER NOT NULL DEFAULT 0,
	coefficients TEXT,
	FOREIGN KEY(slave_id) REFERENCES slaves(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_registers_slave ON registers(slave_id);

CREATE TABLE IF NOT EXISTS private_protocols (
	id TEXT PRIMARY KEY,
	conn_id TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	frame_delimiter_hex TEXT NOT NULL,
	rules_json TEXT NOT NULL,
	FOREIGN KEY(conn_id) REFERENCES connections(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_private_protocols_conn ON private_protocols(conn_id);
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

	if _, err := db.Exec(`ALTER TABLE connections ADD COLUMN service_type INTEGER NOT NULL DEFAULT 0`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			db.Close()
			return nil, err
		}
	}

	if _, err := db.Exec(`ALTER TABLE registers ADD COLUMN jitter_amp INTEGER NOT NULL DEFAULT 0`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			db.Close()
			return nil, err
		}
	}

	s := &Store{
		db:               db,
		connections:      make(map[string]*model.Connection),
		slaves:           make(map[string]*model.Slave),
		registers:        make(map[string]*model.Register),
		privateProtocols: make(map[string]*model.PrivateProtocol),
		portToConnID:     make(map[int]string),
		connIDToSlaves:   make(map[string][]string),
		slaveToRegs:      make(map[string][]string),
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

	connRows, err := s.db.Query(`SELECT id, name, port, protocol_type, service_type FROM connections`)
	if err != nil {
		return err
	}
	defer connRows.Close()

	for connRows.Next() {
		conn := &model.Connection{}
		if err := connRows.Scan(&conn.ID, &conn.Name, &conn.Port, &conn.ProtocolType, &conn.ServiceType); err != nil {
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

	regRows, err := s.db.Query(`SELECT id, slave_id, start_addr, hex_data, names, coefficients, jitter_amp FROM registers`)
	if err != nil {
		return err
	}
	defer regRows.Close()

	for regRows.Next() {
		reg := &model.Register{}
		if err := regRows.Scan(&reg.ID, &reg.SlaveID, &reg.StartAddr, &reg.HexData, &reg.Names, &reg.Coefficients, &reg.JitterAmp); err != nil {
			return err
		}
		s.registers[reg.ID] = reg
		s.slaveToRegs[reg.SlaveID] = append(s.slaveToRegs[reg.SlaveID], reg.ID)
	}
	if err := regRows.Err(); err != nil {
		return err
	}

	ppRows, err := s.db.Query(`SELECT id, conn_id, name, frame_delimiter_hex, rules_json FROM private_protocols`)
	if err != nil {
		return err
	}
	defer ppRows.Close()

	for ppRows.Next() {
		pp := &model.PrivateProtocol{}
		var legacyDelimiter string
		var rulesJSON string
		if err := ppRows.Scan(&pp.ID, &pp.ConnID, &pp.Name, &legacyDelimiter, &rulesJSON); err != nil {
			return err
		}
		if strings.TrimSpace(rulesJSON) == "" {
			pp.Rules = []model.PrivateProtocolRule{}
		} else if err := json.Unmarshal([]byte(rulesJSON), &pp.Rules); err != nil {
			return fmt.Errorf("load private protocol %s rules: %w", pp.ID, err)
		}
		s.privateProtocols[pp.ConnID] = pp
	}
	if err := ppRows.Err(); err != nil {
		return err
	}

	return nil
}

func (s *Store) insertConnection(conn *model.Connection) error {
	if s.db == nil {
		return errors.New("sqlite not initialized")
	}
	_, err := s.db.Exec(
		`INSERT INTO connections (id, name, port, protocol_type, service_type) VALUES (?, ?, ?, ?, ?)`,
		conn.ID, conn.Name, conn.Port, conn.ProtocolType, conn.ServiceType,
	)
	return err
}

func (s *Store) updateConnection(conn *model.Connection) error {
	if s.db == nil {
		return errors.New("sqlite not initialized")
	}
	res, err := s.db.Exec(
		`UPDATE connections SET name = ?, port = ?, protocol_type = ?, service_type = ? WHERE id = ?`,
		conn.Name, conn.Port, conn.ProtocolType, conn.ServiceType, conn.ID,
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

func (s *Store) upsertPrivateProtocol(pp *model.PrivateProtocol) error {
	if s.db == nil {
		return errors.New("sqlite not initialized")
	}
	rulesJSON, err := json.Marshal(pp.Rules)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(
		`UPDATE private_protocols SET id = ?, name = ?, frame_delimiter_hex = ?, rules_json = ? WHERE conn_id = ?`,
		pp.ID, pp.Name, "", string(rulesJSON), pp.ConnID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	_, err = s.db.Exec(
		`INSERT INTO private_protocols (id, conn_id, name, frame_delimiter_hex, rules_json) VALUES (?, ?, ?, ?, ?)`,
		pp.ID, pp.ConnID, pp.Name, "", string(rulesJSON),
	)
	return err
}

func (s *Store) deletePrivateProtocol(connID string) error {
	if s.db == nil {
		return errors.New("sqlite not initialized")
	}
	res, err := s.db.Exec(`DELETE FROM private_protocols WHERE conn_id = ?`, connID)
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
		`INSERT INTO registers (id, slave_id, start_addr, hex_data, names, coefficients, jitter_amp)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		reg.ID, reg.SlaveID, reg.StartAddr, reg.HexData, reg.Names, reg.Coefficients, reg.JitterAmp,
	)
	return err
}

func (s *Store) updateRegister(reg *model.Register) error {
	if s.db == nil {
		return errors.New("sqlite not initialized")
	}
	res, err := s.db.Exec(
		`UPDATE registers SET start_addr = ?, hex_data = ?, names = ?, coefficients = ?, jitter_amp = ? WHERE id = ?`,
		reg.StartAddr, reg.HexData, reg.Names, reg.Coefficients, reg.JitterAmp, reg.ID,
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
