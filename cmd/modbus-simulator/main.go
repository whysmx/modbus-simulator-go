package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/whysmx/modbus-simulator-go/internal/handler"
	"github.com/whysmx/modbus-simulator-go/internal/server"
	"github.com/whysmx/modbus-simulator-go/internal/store"
	"github.com/whysmx/modbus-simulator-go/internal/web"
)

func main() {
	httpPort := flag.Int("http", 3002, "HTTP server port")
	startPort := flag.Int("modbus-start", 1502, "Starting port for Modbus TCP listeners")
	dbPath := flag.String("db", "modbus.db", "SQLite database file path")
	flag.Parse()

	log.Println("Modbus Simulator starting...")

	// Initialize store
	dataStore, err := store.NewSQLite(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open sqlite db: %v", err)
	}
	defer dataStore.Close()

	// Initialize TCP server
	tcpServer := server.NewTCPServer(dataStore)

	// Start listeners for existing connections in DB
	for _, conn := range dataStore.GetAllConnections() {
		if err := tcpServer.StartListener(conn); err != nil {
			log.Printf("Failed to start listener for %s (%d): %v", conn.Name, conn.Port, err)
		}
	}

	// Initialize API handler
	apiHandler := handler.NewAPIHandler(dataStore, tcpServer, *startPort)

	// Setup static file system
	staticFS, err := web.StaticFS()
	if err != nil {
		log.Fatalf("Failed to setup static files: %v", err)
	}

	// Initialize HTTP router
	router := server.NewRouter(apiHandler, staticFS)

	// Start HTTP server
	httpAddr := ":" + strconv.Itoa(*httpPort)
	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: router,
	}

	go func() {
		log.Printf("HTTP server listening on http://localhost%s", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	tcpServer.StopAll()
	httpServer.Close()
	log.Println("Shutdown complete")
}
