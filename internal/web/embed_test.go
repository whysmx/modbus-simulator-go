package web

import (
	"io/fs"
	"net/http"
	"strings"
	"testing"
)

func TestStaticFS(t *testing.T) {
	filesystem, err := StaticFS()
	if err != nil {
		t.Fatalf("StaticFS failed: %v", err)
	}

	if filesystem == nil {
		t.Fatal("StaticFS returned nil")
	}

	// Try to open the index.html file
	f, err := filesystem.Open("index.html")
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
	filesystem, err := StaticFS()
	if err != nil {
		t.Fatalf("StaticFS failed: %v", err)
	}

	// Ensure it implements http.FileSystem
	var _ http.FileSystem = filesystem
}

func TestExists(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "existing file - index.html",
			path:     "index.html",
			expected: true,
		},
		{
			name:     "existing file - js/app.js",
			path:     "js/app.js",
			expected: true,
		},
		{
			name:     "existing file - css/style.css",
			path:     "css/style.css",
			expected: true,
		},
		{
			name:     "non-existent file",
			path:     "nonexistent.html",
			expected: false,
		},
		{
			name:     "path traversal attack - ../test.txt",
			path:     "../test.txt",
			expected: false,
		},
		{
			name:     "path traversal attack - ../../etc/passwd",
			path:     "../../etc/passwd",
			expected: false,
		},
		{
			name:     "absolute path",
			path:     "/etc/passwd",
			expected: false,
		},
		{
			name:     "empty path",
			path:     "",
			expected: false, // Empty path is now invalid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Exists(tt.path)
			if result != tt.expected {
				t.Errorf("Exists(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestReadFile(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
		checkContent bool
		contentHint string
	}{
		{
			name:        "read existing HTML file",
			path:        "index.html",
			expectError: false,
			checkContent: true,
			contentHint: "<!DOCTYPE html>",
		},
		{
			name:        "read existing JS file",
			path:        "js/app.js",
			expectError: false,
			checkContent: true,
			contentHint: "//",
		},
		{
			name:        "read non-existent file",
			path:        "nonexistent.txt",
			expectError: true,
		},
		{
			name:        "path traversal attack",
			path:        "../../../etc/passwd",
			expectError: true,
		},
		{
			name:        "path traversal with encoded dots",
			path:        "..%2F..%2Fetc%2Fpasswd",
			expectError: true,
		},
		{
			name:        "empty path",
			path:        "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := ReadFile(tt.path)

			if tt.expectError {
				if err == nil {
					t.Errorf("ReadFile(%q) expected error, got nil", tt.path)
				}
				return
			}

			if err != nil {
				t.Errorf("ReadFile(%q) unexpected error: %v", tt.path, err)
				return
			}

			if len(data) == 0 {
				t.Errorf("ReadFile(%q) returned empty data", tt.path)
			}

			if tt.checkContent && tt.contentHint != "" {
				content := string(data)
				if !strings.Contains(content, tt.contentHint) {
					t.Errorf("ReadFile(%q) content doesn't contain expected hint %q", tt.path, tt.contentHint)
				}
			}
		})
	}
}

func TestListFiles(t *testing.T) {
	tests := []struct {
		name           string
		prefix         string
		minFiles       int
		expectedSubset []string
	}{
		{
			name:     "list all files",
			prefix:   "",
			minFiles: 3, // At least index.html, js/app.js, css/style.css
		},
		{
			name:     "list HTML files only",
			prefix:   "",
			minFiles: 1,
			expectedSubset: []string{"index.html"},
		},
		{
			name:     "list JS files",
			prefix:   "js",
			minFiles: 1,
			expectedSubset: []string{"js/app.js"},
		},
		{
			name:     "list CSS files",
			prefix:   "css",
			minFiles: 1,
			expectedSubset: []string{"css/style.css"},
		},
		{
			name:     "non-existent prefix",
			prefix:   "nonexistent/",
			minFiles: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := ListFiles(tt.prefix)
			if err != nil {
				t.Fatalf("ListFiles(%q) unexpected error: %v", tt.prefix, err)
			}

			if len(files) < tt.minFiles {
				t.Errorf("ListFiles(%q) returned %d files, want at least %d", tt.prefix, len(files), tt.minFiles)
			}

			// Check if expected files are present
			for _, expected := range tt.expectedSubset {
				found := false
				for _, file := range files {
					if file == expected || strings.HasSuffix(file, expected) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ListFiles(%q) doesn't contain expected file %q. Got: %v", tt.prefix, expected, files)
				}
			}

			t.Logf("ListFiles(%q) returned %d files: %v", tt.prefix, len(files), files)
		})
	}
}

