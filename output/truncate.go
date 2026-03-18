package output

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	MaxStringLength = 4096
	TempDir         = "tmp"
	TempFileTTL     = 1 * time.Hour
)

func agiosDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".agios"), nil
}

func tempDir() (string, error) {
	base, err := agiosDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, TempDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating temp directory: %w", err)
	}
	return dir, nil
}

type spillFunc func(value string) (string, error)

func defaultSpill(value string) (string, error) {
	dir, err := tempDir()
	if err != nil {
		return "", err
	}
	return spillToDir(value, dir)
}

func spillToDir(value string, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating temp directory: %w", err)
	}

	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random name: %w", err)
	}
	filename := hex.EncodeToString(b) + ".txt"
	path := filepath.Join(dir, filename)

	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		return "", fmt.Errorf("writing temp file: %w", err)
	}

	return path, nil
}

// Truncate replaces string values exceeding MaxStringLength with file references.
func Truncate(v any) (any, error) {
	return truncateWalk(v, defaultSpill)
}

// TruncateWithDir is like Truncate but uses a custom directory (for testing).
func TruncateWithDir(v any, dir string) (any, error) {
	spill := func(value string) (string, error) {
		return spillToDir(value, dir)
	}
	return truncateWalk(v, spill)
}

// truncateWalk recursively walks a JSON value, spilling large strings via the
// provided spillFunc.
func truncateWalk(v any, spill spillFunc) (any, error) {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, elem := range val {
			truncated, err := truncateWalk(elem, spill)
			if err != nil {
				return nil, err
			}
			result[k] = truncated
		}
		return result, nil
	case []any:
		result := make([]any, len(val))
		for i, elem := range val {
			truncated, err := truncateWalk(elem, spill)
			if err != nil {
				return nil, err
			}
			result[i] = truncated
		}
		return result, nil
	case string:
		if len(val) > MaxStringLength {
			path, err := spill(val)
			if err != nil {
				return v, err
			}
			return fmt.Sprintf("[truncated: see %s]", path), nil
		}
		return v, nil
	default:
		return v, nil
	}
}

func CleanupTempFiles() error {
	dir, err := tempDir()
	if err != nil {
		return err
	}
	return cleanupTempFilesIn(dir)
}

// CleanupTempFilesInDir removes temp files older than TempFileTTL in a specific directory.
func CleanupTempFilesInDir(dir string) error {
	return cleanupTempFilesIn(dir)
}

// cleanupTempFilesIn removes temp files older than TempFileTTL in the given directory.
func cleanupTempFilesIn(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading temp directory: %w", err)
	}

	cutoff := time.Now().Add(-TempFileTTL)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(dir, entry.Name()))
		}
	}

	return nil
}
