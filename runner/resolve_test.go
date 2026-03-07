package runner

import (
	"testing"
)

func TestResolveFindsExistingBinary(t *testing.T) {
	// "echo" should exist on any Unix system
	path, err := Resolve("echo")
	if err != nil {
		t.Fatalf("Resolve(echo) returned error: %v", err)
	}
	if path == "" {
		t.Fatal("Resolve(echo) returned empty path")
	}
}

func TestResolveReturnsErrorForMissing(t *testing.T) {
	_, err := Resolve("nonexistent-binary-xyz-12345")
	if err == nil {
		t.Fatal("Resolve should return error for nonexistent binary")
	}
}
