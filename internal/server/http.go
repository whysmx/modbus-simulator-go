package server

import (
	"net/http"
	"regexp"

	"github.com/whysmx/modbus-simulator-go/internal/handler"
)

// Router handles HTTP routing
type Router struct {
	apiHandler *handler.APIHandler
	staticFS   http.FileSystem
}

// NewRouter creates a new HTTP router
func NewRouter(api *handler.APIHandler, staticFS http.FileSystem) *Router {
	return &Router{
		apiHandler: api,
		staticFS:   staticFS,
	}
}

// regex patterns for route matching
var (
	connectionsTreePattern = regexp.MustCompile(`^/api/connections/tree$`)
	connectionsPattern     = regexp.MustCompile(`^/api/connections$`)
	connectionPattern      = regexp.MustCompile(`^/api/connections/([a-f0-9]{32})$`)
	privateProtocolPattern = regexp.MustCompile(`^/api/connections/([a-f0-9]{32})/private-protocol$`)
	slavesPattern          = regexp.MustCompile(`^/api/connections/([a-f0-9]{32})/slaves$`)
	slavePattern           = regexp.MustCompile(`^/api/connections/([a-f0-9]{32})/slaves/([a-f0-9]{32})$`)
	registersPattern       = regexp.MustCompile(`^/api/connections/([a-f0-9]{32})/slaves/([a-f0-9]{32})/registers$`)
	registerPattern        = regexp.MustCompile(`^/api/connections/([a-f0-9]{32})/slaves/([a-f0-9]{32})/registers/([a-f0-9]{32})$`)
)

// ServeHTTP implements the http.Handler interface
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path

	// Enable CORS for development
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if req.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// API routes
	if connectionsTreePattern.MatchString(path) && req.Method == "GET" {
		r.apiHandler.GetConnectionsTree(w, req)
		return
	}

	if connectionsPattern.MatchString(path) && req.Method == "POST" {
		r.apiHandler.CreateConnection(w, req)
		return
	}

	if matches := connectionPattern.FindStringSubmatch(path); matches != nil {
		id := matches[1]
		switch req.Method {
		case "PUT":
			r.apiHandler.UpdateConnection(w, req, id)
			return
		case "DELETE":
			r.apiHandler.DeleteConnection(w, req, id)
			return
		}
	}

	if matches := privateProtocolPattern.FindStringSubmatch(path); matches != nil {
		connID := matches[1]
		switch req.Method {
		case "GET":
			r.apiHandler.GetPrivateProtocol(w, req, connID)
			return
		case "PUT":
			r.apiHandler.PutPrivateProtocol(w, req, connID)
			return
		case "DELETE":
			r.apiHandler.DeletePrivateProtocol(w, req, connID)
			return
		}
	}

	if matches := slavesPattern.FindStringSubmatch(path); matches != nil {
		connID := matches[1]
		if req.Method == "POST" {
			r.apiHandler.CreateSlave(w, req, connID)
			return
		}
	}

	if matches := slavePattern.FindStringSubmatch(path); matches != nil {
		connID, slaveID := matches[1], matches[2]
		switch req.Method {
		case "PUT":
			r.apiHandler.UpdateSlave(w, req, connID, slaveID)
			return
		case "DELETE":
			r.apiHandler.DeleteSlave(w, req, connID, slaveID)
			return
		}
	}

	if matches := registersPattern.FindStringSubmatch(path); matches != nil {
		connID, slaveID := matches[1], matches[2]
		switch req.Method {
		case "GET":
			r.apiHandler.GetRegisters(w, req, connID, slaveID)
			return
		case "POST":
			r.apiHandler.CreateRegister(w, req, connID, slaveID)
			return
		}
	}

	if matches := registerPattern.FindStringSubmatch(path); matches != nil {
		connID, slaveID, regID := matches[1], matches[2], matches[3]
		switch req.Method {
		case "GET":
			r.apiHandler.GetRegister(w, req, connID, slaveID, regID)
			return
		case "PUT":
			r.apiHandler.UpdateRegister(w, req, connID, slaveID, regID)
			return
		case "DELETE":
			r.apiHandler.DeleteRegister(w, req, connID, slaveID, regID)
			return
		}
	}

	// Serve index.html for root path
	if path == "/" {
		if r.staticFS != nil {
			f, err := r.staticFS.Open("index.html")
			if err == nil {
				defer f.Close()
				stat, _ := f.Stat()
				http.ServeContent(w, req, "index.html", stat.ModTime(), f.(http.File))
				return
			}
		}
	}

	// Static files with /static/ prefix
	if r.staticFS != nil && len(path) > 8 && path[:8] == "/static/" {
		// Strip /static/ prefix
		req.URL.Path = path[7:] // Keep leading slash: /static/css/... -> /css/...
		fileServer := http.FileServer(r.staticFS)
		fileServer.ServeHTTP(w, req)
		return
	}

	http.NotFound(w, req)
}
