package server

import (
	"bytes"
	"testing"

	"github.com/whysmx/modbus-simulator-go/internal/model"
)

func TestBuildPrivateResponseExactRequestWithoutDelimiter(t *testing.T) {
	pp := &model.PrivateProtocol{
		Rules: []model.PrivateProtocolRule{{
			MatchMode:   model.PrivateMatchExact,
			RequestHex:  "313233",
			ResponseHex: "343536",
		}},
	}
	resp, matched, err := buildPrivateResponse(pp, []byte("123"))
	if err != nil {
		t.Fatalf("buildPrivateResponse error: %v", err)
	}
	if !matched {
		t.Fatalf("matched=false, want true")
	}
	if !bytes.Equal(resp, []byte("456")) {
		t.Fatalf("resp=%q, want 456", resp)
	}
}

func TestBuildPrivateResponseWithFixedRandomToken(t *testing.T) {
	pp := &model.PrivateProtocol{
		Rules: []model.PrivateProtocolRule{{
			MatchMode:   model.PrivateMatchExact,
			RequestHex:  "AA01",
			ResponseHex: "BB@RCC",
			RandomConfig: []model.PrivateRandomConfig{{
				Token:      "@R",
				Min:        5,
				Max:        5,
				WidthBytes: 1,
			}},
		}},
	}
	resp, matched, err := buildPrivateResponse(pp, []byte{0xAA, 0x01})
	if err != nil {
		t.Fatalf("buildPrivateResponse error: %v", err)
	}
	if !matched {
		t.Fatalf("matched=false, want true")
	}
	want := []byte{0xBB, 0x05, 0xCC}
	if !bytes.Equal(resp, want) {
		t.Fatalf("resp=% X, want % X", resp, want)
	}
}

func TestBuildPrivateResponseContainsMatch(t *testing.T) {
	pp := &model.PrivateProtocol{
		Rules: []model.PrivateProtocolRule{{
			MatchMode:   model.PrivateMatchContains,
			RequestHex:  "313233",
			ResponseHex: "343536",
		}},
	}
	resp, matched, err := buildPrivateResponse(pp, []byte("0012300"))
	if err != nil {
		t.Fatalf("buildPrivateResponse error: %v", err)
	}
	if !matched {
		t.Fatalf("matched=false, want true")
	}
	if !bytes.Equal(resp, []byte("456")) {
		t.Fatalf("resp=%q, want 456", resp)
	}
}
