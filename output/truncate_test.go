package output

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTruncateShortString(t *testing.T) {
	result, err := TruncateWithDir("hello", t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello" {
		t.Errorf("expected %q, got %q", "hello", result)
	}
}

func TestTruncateExactThreshold(t *testing.T) {
	// Exactly 4096 chars should NOT be truncated
	s := strings.Repeat("a", MaxStringLength)
	result, err := TruncateWithDir(s, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != s {
		t.Error("expected string at exact threshold to not be truncated")
	}
}

func TestTruncateLongString(t *testing.T) {
	dir := t.TempDir()
	s := strings.Repeat("x", MaxStringLength+1)
	result, err := TruncateWithDir(s, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}
	if !strings.HasPrefix(resultStr, "[truncated: see ") {
		t.Errorf("expected truncation marker, got %q", resultStr)
	}

	// Extract file path and verify contents
	path := strings.TrimPrefix(resultStr, "[truncated: see ")
	path = strings.TrimSuffix(path, "]")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read spilled file: %v", err)
	}
	if string(data) != s {
		t.Errorf("spilled file content mismatch: got %d chars, want %d", len(data), len(s))
	}
}

func TestTruncateMap(t *testing.T) {
	dir := t.TempDir()
	longValue := strings.Repeat("z", MaxStringLength+100)
	input := map[string]any{
		"short": "hello",
		"long":  longValue,
		"num":   42.0,
		"null":  nil,
		"bool":  true,
	}

	result, err := TruncateWithDir(input, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}

	// Short string should be unchanged
	if m["short"] != "hello" {
		t.Errorf("short string changed: %v", m["short"])
	}

	// Long string should be truncated
	longResult, ok := m["long"].(string)
	if !ok {
		t.Fatalf("expected string for long value, got %T", m["long"])
	}
	if !strings.HasPrefix(longResult, "[truncated: see ") {
		t.Errorf("long value not truncated: %q", longResult)
	}

	// Non-string types should be unchanged
	if m["num"] != 42.0 {
		t.Errorf("number changed: %v", m["num"])
	}
	if m["null"] != nil {
		t.Errorf("null changed: %v", m["null"])
	}
	if m["bool"] != true {
		t.Errorf("bool changed: %v", m["bool"])
	}
}

func TestTruncateSlice(t *testing.T) {
	dir := t.TempDir()
	longValue := strings.Repeat("q", MaxStringLength+50)
	input := []any{"short", longValue, 123.0}

	result, err := TruncateWithDir(input, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s, ok := result.([]any)
	if !ok {
		t.Fatalf("expected slice result, got %T", result)
	}

	if len(s) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(s))
	}

	if s[0] != "short" {
		t.Errorf("first element changed: %v", s[0])
	}

	longResult, ok := s[1].(string)
	if !ok {
		t.Fatalf("expected string for second element, got %T", s[1])
	}
	if !strings.HasPrefix(longResult, "[truncated: see ") {
		t.Errorf("long element not truncated: %q", longResult)
	}

	if s[2] != 123.0 {
		t.Errorf("third element changed: %v", s[2])
	}
}

func TestTruncateNestedMap(t *testing.T) {
	dir := t.TempDir()
	longValue := strings.Repeat("n", MaxStringLength+10)
	input := map[string]any{
		"outer": map[string]any{
			"inner": longValue,
			"ok":    "fine",
		},
	}

	result, err := TruncateWithDir(input, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]any)
	inner := m["outer"].(map[string]any)

	if inner["ok"] != "fine" {
		t.Errorf("nested ok value changed: %v", inner["ok"])
	}

	longResult := inner["inner"].(string)
	if !strings.HasPrefix(longResult, "[truncated: see ") {
		t.Errorf("nested long value not truncated: %q", longResult)
	}
}

func TestTruncateNestedSlice(t *testing.T) {
	dir := t.TempDir()
	longValue := strings.Repeat("s", MaxStringLength+10)
	input := map[string]any{
		"items": []any{
			map[string]any{"body": longValue},
		},
	}

	result, err := TruncateWithDir(input, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]any)
	items := m["items"].([]any)
	item := items[0].(map[string]any)

	bodyResult := item["body"].(string)
	if !strings.HasPrefix(bodyResult, "[truncated: see ") {
		t.Errorf("nested array item not truncated: %q", bodyResult)
	}
}

func TestTruncateNonStringTypes(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name  string
		input any
	}{
		{"number", 42.0},
		{"bool", true},
		{"nil", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := TruncateWithDir(tt.input, dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.input {
				t.Errorf("expected %v, got %v", tt.input, result)
			}
		})
	}
}

func TestCleanupTempFilesInDir(t *testing.T) {
	dir := t.TempDir()

	// Create an old file (modify its mod time)
	oldPath := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(oldPath, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-2 * time.Hour)
	os.Chtimes(oldPath, oldTime, oldTime)

	// Create a recent file
	newPath := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(newPath, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := CleanupTempFilesInDir(dir); err != nil {
		t.Fatalf("cleanup error: %v", err)
	}

	// Old file should be removed
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("expected old file to be removed")
	}

	// New file should still exist
	if _, err := os.Stat(newPath); err != nil {
		t.Error("expected new file to still exist")
	}
}

func TestCleanupNonExistentDir(t *testing.T) {
	err := CleanupTempFilesInDir("/nonexistent/dir/that/does/not/exist")
	if err != nil {
		t.Errorf("expected nil for nonexistent dir, got: %v", err)
	}
}

func TestSpillToFileInDir(t *testing.T) {
	dir := t.TempDir()
	content := "test content for spill"

	path, err := spillToFileInDir(content, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(path, dir) {
		t.Errorf("expected path in %s, got %s", dir, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read spilled file: %v", err)
	}
	if string(data) != content {
		t.Errorf("content mismatch: got %q, want %q", string(data), content)
	}
}
