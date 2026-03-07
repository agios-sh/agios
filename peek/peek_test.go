package peek

import (
	"encoding/json"
	"testing"
)

func TestParsePeekObject(t *testing.T) {
	data := `{"tabs": [{"title": "Google", "url": "https://google.com"}], "active_tab": 0}`

	result, err := parsePeek([]byte(data))
	if err != nil {
		t.Fatalf("parsePeek: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	tabs, ok := result["tabs"]
	if !ok {
		t.Error("expected tabs key in result")
	}

	tabList, ok := tabs.([]any)
	if !ok || len(tabList) != 1 {
		t.Errorf("expected 1 tab, got %v", tabs)
	}
}

func TestParsePeekEmpty(t *testing.T) {
	data := `{}`

	result, err := parsePeek([]byte(data))
	if err != nil {
		t.Fatalf("parsePeek: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestParsePeekJSONL(t *testing.T) {
	data := `{"progress": "fetching..."}
{"sessions": [{"id": 1, "name": "dev"}], "active_id": 1}`

	result, err := parsePeek([]byte(data))
	if err != nil {
		t.Fatalf("parsePeek: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if _, ok := result["sessions"]; !ok {
		t.Error("expected sessions key in JSONL result")
	}
}

func TestParsePeekInvalid(t *testing.T) {
	_, err := parsePeek([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestFetchAllMissingApp(t *testing.T) {
	results := FetchAll([]string{"nonexistent-binary-xyz-99999"})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Error == "" {
		t.Error("expected error for missing binary")
	}
}

func TestAppEntrySerialization(t *testing.T) {
	entry := AppEntry{
		Name:    "test-app",
		Summary: "A test app",
		Peek:    map[string]any{"open": 5, "closed": 12},
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if parsed["name"] != "test-app" {
		t.Errorf("expected name test-app, got %v", parsed["name"])
	}

	peek, ok := parsed["peek"].(map[string]any)
	if !ok {
		t.Fatalf("expected peek map, got %v", parsed["peek"])
	}

	if peek["open"].(float64) != 5 {
		t.Errorf("expected open=5, got %v", peek["open"])
	}
}

func TestAppEntryOmitsEmptyPeek(t *testing.T) {
	entry := AppEntry{
		Name:    "test-app",
		Summary: "A test app",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if _, ok := parsed["peek"]; ok {
		t.Error("expected peek to be omitted when nil")
	}
}
