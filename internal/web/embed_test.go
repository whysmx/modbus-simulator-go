package web

import (
	"net/http"
	"testing"
)

func TestStaticFS(t *testing.T) {
	fs, err := StaticFS()
	if err != nil {
		t.Fatalf("StaticFS failed: %v", err)
	}

	if fs == nil {
		t.Fatal("StaticFS returned nil")
	}

	// Try to open the index.html file
	f, err := fs.Open("index.html")
	if err != nil {
		t.Fatalf("Failed to open index.html: %v", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		t.Fatalf("Failed to stat index.html: %v", err)
	}

	if stat.IsDir() {
		t.Error("index.html should be a file, not a directory")
	}
}

func TestStaticFSInterface(t *testing.T) {
	fs, err := StaticFS()
	if err != nil {
		t.Fatalf("StaticFS failed: %v", err)
	}

	// Ensure it implements http.FileSystem
	var _ http.FileSystem = fs
}
