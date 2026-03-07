package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCollectAppStatusesMissingBinary(t *testing.T) {
	results := collectAppStatuses([]string{"nonexistent-binary-xyz-99999"})

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	r := results[0]
	if r.Name != "nonexistent-binary-xyz-99999" {
		t.Errorf("Name = %q, want %q", r.Name, "nonexistent-binary-xyz-99999")
	}
	if r.Status != "error" {
		t.Errorf("Status = %q, want %q", r.Status, "error")
	}
	if r.Error == "" {
		t.Error("expected Error to be set for missing binary")
	}
}

func TestCollectAppStatusesEmptyList(t *testing.T) {
	results := collectAppStatuses([]string{})
	if len(results) != 0 {
		t.Fatalf("got %d results, want 0", len(results))
	}
}

// buildMockBinary compiles a Go source file into a binary in the given directory.
// Returns the directory containing the binary (for adding to PATH).
func buildMockBinary(t *testing.T, name, src string) string {
	t.Helper()
	dir := t.TempDir()
	srcPath := filepath.Join(dir, name+".go")
	os.WriteFile(srcPath, []byte(src), 0644)

	binPath := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	build := exec.Command("go", "build", "-o", binPath, srcPath)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building %s: %v\n%s", name, err, out)
	}

	return dir
}

// withPathPrepend temporarily adds a directory to PATH for the test.
func withPathPrepend(t *testing.T, dir string) {
	t.Helper()
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })
}

func TestCollectAppStatusesSuccessful(t *testing.T) {
	dir := buildMockBinary(t, "healthy-app", `package main

import "fmt"

func main() {
	fmt.Println("{\"status\": \"ok\", \"version\": \"2.1.0\", \"user\": \"alice@example.com\"}")
}
`)
	withPathPrepend(t, dir)

	results := collectAppStatuses([]string{"healthy-app"})
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}

	r := results[0]
	if r.Name != "healthy-app" {
		t.Errorf("Name = %q, want %q", r.Name, "healthy-app")
	}
	if r.Status != "ok" {
		t.Errorf("Status = %q, want %q", r.Status, "ok")
	}
	if r.Version != "2.1.0" {
		t.Errorf("Version = %q, want %q", r.Version, "2.1.0")
	}
	if r.User != "alice@example.com" {
		t.Errorf("User = %q, want %q", r.User, "alice@example.com")
	}
	if r.Error != "" {
		t.Errorf("Error = %q, want empty", r.Error)
	}
}

func TestCollectAppStatusesMixedResults(t *testing.T) {
	// Build a healthy app
	dir := buildMockBinary(t, "good-app", `package main

import "fmt"

func main() {
	fmt.Println("{\"status\": \"ok\", \"version\": \"1.0.0\"}")
}
`)
	withPathPrepend(t, dir)

	// Run with one good app and one missing app
	results := collectAppStatuses([]string{"good-app", "nonexistent-binary-xyz-99999"})
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	// Results maintain order
	if results[0].Name != "good-app" {
		t.Errorf("results[0].Name = %q, want %q", results[0].Name, "good-app")
	}
	if results[0].Status != "ok" {
		t.Errorf("results[0].Status = %q, want %q", results[0].Status, "ok")
	}

	if results[1].Name != "nonexistent-binary-xyz-99999" {
		t.Errorf("results[1].Name = %q, want %q", results[1].Name, "nonexistent-binary-xyz-99999")
	}
	if results[1].Status != "error" {
		t.Errorf("results[1].Status = %q, want %q", results[1].Status, "error")
	}
	if results[1].Error == "" {
		t.Error("expected Error for missing binary")
	}
}

func TestQueryAppStatusInvalidOutput(t *testing.T) {
	dir := buildMockBinary(t, "bad-output-app", `package main

import "fmt"

func main() {
	fmt.Println("this is not json at all")
}
`)
	withPathPrepend(t, dir)

	r := queryAppStatus("bad-output-app")
	if r.Status != "error" {
		t.Errorf("Status = %q, want %q", r.Status, "error")
	}
	if r.Error == "" {
		t.Error("expected Error for invalid output")
	}
}

func TestQueryAppStatusNonZeroExit(t *testing.T) {
	dir := buildMockBinary(t, "failing-app", `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("{\"status\": \"error\", \"error\": \"auth expired\"}")
	os.Exit(1)
}
`)
	withPathPrepend(t, dir)

	r := queryAppStatus("failing-app")
	if r.Name != "failing-app" {
		t.Errorf("Name = %q, want %q", r.Name, "failing-app")
	}
	if r.Status != "error" {
		t.Errorf("Status = %q, want %q", r.Status, "error")
	}
	if r.Error != "auth expired" {
		t.Errorf("Error = %q, want %q", r.Error, "auth expired")
	}
}

func TestQueryAppStatusNoOutputOnExit(t *testing.T) {
	dir := buildMockBinary(t, "silent-app", `package main

import "os"

func main() {
	os.Exit(1)
}
`)
	withPathPrepend(t, dir)

	r := queryAppStatus("silent-app")
	if r.Status != "error" {
		t.Errorf("Status = %q, want %q", r.Status, "error")
	}
	if r.Error == "" {
		t.Error("expected Error for silent failing app")
	}
}

func TestCollectAppStatusesConcurrency(t *testing.T) {
	// Build 3 identical apps to verify concurrency works correctly
	dirs := make([]string, 3)
	names := []string{"app-a", "app-b", "app-c"}

	for i, name := range names {
		dirs[i] = buildMockBinary(t, name, `package main

import "fmt"

func main() {
	fmt.Println("{\"status\": \"ok\", \"version\": \"1.0.0\"}")
}
`)
	}

	origPath := os.Getenv("PATH")
	pathParts := ""
	for _, d := range dirs {
		pathParts += d + string(os.PathListSeparator)
	}
	os.Setenv("PATH", pathParts+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	results := collectAppStatuses(names)
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	// All should succeed and maintain order
	for i, name := range names {
		if results[i].Name != name {
			t.Errorf("results[%d].Name = %q, want %q", i, results[i].Name, name)
		}
		if results[i].Status != "ok" {
			t.Errorf("results[%d].Status = %q, want %q", i, results[i].Status, "ok")
		}
	}
}
