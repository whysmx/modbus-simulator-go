package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/whysmx/modbus-simulator-go/internal/model"
	"github.com/whysmx/modbus-simulator-go/internal/store"
)

const (
	privateReadBufferSize = 512
)

func (s *TCPServer) handlePrivateConnection(conn net.Conn, pl *portListener) {
	debugEnabled := os.Getenv("MODBUS_DEBUG") == "1"
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
		_ = tcpConn.SetKeepAlive(true)
		_ = tcpConn.SetKeepAlivePeriod(60 * time.Second)
	}
	if debugEnabled {
		log.Printf("CONN %s service=private open", conn.RemoteAddr())
	}

	buf := make([]byte, privateReadBufferSize)

	for {
		select {
		case <-pl.done:
			if debugEnabled {
				log.Printf("CONN %s service=private closed by server", conn.RemoteAddr())
			}
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		n, err := conn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if debugEnabled {
					log.Printf("CONN %s service=private read timeout", conn.RemoteAddr())
				}
				continue
			}
			if err != io.EOF {
				log.Printf("Private protocol read error from %s: %v", conn.RemoteAddr(), err)
			}
			if debugEnabled {
				log.Printf("CONN %s service=private closed by peer", conn.RemoteAddr())
			}
			return
		}
		if n == 0 {
			continue
		}

		pp, err := s.store.GetPrivateProtocol(pl.connID)
		if err == store.ErrNotFound {
			if debugEnabled {
				log.Printf("CONN %s service=private config missing", conn.RemoteAddr())
			}
			continue
		}
		if err != nil {
			log.Printf("Private protocol config error for connection %s: %v", pl.connID, err)
			continue
		}

		frame := append([]byte(nil), buf[:n]...)
		if debugEnabled {
			log.Printf("RX %s service=private bytes=%d data=%s", conn.RemoteAddr(), len(frame), formatHex(frame))
		}

		respBytes, matched, err := buildPrivateResponse(pp, frame)
		if err != nil {
			log.Printf("Private protocol response build error for connection %s: %v", pl.connID, err)
			continue
		}
		if !matched {
			if debugEnabled {
				log.Printf("CONN %s service=private no rule for data=%s", conn.RemoteAddr(), formatHex(frame))
			}
			continue
		}

		if debugEnabled {
			log.Printf("TX %s service=private bytes=%d data=%s", conn.RemoteAddr(), len(respBytes), formatHex(respBytes))
		}
		if _, err := conn.Write(respBytes); err != nil {
			log.Printf("Private protocol write error to %s: %v", conn.RemoteAddr(), err)
			return
		}
	}
}

func buildPrivateResponse(pp *model.PrivateProtocol, frame []byte) ([]byte, bool, error) {
	requestHex := strings.ToUpper(hex.EncodeToString(frame))
	for _, rule := range pp.Rules {
		if !privateRuleMatches(rule, requestHex) {
			continue
		}

		responseHex, err := renderPrivateResponseHex(rule.ResponseHex, rule.RandomConfig)
		if err != nil {
			return nil, true, err
		}
		resp, err := hex.DecodeString(responseHex)
		if err != nil {
			return nil, true, err
		}
		return resp, true, nil
	}
	return nil, false, nil
}

func privateRuleMatches(rule model.PrivateProtocolRule, requestHex string) bool {
	switch rule.MatchMode {
	case model.PrivateMatchExact:
		return strings.EqualFold(rule.RequestHex, requestHex)
	case model.PrivateMatchContains:
		return strings.Contains(strings.ToUpper(requestHex), strings.ToUpper(rule.RequestHex))
	default:
		return false
	}
}

func renderPrivateResponseHex(template string, configs []model.PrivateRandomConfig) (string, error) {
	responseHex := strings.ToUpper(strings.TrimSpace(template))
	if len(configs) == 0 {
		return responseHex, nil
	}

	sortedConfigs := make([]model.PrivateRandomConfig, len(configs))
	copy(sortedConfigs, configs)
	sort.Slice(sortedConfigs, func(i, j int) bool {
		return len(sortedConfigs[i].Token) > len(sortedConfigs[j].Token)
	})

	for _, cfg := range sortedConfigs {
		if cfg.WidthBytes < 1 || cfg.WidthBytes > 8 {
			return "", fmt.Errorf("invalid widthBytes for token %s", cfg.Token)
		}
		if cfg.Max < cfg.Min {
			return "", fmt.Errorf("invalid range for token %s", cfg.Token)
		}

		span := big.NewInt(int64(cfg.Max))
		span.Sub(span, big.NewInt(int64(cfg.Min)))
		span.Add(span, big.NewInt(1))
		if span.Sign() <= 0 {
			return "", fmt.Errorf("invalid range for token %s", cfg.Token)
		}

		n, err := rand.Int(rand.Reader, span)
		if err != nil {
			return "", err
		}
		value := big.NewInt(int64(cfg.Min))
		value.Add(value, n)
		tokenHex := strings.ToUpper(value.Text(16))
		width := cfg.WidthBytes * 2
		if len(tokenHex) > width {
			return "", fmt.Errorf("value for token %s does not fit widthBytes", cfg.Token)
		}
		if len(tokenHex) < width {
			tokenHex = strings.Repeat("0", width-len(tokenHex)) + tokenHex
		}
		responseHex = strings.ReplaceAll(responseHex, strings.ToUpper(cfg.Token), tokenHex)
	}
	return responseHex, nil
}
