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

func TestPrivateProtocolLifecycle(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)

	createBody := bytes.NewBufferString(`{"name":"Private","port":2502,"serviceType":1}`)
	createReq := httptest.NewRequest("POST", "/api/connections", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	h.CreateConnection(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("CreateConnection status = %d, body=%s", createW.Code, createW.Body.String())
	}
	var conn model.Connection
	if err := json.NewDecoder(createW.Body).Decode(&conn); err != nil {
		t.Fatalf("decode connection: %v", err)
	}
	if conn.ServiceType != model.ServiceTypePrivateProtocol {
		t.Fatalf("serviceType = %d, want private", conn.ServiceType)
	}

	getReq := httptest.NewRequest("GET", "/api/connections/"+conn.ID+"/private-protocol", nil)
	getW := httptest.NewRecorder()
	h.GetPrivateProtocol(getW, getReq, conn.ID)
	if getW.Code != http.StatusOK {
		t.Fatalf("GetPrivateProtocol default status = %d, body=%s", getW.Code, getW.Body.String())
	}

	putBody := bytes.NewBufferString(`{
		"name":"proto",
		"rules":[{
			"name":"rule",
			"matchMode":0,
			"requestHex":"aa 01",
			"responseHex":"bb @1 cc",
			"randomConfig":[{"token":"@1","min":1,"max":1,"widthBytes":2}]
		}]
	}`)
	putReq := httptest.NewRequest("PUT", "/api/connections/"+conn.ID+"/private-protocol", putBody)
	putReq.Header.Set("Content-Type", "application/json")
	putW := httptest.NewRecorder()
	h.PutPrivateProtocol(putW, putReq, conn.ID)
	if putW.Code != http.StatusOK {
		t.Fatalf("PutPrivateProtocol status = %d, body=%s", putW.Code, putW.Body.String())
	}
	var pp model.PrivateProtocol
	if err := json.NewDecoder(putW.Body).Decode(&pp); err != nil {
		t.Fatalf("decode private protocol: %v", err)
	}
	if pp.Rules[0].RequestHex != "AA01" || pp.Rules[0].ResponseHex != "BB@1CC" {
		t.Fatalf("normalization failed: %+v", pp)
	}

	slaveReq := httptest.NewRequest("POST", "/api/connections/"+conn.ID+"/slaves", bytes.NewBufferString(`{"name":"s","slaveAddr":1}`))
	slaveReq.Header.Set("Content-Type", "application/json")
	slaveW := httptest.NewRecorder()
	h.CreateSlave(slaveW, slaveReq, conn.ID)
	if slaveW.Code != http.StatusBadRequest {
		t.Fatalf("CreateSlave on private status = %d, want 400", slaveW.Code)
	}

	updateReq := httptest.NewRequest("PUT", "/api/connections/"+conn.ID, bytes.NewBufferString(`{"name":"Private","port":2502,"serviceType":0}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()
	h.UpdateConnection(updateW, updateReq, conn.ID)
	if updateW.Code != http.StatusBadRequest {
		t.Fatalf("UpdateConnection serviceType switch status = %d, want 400", updateW.Code)
	}

	deleteReq := httptest.NewRequest("DELETE", "/api/connections/"+conn.ID+"/private-protocol", nil)
	deleteW := httptest.NewRecorder()
	h.DeletePrivateProtocol(deleteW, deleteReq, conn.ID)
	if deleteW.Code != http.StatusNoContent {
		t.Fatalf("DeletePrivateProtocol status = %d, want 204", deleteW.Code)
	}
}

func TestPrivateProtocolRejectsInvalidRule(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)
	conn := &model.Connection{ID: "privateinvalidprivateinvalid0000", Name: "Private", Port: 2503, ServiceType: model.ServiceTypePrivateProtocol}
	if err := s.CreateConnection(conn); err != nil {
		t.Fatal(err)
	}

	body := bytes.NewBufferString(`{"name":"proto","rules":[{"matchMode":2,"requestHex":"AA","responseHex":"BB"}]}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID+"/private-protocol", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.PutPrivateProtocol(w, req, conn.ID)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestPrivateProtocolAcceptsContainsRule(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	h := NewAPIHandler(s, tcp, 1502)
	conn := &model.Connection{ID: "privatecontainsprivatecontains000", Name: "Private", Port: 2504, ServiceType: model.ServiceTypePrivateProtocol}
	if err := s.CreateConnection(conn); err != nil {
		t.Fatal(err)
	}

	body := bytes.NewBufferString(`{"name":"proto","rules":[{"matchMode":1,"requestHex":"31 32","responseHex":"34 35 36"}]}`)
	req := httptest.NewRequest("PUT", "/api/connections/"+conn.ID+"/private-protocol", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.PutPrivateProtocol(w, req, conn.ID)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var pp model.PrivateProtocol
	if err := json.NewDecoder(w.Body).Decode(&pp); err != nil {
		t.Fatalf("decode private protocol: %v", err)
	}
	if len(pp.Rules) != 1 || pp.Rules[0].MatchMode != model.PrivateMatchContains || pp.Rules[0].RequestHex != "3132" {
		t.Fatalf("unexpected private protocol: %+v", pp)
	}
}