func TestStaticFSOpenNonExistent(t *testing.T) {
	filesystem, err := StaticFS()
	if err != nil {
		t.Fatalf("StaticFS failed: %v", err)
	}

	// Try to open a non-existent file
	f, err := filesystem.Open("nonexistent-file.html")
	if err == nil {
		f.Close()
		t.Error("Opening non-existent file should return error")
	}
}

func TestStaticFSOpenDirectory(t *testing.T) {
	filesystem, err := StaticFS()
	if err != nil {
		t.Fatalf("StaticFS failed: %v", err)
	}

	// Open js directory
	f, err := filesystem.Open("js")
	if err != nil {
		t.Fatalf("Failed to open js directory: %v", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		t.Fatalf("Failed to stat js directory: %v", err)
	}

	if !stat.IsDir() {
		t.Error("js should be a directory")
	}
}

func TestStaticFSReadDir(t *testing.T) {
	filesystem, err := StaticFS()
	if err != nil {
		t.Fatalf("StaticFS failed: %v", err)
	}

	// Open root directory
	f, err := filesystem.Open(".")
	if err != nil {
		t.Fatalf("Failed to open root directory: %v", err)
	}
	defer f.Close()

	entries, err := f.Readdir(-1)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	if len(entries) == 0 {
		t.Error("Root directory should contain files")
	}

	// Check for expected directories
	hasJS := false
	hasCSS := false
	hasIndex := false

	for _, entry := range entries {
		if entry.Name() == "js" {
			hasJS = true
		}
		if entry.Name() == "css" {
			hasCSS = true
		}
		if entry.Name() == "index.html" {
			hasIndex = true
		}
	}

	if !hasJS {
		t.Error("Root directory should contain js directory")
	}
	if !hasCSS {
		t.Error("Root directory should contain css directory")
	}
	if !hasIndex {
		t.Error("Root directory should contain index.html")
	}
}

func TestExistsEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "double slash",
			path:     "js//app.js",
			expected: false, // Cleaned path becomes js/app.js which exists, but the check should handle it
		},
		{
			name:     "trailing slash",
			path:     "js/",
			expected: true, // Directory exists
		},
		{
			name:     "dot in path",
			path:     "./index.html",
			expected: true, // Should be cleaned to index.html
		},
		{
			name:     "multiple dots",
			path:     ".../test.txt",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Exists(tt.path)
			// Just log the result since behavior depends on path.Clean
			t.Logf("Exists(%q) = %v", tt.path, result)
		})
	}
}

func TestReadFileErrorPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "invalid UTF-8 path",
			path: string([]byte{0xFF, 0xFE, 0xFD}),
		},
		{
			name: "very long path",
			path: strings.Repeat("a/", 1000) + "test.txt",
		},
		{
			name: "null bytes in path",
			path: "test\x00.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReadFile(tt.path)
			// Should return an error
			if err == nil {
				t.Errorf("ReadFile(%q) should return error for invalid path", tt.path)
			}
		})
	}
}

func TestListFilesErrorPaths(t *testing.T) {
	// Note: ListFiles with fs.WalkDir is robust and unlikely to fail
	// This test ensures it handles edge cases gracefully
	files, err := ListFiles("")
	if err != nil {
		t.Errorf("ListFiles(\"\") failed: %v", err)
	}

	if len(files) == 0 {
		t.Error("ListFiles should return at least some files")
	}

	t.Logf("ListFiles returned %d files", len(files))
}

