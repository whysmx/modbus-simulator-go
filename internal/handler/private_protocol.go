package handler

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/whysmx/modbus-simulator-go/internal/model"
	"github.com/whysmx/modbus-simulator-go/internal/store"
)

var privateTokenPattern = regexp.MustCompile(`^@[A-Z0-9_]+$`)

func (h *APIHandler) getPrivateConnection(w http.ResponseWriter, connID string) (*model.Connection, bool) {
	conn, err := h.store.GetConnection(connID)
	if err != nil {
		writeError(w, http.StatusNotFound, "connection not found")
		return nil, false
	}
	if conn.ServiceType != model.ServiceTypePrivateProtocol {
		writeError(w, http.StatusBadRequest, "connection is not a private protocol service")
		return nil, false
	}
	return conn, true
}

// GetPrivateProtocol returns the private protocol config for a private connection.
func (h *APIHandler) GetPrivateProtocol(w http.ResponseWriter, r *http.Request, connID string) {
	conn, ok := h.getPrivateConnection(w, connID)
	if !ok {
		return
	}

	pp, err := h.store.GetPrivateProtocol(connID)
	if err == store.ErrNotFound {
		writeJSON(w, http.StatusOK, model.PrivateProtocol{
			ConnID: connID,
			Name:   conn.Name,
			Rules:  []model.PrivateProtocolRule{},
		})
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, pp)
}

// PutPrivateProtocol creates or replaces the private protocol config for a private connection.
func (h *APIHandler) PutPrivateProtocol(w http.ResponseWriter, r *http.Request, connID string) {
	conn, ok := h.getPrivateConnection(w, connID)
	if !ok {
		return
	}

	var pp model.PrivateProtocol
	if err := json.NewDecoder(r.Body).Decode(&pp); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	pp.ConnID = connID
	if existing, err := h.store.GetPrivateProtocol(connID); err == nil {
		pp.ID = existing.ID
	} else if err == store.ErrNotFound {
		pp.ID = generateUUID()
	} else {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(pp.Name) == "" {
		pp.Name = conn.Name
	}
	if pp.Rules == nil {
		pp.Rules = []model.PrivateProtocolRule{}
	}

	if err := normalizePrivateProtocol(&pp); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.store.UpsertPrivateProtocol(&pp); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, pp)
}

// DeletePrivateProtocol removes the private protocol config for a private connection.
func (h *APIHandler) DeletePrivateProtocol(w http.ResponseWriter, r *http.Request, connID string) {
	if _, ok := h.getPrivateConnection(w, connID); !ok {
		return
	}

	if err := h.store.DeletePrivateProtocol(connID); err != nil && err != store.ErrNotFound {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func normalizePrivateProtocol(pp *model.PrivateProtocol) error {
	pp.Name = strings.TrimSpace(pp.Name)
	if pp.Name == "" {
		return fmt.Errorf("name is required")
	}

	var err error
	for i := range pp.Rules {
		rule := &pp.Rules[i]
		rule.Name = strings.TrimSpace(rule.Name)
		if rule.Name == "" {
			rule.Name = fmt.Sprintf("规则%d", i+1)
		}
		switch rule.MatchMode {
		case model.PrivateMatchExact, model.PrivateMatchContains:
		default:
			return fmt.Errorf("rules[%d].matchMode must be 0 or 1", i)
		}
		rule.RequestHex, err = normalizeHexStrict(rule.RequestHex, false)
		if err != nil {
			return fmt.Errorf("invalid rules[%d].requestHex: %w", i, err)
		}
		responseHex, randomConfig, err := normalizePrivateResponseHex(rule.ResponseHex, rule.RandomConfig)
		if err != nil {
			return fmt.Errorf("invalid rules[%d].responseHex: %w", i, err)
		}
		rule.ResponseHex = responseHex
		rule.RandomConfig = randomConfig
	}

	return nil
}

func normalizeHexStrict(value string, allowEmpty bool) (string, error) {
	value = stripSpacesUpper(value)
	if value == "" {
		if allowEmpty {
			return value, nil
		}
		return "", fmt.Errorf("must not be empty")
	}
	if len(value)%2 != 0 {
		return "", fmt.Errorf("length must be even")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", err
	}
	return value, nil
}

func normalizePrivateResponseHex(value string, configs []model.PrivateRandomConfig) (string, []model.PrivateRandomConfig, error) {
	responseHex := stripSpacesUpper(value)
	if responseHex == "" {
		return "", nil, fmt.Errorf("must not be empty")
	}

	normalizedConfigs := make([]model.PrivateRandomConfig, len(configs))
	copy(normalizedConfigs, configs)
	seen := map[string]struct{}{}
	for i := range normalizedConfigs {
		cfg := &normalizedConfigs[i]
		cfg.Token = strings.ToUpper(strings.TrimSpace(cfg.Token))
		if !privateTokenPattern.MatchString(cfg.Token) {
			return "", nil, fmt.Errorf("randomConfig[%d].token must look like @TOKEN", i)
		}
		if _, ok := seen[cfg.Token]; ok {
			return "", nil, fmt.Errorf("randomConfig[%d].token is duplicated", i)
		}
		seen[cfg.Token] = struct{}{}
		if cfg.WidthBytes < 1 || cfg.WidthBytes > 8 {
			return "", nil, fmt.Errorf("randomConfig[%d].widthBytes must be 1-8", i)
		}
		if cfg.Min < 0 {
			return "", nil, fmt.Errorf("randomConfig[%d].min must be >= 0", i)
		}
		if cfg.Max < cfg.Min {
			return "", nil, fmt.Errorf("randomConfig[%d].max must be >= min", i)
		}
		if uint64(cfg.Max) > maxValueForWidth(cfg.WidthBytes) {
			return "", nil, fmt.Errorf("randomConfig[%d].max does not fit widthBytes", i)
		}
		if !strings.Contains(responseHex, cfg.Token) {
			return "", nil, fmt.Errorf("randomConfig[%d].token is not used in responseHex", i)
		}
	}

	sort.Slice(normalizedConfigs, func(i, j int) bool {
		return len(normalizedConfigs[i].Token) > len(normalizedConfigs[j].Token)
	})

	concrete := responseHex
	for _, cfg := range normalizedConfigs {
		concrete = strings.ReplaceAll(concrete, cfg.Token, strings.Repeat("0", cfg.WidthBytes*2))
	}
	if strings.Contains(concrete, "@") {
		return "", nil, fmt.Errorf("responseHex contains token without randomConfig")
	}
	if _, err := normalizeHexStrict(concrete, false); err != nil {
		return "", nil, err
	}

	return responseHex, normalizedConfigs, nil
}

func stripSpacesUpper(value string) string {
	var b strings.Builder
	for _, r := range value {
		if unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(unicode.ToUpper(r))
	}
	return b.String()
}

func maxValueForWidth(widthBytes int) uint64 {
	if widthBytes >= 8 {
		return ^uint64(0)
	}
	return (uint64(1) << uint(widthBytes*8)) - 1
}
