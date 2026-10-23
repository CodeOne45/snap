package hookrunner

import (
	"strings"
	"testing"

	"github.com/CodeOne45/snap/internal/ports"
)

func TestRun_Success(t *testing.T) {
	dir := t.TempDir()
	result, err := New().Run(ports.Hook{Run: "echo hello", Dir: dir})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(result.Output, "hello") {
		t.Errorf("Output = %q, want it to contain %q", result.Output, "hello")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestRun_Failure(t *testing.T) {
	dir := t.TempDir()
	result, err := New().Run(ports.Hook{Run: "exit 7", Dir: dir})
	if err == nil {
		t.Fatal("Run(exit 7): want error, got nil")
	}
	if result.ExitCode != 7 {
		t.Errorf("ExitCode = %d, want 7", result.ExitCode)
	}
}

func TestRun_RunsInGivenDir(t *testing.T) {
	dir := t.TempDir()
	result, err := New().Run(ports.Hook{Run: "pwd", Dir: dir})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(result.Output, dir) {
		t.Errorf("Output = %q, want it to contain the working dir %q", result.Output, dir)
	}
}

func TestRun_OutputCapturedOnFailure(t *testing.T) {
	dir := t.TempDir()
	result, err := New().Run(ports.Hook{Run: "echo oops >&2; exit 1", Dir: dir})
	if err == nil {
		t.Fatal("Run: want error, got nil")
	}
	if !strings.Contains(result.Output, "oops") {
		t.Errorf("Output = %q, want it to contain stderr output even on failure", result.Output)
	}
}
