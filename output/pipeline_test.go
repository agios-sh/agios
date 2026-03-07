package output

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestProcessSimple(t *testing.T) {
	os.Setenv("AGIOS_FORMAT", "json")
	defer os.Unsetenv("AGIOS_FORMAT")

	input := map[string]any{
		"name":   "test",
		"status": "ok",
	}

	data, err := Process(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("expected name=test, got %v", result["name"])
	}
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", result["status"])
	}
}

func TestProcessWithLargeValue(t *testing.T) {
	os.Setenv("AGIOS_FORMAT", "json")
	defer os.Unsetenv("AGIOS_FORMAT")

	longValue := strings.Repeat("a", MaxStringLength+100)
	input := map[string]any{
		"short": "hello",
		"long":  longValue,
	}

	data, err := Process(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if result["short"] != "hello" {
		t.Errorf("short value changed: %v", result["short"])
	}

	longResult, ok := result["long"].(string)
	if !ok {
		t.Fatalf("expected string for long value, got %T", result["long"])
	}
	if !strings.HasPrefix(longResult, "[truncated: see ") {
		t.Errorf("long value not truncated in output: %q", longResult)
	}
}

func TestProcessPreservesTypes(t *testing.T) {
	os.Setenv("AGIOS_FORMAT", "json")
	defer os.Unsetenv("AGIOS_FORMAT")

	input := map[string]any{
		"str":   "hello",
		"num":   42.0,
		"bool":  true,
		"null":  nil,
		"array": []any{1.0, 2.0, 3.0},
	}

	data, err := Process(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if result["str"] != "hello" {
		t.Errorf("str changed: %v", result["str"])
	}
	if result["num"] != 42.0 {
		t.Errorf("num changed: %v", result["num"])
	}
	if result["bool"] != true {
		t.Errorf("bool changed: %v", result["bool"])
	}
	if result["null"] != nil {
		t.Errorf("null changed: %v", result["null"])
	}
}

func TestProcessNestedTruncation(t *testing.T) {
	os.Setenv("AGIOS_FORMAT", "json")
	defer os.Unsetenv("AGIOS_FORMAT")

	longValue := strings.Repeat("b", MaxStringLength+50)
	input := map[string]any{
		"notifications": []any{
			map[string]any{
				"id":   "1",
				"body": longValue,
			},
		},
	}

	data, err := Process(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	notifs := result["notifications"].([]any)
	notif := notifs[0].(map[string]any)

	if notif["id"] != "1" {
		t.Errorf("id changed: %v", notif["id"])
	}

	body := notif["body"].(string)
	if !strings.HasPrefix(body, "[truncated: see ") {
		t.Errorf("nested body not truncated: %q", body)
	}
}

func TestProcessDefaultsTOON(t *testing.T) {
	// Without AGIOS_FORMAT=json, output should be TOON (not JSON)
	os.Unsetenv("AGIOS_FORMAT")

	input := map[string]any{
		"name":   "test",
		"status": "ok",
	}

	data, err := Process(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)
	// TOON uses key: value format, not JSON braces
	if strings.HasPrefix(out, "{") {
		t.Error("expected TOON output, got JSON")
	}
	if !strings.Contains(out, "name: test") {
		t.Errorf("expected 'name: test' in TOON output: %s", out)
	}
	if !strings.Contains(out, "status: ok") {
		t.Errorf("expected 'status: ok' in TOON output: %s", out)
	}
}

func TestProcessNormalizesStructs(t *testing.T) {
	os.Setenv("AGIOS_FORMAT", "json")
	defer os.Unsetenv("AGIOS_FORMAT")

	type item struct {
		Name  string `json:"name"`
		Value string `json:"value,omitempty"`
	}

	input := map[string]any{
		"item": item{Name: "test"},
	}

	data, err := Process(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	itemMap := result["item"].(map[string]any)
	if itemMap["name"] != "test" {
		t.Errorf("expected name=test, got %v", itemMap["name"])
	}
	// omitempty should omit the empty value field
	if _, exists := itemMap["value"]; exists {
		t.Error("expected 'value' to be omitted (omitempty)")
	}
}