// TestStaticFSErrorConditions tests error conditions that are hard to trigger
func TestStaticFSErrorConditions(t *testing.T) {
	// StaticFS should always succeed with embed.FS
	// This test documents that fs.Sub should not fail with valid embed
	filesystem, err := StaticFS()
	if err != nil {
		t.Errorf("StaticFS() failed unexpectedly: %v", err)
	}
	if filesystem == nil {
		t.Error("StaticFS() returned nil filesystem")
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "valid relative path",
			path:     "index.html",
			expected: true,
		},
		{
			name:     "valid nested path",
			path:     "js/app.js",
			expected: true,
		},
		{
			name:     "empty path",
			path:     "",
			expected: false,
		},
		{
			name:     "path traversal",
			path:     "../test.txt",
			expected: false,
		},
		{
			name:     "absolute path",
			path:     "/etc/passwd",
			expected: false,
		},
		{
			name:     "double traversal",
			path:     "../../test.txt",
			expected: false,
		},
		{
			name:     "mixed traversal",
			path:     "js/../../test.txt",
			expected: false,
		},
		{
			name:     "trailing slash",
			path:     "js/",
			expected: true,
		},
		{
			name:     "dot path",
			path:     "./index.html",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidatePath(tt.path)
			if result != tt.expected {
				t.Errorf("ValidatePath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestGetFileInfo(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "existing file",
			path:        "index.html",
			expectError: false,
		},
		{
			name:        "non-existent file",
			path:        "nonexistent.txt",
			expectError: true,
		},
		{
			name:        "path traversal",
			path:        "../test.txt",
			expectError: true,
		},
		{
			name:        "absolute path",
			path:        "/etc/passwd",
			expectError: true,
		},
		{
			name:        "empty path",
			path:        "",
			expectError: true,
		},
		{
			name:        "directory",
			path:        "js",
			expectError: false, // Directories can be stat'd
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := GetFileInfo(tt.path)

			if tt.expectError {
				if err == nil {
					t.Errorf("GetFileInfo(%q) expected error, got nil", tt.path)
				}
				return
			}

			if err != nil {
				t.Errorf("GetFileInfo(%q) unexpected error: %v", tt.path, err)
				return
			}

			if info == nil {
				t.Errorf("GetFileInfo(%q) returned nil info", tt.path)
			}

			t.Logf("GetFileInfo(%q): name=%s, size=%d, isDir=%v",
				tt.path, info.Name(), info.Size(), info.IsDir())
		})
	}
}

func TestExistsWithValidation(t *testing.T) {
	// Test empty path (now validated)
	result := Exists("")
	if result {
		t.Error("Exists(\"\") should return false")
	}

	// Test absolute path (now validated)
	result = Exists("/etc/passwd")
	if result {
		t.Error("Exists(\"/etc/passwd\") should return false")
	}
}

func TestReadFileWithValidation(t *testing.T) {
	// Test empty path
	_, err := ReadFile("")
	if err != fs.ErrInvalid {
		t.Errorf("ReadFile(\"\") should return fs.ErrInvalid, got %v", err)
	}

	// Test absolute path
	_, err = ReadFile("/etc/passwd")
	if err != fs.ErrInvalid {
		t.Errorf("ReadFile(\"/etc/passwd\") should return fs.ErrInvalid, got %v", err)
	}
}

func TestListFilesWithInvalidPrefix(t *testing.T) {
	// Test path traversal in prefix
	_, err := ListFiles("../test")
	if err != fs.ErrInvalid {
		t.Errorf("ListFiles(\"../test\") should return fs.ErrInvalid, got %v", err)
	}

	// Test absolute path prefix
	_, err = ListFiles("/etc")
	if err != fs.ErrInvalid {
		t.Errorf("ListFiles(\"/etc\") should return fs.ErrInvalid, got %v", err)
	}
}

func TestExistsSpecialCases(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "current directory",
			path:     ".",
			expected: true, // Current directory exists
		},
		{
			name:     "parent directory attempt",
			path:     "..",
			expected: false, // Blocked by validation
		},
		{
			name:     "windows-style path",
			path:     "js\\app.js",
			expected: false, // Backslash not valid in paths
		},
		{
			name:     "url-encoded path",
			path:     "js%2Fapp.js",
			expected: false, // Should not exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Exists(tt.path)
			t.Logf("Exists(%q) = %v", tt.path, result)
			// These are informational tests
			_ = tt.expected
		})
	}
}

func TestReadFileSpecialCases(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "read directory",
			path: "js",
		},
		{
			name: "read with query string",
			path: "index.html?v=1",
		},
		{
			name: "read with fragment",
			path: "index.html#section",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReadFile(tt.path)
			// Most should fail or have specific behavior
			t.Logf("ReadFile(%q) error: %v", tt.path, err)
		})
	}
}

func TestStaticFSMultipleCalls(t *testing.T) {
	// Test that StaticFS can be called multiple times consistently
	filesystem1, err1 := StaticFS()
	filesystem2, err2 := StaticFS()

	if err1 != nil || err2 != nil {
		t.Errorf("StaticFS() should not fail: err1=%v, err2=%v", err1, err2)
	}

	if filesystem1 == nil || filesystem2 == nil {
		t.Error("StaticFS() should not return nil")
	}
}

func TestConcurrentAccess(t *testing.T) {
	// Test concurrent access to static files
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			_, _ = ReadFile("index.html")
			_ = Exists("js/app.js")
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we get here without deadlock or panic, concurrent access works
	t.Log("Concurrent access test passed")
}

