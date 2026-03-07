package cmd

import (
	"encoding/json"
	"testing"

	"github.com/agios-sh/agios/peek"
)

func TestHomePeek(t *testing.T) {
	// Build a mock app that responds to both "status" and "peek"
	dir := buildMockBinary(t, "peek-app", `package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}
	switch os.Args[1] {
	case "status":
		fmt.Println("{\"name\": \"peek-app\", \"description\": \"Test peek app\", \"status\": \"ok\"}")
	case "peek":
		fmt.Println("{\"open\": 5, \"recent\": [{\"id\": \"1\", \"title\": \"Fix bug\"}]}")
	}
}
`)
	withPathPrepend(t, dir)

	// Test FetchAll
	results := peek.FetchAll([]string{"peek-app"})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].AppName != "peek-app" {
		t.Errorf("expected app name peek-app, got %s", results[0].AppName)
	}

	if results[0].Peek == nil {
		t.Fatal("expected non-nil peek data")
	}

	if results[0].Peek["open"].(float64) != 5 {
		t.Errorf("expected open=5, got %v", results[0].Peek["open"])
	}

	// Build app entries
	apps := make([]peek.AppEntry, len(results))
	for i, r := range results {
		apps[i] = peek.AppEntry{
			Name:    r.AppName,
			Summary: r.Description,
			Peek:    r.Peek,
			Error:   r.Error,
		}
	}

	if apps[0].Name != "peek-app" {
		t.Errorf("expected name peek-app, got %s", apps[0].Name)
	}

	// Verify the output serializes correctly
	output := map[string]any{
		"apps": apps,
		"help": []string{"Run `agios <app>` to see an app's current state"},
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	appsArr := parsed["apps"].([]any)
	app := appsArr[0].(map[string]any)
	peekData := app["peek"].(map[string]any)
	if peekData["open"].(float64) != 5 {
		t.Errorf("expected open=5 in JSON, got %v", peekData["open"])
	}
}

func TestHomePeekMissingApp(t *testing.T) {
	// Test with a non-existent binary
	results := peek.FetchAll([]string{"nonexistent-binary-xyz-99999"})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Error == "" {
		t.Error("expected error for missing binary")
	}

	// Build app entry
	entry := peek.AppEntry{
		Name:  results[0].AppName,
		Error: results[0].Error,
	}

	if entry.Error == "" {
		t.Error("expected error to propagate to app entry")
	}
}

func TestHomePeekEmpty(t *testing.T) {
	// Build a mock app that returns empty peek
	dir := buildMockBinary(t, "empty-peek-app", `package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}
	switch os.Args[1] {
	case "status":
		fmt.Println("{\"name\": \"empty-peek-app\", \"description\": \"App with no peek data\", \"status\": \"ok\"}")
	case "peek":
		fmt.Println("{}")
	}
}
`)
	withPathPrepend(t, dir)

	results := peek.FetchAll([]string{"empty-peek-app"})

	if results[0].Peek == nil || len(results[0].Peek) != 0 {
		t.Errorf("expected empty peek, got %v", results[0].Peek)
	}
}

func TestHomeFetchDescription(t *testing.T) {
	// Build an app that returns description in status
	dir := buildMockBinary(t, "desc-app", `package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}
	switch os.Args[1] {
	case "status":
		fmt.Println("{\"name\": \"desc-app\", \"description\": \"A test app with description\", \"status\": \"ok\"}")
	case "peek":
		fmt.Println("{}")
	}
}
`)
	withPathPrepend(t, dir)

	results := peek.FetchAll([]string{"desc-app"})

	if results[0].Description != "A test app with description" {
		t.Errorf("expected description, got %q", results[0].Description)
	}
}
