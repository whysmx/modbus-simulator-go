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

func TestRouterPrivateProtocolRoutes(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	api := handler.NewAPIHandler(s, tcp, 1502)
	router := NewRouter(api, nil)

	conn := &model.Connection{ID: "abcdefabcdefabcdefabcdefabcdefab", Name: "Private", Port: 2502, ServiceType: model.ServiceTypePrivateProtocol}
	s.CreateConnection(conn)

	getReq := httptest.NewRequest("GET", "/api/connections/abcdefabcdefabcdefabcdefabcdefab/private-protocol", nil)
	getW := httptest.NewRecorder()
	router.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d. Body: %s", getW.Code, http.StatusOK, getW.Body.String())
	}

	body := bytes.NewBufferString(`{"name":"proto","rules":[{"matchMode":0,"requestHex":"AA","responseHex":"BB"}]}`)
	putReq := httptest.NewRequest("PUT", "/api/connections/abcdefabcdefabcdefabcdefabcdefab/private-protocol", body)
	putReq.Header.Set("Content-Type", "application/json")
	putW := httptest.NewRecorder()
	router.ServeHTTP(putW, putReq)
	if putW.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d. Body: %s", putW.Code, http.StatusOK, putW.Body.String())
	}
}

func TestRouterPrivateProtocolRoutesContainsMatchMode(t *testing.T) {
	s := store.New()
	tcp := &mockTCPServer{}
	api := handler.NewAPIHandler(s, tcp, 1502)
	router := NewRouter(api, nil)

	conn := &model.Connection{ID: "fedcbafedcbafedcbafedcbafedcbafe", Name: "PrivateContains", Port: 2503, ServiceType: model.ServiceTypePrivateProtocol}
	s.CreateConnection(conn)

	body := bytes.NewBufferString(`{"name":"proto","rules":[{"matchMode":1,"requestHex":"AA","responseHex":"BB"}]}`)
	putReq := httptest.NewRequest("PUT", "/api/connections/"+conn.ID+"/private-protocol", body)
	putReq.Header.Set("Content-Type", "application/json")
	putW := httptest.NewRecorder()
	router.ServeHTTP(putW, putReq)
	if putW.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d. Body: %s", putW.Code, http.StatusOK, putW.Body.String())
	}
}
