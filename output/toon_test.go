package output

import (
	"strings"
	"testing"
)

func TestToTOONObject(t *testing.T) {
	input := map[string]any{
		"name":   "test",
		"count":  42.0,
		"active": true,
	}

	data, err := ToTOON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)
	// TOON uses key: value format
	if !strings.Contains(out, "name: test") {
		t.Errorf("expected 'name: test' in output: %s", out)
	}
	if !strings.Contains(out, "count: 42") {
		t.Errorf("expected 'count: 42' in output: %s", out)
	}
	if !strings.Contains(out, "active: true") {
		t.Errorf("expected 'active: true' in output: %s", out)
	}
}

func TestToTOONPrimitiveArray(t *testing.T) {
	input := map[string]any{
		"tags": []any{"foo", "bar", "baz"},
	}

	data, err := ToTOON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)
	// Primitive arrays use inline format: key[N]: v1,v2,v3
	if !strings.Contains(out, "tags[3]:") {
		t.Errorf("expected 'tags[3]:' in output: %s", out)
	}
	if !strings.Contains(out, "foo") || !strings.Contains(out, "bar") || !strings.Contains(out, "baz") {
		t.Errorf("expected all tag values in output: %s", out)
	}
}

func TestToTOONTabularArray(t *testing.T) {
	input := map[string]any{
		"users": []any{
			map[string]any{"id": 1.0, "name": "Alice"},
			map[string]any{"id": 2.0, "name": "Bob"},
		},
	}

	data, err := ToTOON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)
	// Tabular arrays: key[N]{fields}: with rows
	if !strings.Contains(out, "users[2]") {
		t.Errorf("expected 'users[2]' header in output: %s", out)
	}
	if !strings.Contains(out, "Alice") || !strings.Contains(out, "Bob") {
		t.Errorf("expected user names in output: %s", out)
	}
}

func TestToTOONNestedObject(t *testing.T) {
	input := map[string]any{
		"server": map[string]any{
			"host": "localhost",
			"port": 8080.0,
		},
	}

	data, err := ToTOON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "server:") {
		t.Errorf("expected 'server:' in output: %s", out)
	}
	if !strings.Contains(out, "host: localhost") {
		t.Errorf("expected 'host: localhost' in output: %s", out)
	}
	if !strings.Contains(out, "port: 8080") {
		t.Errorf("expected 'port: 8080' in output: %s", out)
	}
}

func TestToTOONNil(t *testing.T) {
	data, err := ToTOON(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(data) != "null" {
		t.Errorf("expected null, got %s", string(data))
	}
}

func TestToTOONStringQuoting(t *testing.T) {
	input := map[string]any{
		"simple":  "hello world",
		"special": "has:colon",
		"keyword": "true",
		"empty":   "",
	}

	data, err := ToTOON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)
	// Simple strings should not be quoted
	if !strings.Contains(out, "simple: hello world") {
		t.Errorf("expected unquoted 'hello world' in output: %s", out)
	}
	// Strings with colons must be quoted
	if !strings.Contains(out, `special: "has:colon"`) {
		t.Errorf("expected quoted 'has:colon' in output: %s", out)
	}
	// Keywords must be quoted
	if !strings.Contains(out, `keyword: "true"`) {
		t.Errorf("expected quoted 'true' in output: %s", out)
	}
}

func TestToTOONErrorResponse(t *testing.T) {
	// Test a typical agios error response
	input := map[string]any{
		"error": "App not found",
		"code":  "NOT_FOUND",
		"help":  []any{"Run `agios status` to see configured apps"},
	}

	data, err := ToTOON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "code: NOT_FOUND") {
		t.Errorf("expected 'code: NOT_FOUND' in output: %s", out)
	}
	if !strings.Contains(out, "error: App not found") {
		t.Errorf("expected 'error: App not found' in output: %s", out)
	}
	if !strings.Contains(out, "help[1]:") {
		t.Errorf("expected 'help[1]:' in output: %s", out)
	}
}
