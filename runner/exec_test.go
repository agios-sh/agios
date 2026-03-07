package runner

import (
	"os"
	"testing"
	"time"
)

func TestExecCapturesStdout(t *testing.T) {
	// Use "echo" to produce known stdout
	result, err := Exec("/bin/echo", []string{"hello world"}, DefaultTimeout)
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if string(result.Stdout) != "hello world\n" {
		t.Errorf("expected 'hello world\\n', got %q", string(result.Stdout))
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestExecCapturesStderr(t *testing.T) {
	// Use sh -c to write to stderr
	result, err := Exec("/bin/sh", []string{"-c", "echo error >&2"}, DefaultTimeout)
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if string(result.Stderr) != "error\n" {
		t.Errorf("expected 'error\\n' on stderr, got %q", string(result.Stderr))
	}
}

func TestExecNonZeroExit(t *testing.T) {
	result, err := Exec("/bin/sh", []string{"-c", "exit 42"}, DefaultTimeout)
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	if result.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", result.ExitCode)
	}
}

func TestExecTimeout(t *testing.T) {
	_, err := Exec("/bin/sleep", []string{"10"}, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestExecPassesEnvVars(t *testing.T) {
	os.Setenv("AGIOS_FRESH", "1")
	defer os.Unsetenv("AGIOS_FRESH")

	result, err := Exec("/bin/sh", []string{"-c", "echo $AGIOS_FRESH"}, DefaultTimeout)
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if string(result.Stdout) != "1\n" {
		t.Errorf("expected AGIOS_FRESH=1, got %q", string(result.Stdout))
	}
}

func TestExecDefaultTimeout(t *testing.T) {
	// Pass 0 timeout — should use DefaultTimeout (5s), which is plenty for echo
	result, err := Exec("/bin/echo", []string{"ok"}, 0)
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if string(result.Stdout) != "ok\n" {
		t.Errorf("expected 'ok\\n', got %q", string(result.Stdout))
	}
}