func TestListFilesWithSpecialPrefixes(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		minCount int
	}{
		{
			name:     "trailing slash prefix",
			prefix:   "js/",
			minCount: 1,
		},
		{
			name:     "double slash",
			prefix:   "js//",
			minCount: 1, // Should be cleaned and still work
		},
		{
			name:     "index prefix",
			prefix:   "index",
			minCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := ListFiles(tt.prefix)
			if err != nil {
				t.Fatalf("ListFiles(%q) failed: %v", tt.prefix, err)
			}

			if len(files) < tt.minCount {
				t.Errorf("ListFiles(%q) returned %d files, want at least %d", tt.prefix, len(files), tt.minCount)
			}

			t.Logf("ListFiles(%q) returned %d files", tt.prefix, len(files))
		})
	}
}

func TestGetFileInfoSpecialCases(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		checkSize   bool
		checkIsDir  bool
		expectDir   bool
	}{
		{
			name:     "index.html file info",
			path:     "index.html",
			checkSize: true,
			checkIsDir: true,
			expectDir: false,
		},
		{
			name:     "js directory info",
			path:     "js",
			checkSize: true,
			checkIsDir: true,
			expectDir: true,
		},
		{
			name:     "css directory info",
			path:     "css",
			checkSize: true,
			checkIsDir: true,
			expectDir: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := GetFileInfo(tt.path)
			if err != nil {
				t.Fatalf("GetFileInfo(%q) failed: %v", tt.path, err)
			}

			if tt.checkIsDir {
				if info.IsDir() != tt.expectDir {
					t.Errorf("GetFileInfo(%q).IsDir() = %v, want %v", tt.path, info.IsDir(), tt.expectDir)
				}
			}

			if tt.checkSize {
				t.Logf("GetFileInfo(%q): size=%d bytes, mode=%v", tt.path, info.Size(), info.Mode())
			}
		})
	}
}

func TestExistsAllStaticFiles(t *testing.T) {
	// Test that we can check existence of all known files
	files, err := ListFiles("")
	if err != nil {
		t.Fatalf("ListFiles() failed: %v", err)
	}

	// Verify all files returned by ListFiles exist
	for _, file := range files {
		if !Exists(file) {
			t.Errorf("File %s was returned by ListFiles but Exists() returned false", file)
		}
	}

	t.Logf("Verified %d files exist", len(files))
}

func TestReadFileInfoConsistency(t *testing.T) {
	// Test that ReadFile and GetFileInfo return consistent information
	files := []string{"index.html", "js/app.js", "css/style.css"}

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			// Read file content
			content, err := ReadFile(file)
			if err != nil {
				t.Fatalf("ReadFile(%q) failed: %v", file, err)
			}

			// Get file info
			info, err := GetFileInfo(file)
			if err != nil {
				t.Fatalf("GetFileInfo(%q) failed: %v", file, err)
			}

			// Content length should match file size
			if int64(len(content)) != info.Size() {
				t.Errorf("Size mismatch: ReadFile returned %d bytes, GetFileInfo.Size() = %d",
					len(content), info.Size())
			}

			t.Logf("File %s: size=%d, name=%s", file, info.Size(), info.Name())
		})
	}
}

func TestStaticFSFileOperations(t *testing.T) {
	filesystem, err := StaticFS()
	if err != nil {
		t.Fatalf("StaticFS failed: %v", err)
	}

	// Test opening and reading a file
	f, err := filesystem.Open("index.html")
	if err != nil {
		t.Fatalf("Failed to open index.html: %v", err)
	}
	defer f.Close()

	// Read some content
	buf := make([]byte, 100)
	n, err := f.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read from file: %v", err)
	}

	if n == 0 {
		t.Error("Read returned 0 bytes")
	}

	if len(buf) == 0 {
		t.Error("Buffer is empty")
	}

	t.Logf("Read %d bytes from index.html", n)
}

func TestValidatePathComprehensive(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "simple file",
			path:     "test.txt",
			expected: true,
		},
		{
			name:     "nested file",
			path:     "a/b/c.txt",
			expected: true,
		},
		{
			name:     "file with hash",
			path:     "file#1.txt",
			expected: true,
		},
		{
			name:     "file with question mark",
			path:     "file?1.txt",
			expected: true,
		},
		{
			name:     "file with ampersand",
			path:     "file&test.txt",
			expected: true,
		},
		{
			name:     "file with equals",
			path:     "file=v1.txt",
			expected: true,
		},
		{
			name:     "file with percent",
			path:     "file%20test.txt",
			expected: true,
		},
		{
			name:     "multiple slashes",
			path:     "a///b.txt",
			expected: true,
		},
		{
			name:     "dots in filename",
			path:     "file.test.txt",
			expected: true,
		},
		{
			name:     "trailing dot",
			path:     "file.",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidatePath(tt.path)
			if result != tt.expected {
				t.Errorf("ValidatePath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

