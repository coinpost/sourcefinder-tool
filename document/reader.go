package document

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// MaxDocumentSize is the maximum allowed document size (10MB)
	MaxDocumentSize = 10 * 1024 * 1024
)

// ReadDocument reads the document content from the given file path
func ReadDocument(path string) (string, error) {
	// Clean and validate the path
	cleanPath := filepath.Clean(path)

	// Check if file exists
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", cleanPath)
		}
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	// Check if it's a directory
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s", cleanPath)
	}

	// Check file size
	if info.Size() > MaxDocumentSize {
		return "", fmt.Errorf("file too large (max %d bytes): %s", MaxDocumentSize, cleanPath)
	}

	// Read the file
	content, err := os.ReadFile(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Check if file is empty
	if len(content) == 0 {
		return "", fmt.Errorf("file is empty: %s", cleanPath)
	}

	return string(content), nil
}
