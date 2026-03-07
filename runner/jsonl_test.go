package runner

import (
	"testing"
)

func TestParseJSONLSingleResult(t *testing.T) {
	data := []byte(`{"name": "test-app", "status": "ok"}` + "\n")
	parsed, err := ParseJSONL(data)
	if err != nil {
		t.Fatalf("ParseJSONL returned error: %v", err)
	}
	if len(parsed.Progress) != 0 {
		t.Errorf("expected 0 progress lines, got %d", len(parsed.Progress))
	}
	if parsed.Result == nil {
		t.Fatal("expected non-nil result")
	}
	if parsed.Result["name"] != "test-app" {
		t.Errorf("expected name=test-app, got %v", parsed.Result["name"])
	}
}

func TestParseJSONLWithProgress(t *testing.T) {
	data := []byte(
		`{"progress": {"message": "Loading...", "percent": 30}}` + "\n" +
			`{"progress": {"message": "Almost done", "percent": 90}}` + "\n" +
			`{"items": ["a", "b"], "help": ["next step"]}` + "\n",
	)
	parsed, err := ParseJSONL(data)
	if err != nil {
		t.Fatalf("ParseJSONL returned error: %v", err)
	}
	if len(parsed.Progress) != 2 {
		t.Errorf("expected 2 progress lines, got %d", len(parsed.Progress))
	}
	if parsed.Result == nil {
		t.Fatal("expected non-nil result")
	}
	items, ok := parsed.Result["items"].([]any)
	if !ok {
		t.Fatal("expected items to be a slice")
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestParseJSONLLastNonProgressWins(t *testing.T) {
	data := []byte(
		`{"result": "first"}` + "\n" +
			`{"progress": {"message": "working"}}` + "\n" +
			`{"result": "second"}` + "\n",
	)
	parsed, err := ParseJSONL(data)
	if err != nil {
		t.Fatalf("ParseJSONL returned error: %v", err)
	}
	if parsed.Result["result"] != "second" {
		t.Errorf("expected last non-progress line to win, got %v", parsed.Result["result"])
	}
	if len(parsed.Progress) != 1 {
		t.Errorf("expected 1 progress line, got %d", len(parsed.Progress))
	}
}

func TestParseJSONLEmptyOutput(t *testing.T) {
	_, err := ParseJSONL([]byte(""))
	if err == nil {
		t.Fatal("expected error for empty output")
	}
	invErr, ok := err.(*InvalidOutputError)
	if !ok {
		t.Fatalf("expected InvalidOutputError, got %T", err)
	}
	if invErr.Message != "empty output" {
		t.Errorf("expected 'empty output' message, got %q", invErr.Message)
	}
}

func TestParseJSONLOnlyBlankLines(t *testing.T) {
	_, err := ParseJSONL([]byte("\n\n  \n"))
	if err == nil {
		t.Fatal("expected error for blank-only output")
	}
}

func TestParseJSONLInvalidJSON(t *testing.T) {
	data := []byte("this is not json\n")
	_, err := ParseJSONL(data)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	invErr, ok := err.(*InvalidOutputError)
	if !ok {
		t.Fatalf("expected InvalidOutputError, got %T", err)
	}
	if invErr.Raw != "this is not json\n" {
		t.Errorf("expected raw output in error, got %q", invErr.Raw)
	}
}

func TestParseJSONLOnlyProgressLines(t *testing.T) {
	data := []byte(`{"progress": {"message": "working"}}` + "\n")
	_, err := ParseJSONL(data)
	if err == nil {
		t.Fatal("expected error when all lines are progress")
	}
	invErr, ok := err.(*InvalidOutputError)
	if !ok {
		t.Fatalf("expected InvalidOutputError, got %T", err)
	}
	if invErr.Message != "no result line found (all lines were progress updates)" {
		t.Errorf("unexpected error message: %q", invErr.Message)
	}
}

func TestParseJSONLSkipsBlankLines(t *testing.T) {
	data := []byte("\n" + `{"status": "ok"}` + "\n\n")
	parsed, err := ParseJSONL(data)
	if err != nil {
		t.Fatalf("ParseJSONL returned error: %v", err)
	}
	if parsed.Result["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", parsed.Result["status"])
	}
}
