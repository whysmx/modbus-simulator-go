package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed static
var staticFiles embed.FS

// StaticFS returns the static file system
func StaticFS() (http.FileSystem, error) {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}
	return http.FS(sub), nil
}

// Exists checks if a file exists in the static file system
func Exists(name string) bool {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return false
	}

	// Validate input
	if name == "" {
		return false
	}

	// Clean the path to prevent directory traversal
	cleanName := path.Clean(name)
	if strings.Contains(cleanName, "..") {
		return false
	}

	// Check if path is absolute
	if path.IsAbs(cleanName) {
		return false
	}

	_, err = sub.Open(cleanName)
	return err == nil
}

// ReadFile reads a file from the static file system
func ReadFile(name string) ([]byte, error) {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}

	// Validate input
	if name == "" {
		return nil, fs.ErrInvalid
	}

	// Clean the path to prevent directory traversal
	cleanName := path.Clean(name)
	if strings.Contains(cleanName, "..") {
		return nil, fs.ErrInvalid
	}

	// Check if path is absolute
	if path.IsAbs(cleanName) {
		return nil, fs.ErrInvalid
	}

	return fs.ReadFile(sub, cleanName)
}

// ListFiles lists all files in the static file system with a given prefix
func ListFiles(prefix string) ([]string, error) {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}

	var files []string

	// Validate and clean prefix
	cleanPrefix := prefix
	if cleanPrefix != "" {
		cleanPrefix = path.Clean(cleanPrefix)

		// Check for path traversal before cleaning
		if strings.Contains(prefix, "..") {
			return nil, fs.ErrInvalid
		}

		// Check if it's an absolute path
		if path.IsAbs(cleanPrefix) || strings.HasPrefix(prefix, "/") {
			return nil, fs.ErrInvalid
		}
	}

	err = fs.WalkDir(sub, ".", func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Filter by prefix if specified
		if cleanPrefix == "" || strings.HasPrefix(filePath, cleanPrefix) {
			files = append(files, filePath)
		}

		return nil
	})

	return files, err
}

// ValidatePath checks if a path is valid (no traversal attempts, not absolute)
func ValidatePath(name string) bool {
	if name == "" {
		return false
	}

	cleanName := path.Clean(name)
	if strings.Contains(cleanName, "..") {
		return false
	}

	if path.IsAbs(cleanName) {
		return false
	}

	return true
}

// GetFileInfo returns file information for a given path
func GetFileInfo(name string) (fs.FileInfo, error) {
	if !ValidatePath(name) {
		return nil, fs.ErrInvalid
	}

	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}

	cleanName := path.Clean(name)
	f, err := sub.Open(cleanName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return f.Stat()
}
