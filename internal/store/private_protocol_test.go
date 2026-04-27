package store

import (
	"testing"

	"github.com/whysmx/modbus-simulator-go/internal/model"
)

func TestPrivateProtocolCRUD(t *testing.T) {
	s := New()
	conn := &model.Connection{
		ID:          "privatecrudconnprivatecrudconn",
		Name:        "Private",
		Port:        2502,
		ServiceType: model.ServiceTypePrivateProtocol,
	}
	if err := s.CreateConnection(conn); err != nil {
		t.Fatalf("CreateConnection failed: %v", err)
	}

	pp := &model.PrivateProtocol{
		ID:     "privatecrudprotocolprivatecrudp",
		ConnID: conn.ID,
		Name:   "Proto",
		Rules: []model.PrivateProtocolRule{{
			MatchMode:   model.PrivateMatchExact,
			RequestHex:  "AA",
			ResponseHex: "BB",
		}},
	}
	if err := s.UpsertPrivateProtocol(pp); err != nil {
		t.Fatalf("UpsertPrivateProtocol failed: %v", err)
	}

	got, err := s.GetPrivateProtocol(conn.ID)
	if err != nil {
		t.Fatalf("GetPrivateProtocol failed: %v", err)
	}
	if got.Name != "Proto" || got.Rules[0].RequestHex != "AA" {
		t.Fatalf("unexpected private protocol: %+v", got)
	}

	pp.Name = "Proto2"
	if err := s.UpsertPrivateProtocol(pp); err != nil {
		t.Fatalf("second UpsertPrivateProtocol failed: %v", err)
	}
	got, err = s.GetPrivateProtocol(conn.ID)
	if err != nil || got.Name != "Proto2" {
		t.Fatalf("updated private protocol not found: got=%+v err=%v", got, err)
	}

	if err := s.DeleteConnection(conn.ID); err != nil {
		t.Fatalf("DeleteConnection failed: %v", err)
	}
	if _, err := s.GetPrivateProtocol(conn.ID); err != ErrNotFound {
		t.Fatalf("private protocol should cascade from memory store, got %v", err)
	}
}
