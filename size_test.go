package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "Unknown"},
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		result := formatSize(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatSize(%d) = %s; expected %s", tt.bytes, result, tt.expected)
		}
	}
}

func TestGetDirSize(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Create a file of 100 bytes
	filePath := filepath.Join(tempDir, "test.txt")
	data := make([]byte, 100)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Create a sub-directory and another file of 50 bytes
	subDir := filepath.Join(tempDir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create sub directory: %v", err)
	}
	subFilePath := filepath.Join(subDir, "test2.txt")
	subData := make([]byte, 50)
	if err := os.WriteFile(subFilePath, subData, 0644); err != nil {
		t.Fatalf("Failed to write temp sub file: %v", err)
	}

	// Calculate size
	size, err := getDirSize(tempDir)
	if err != nil {
		t.Errorf("getDirSize returned unexpected error: %v", err)
	}

	if size != 150 {
		t.Errorf("getDirSize = %d; expected 150", size)
	}
}
