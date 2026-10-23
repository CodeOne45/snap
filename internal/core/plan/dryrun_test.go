package plan

import (
	"strings"
	"testing"
)

func TestDryRun(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.CreateDir("src"); err != nil {
		t.Fatalf("CreateDir: %v", err)
	}
	if err := b.WriteFile("src/main.go", []byte("package main"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := b.RunHook("git init"); err != nil {
		t.Fatalf("RunHook: %v", err)
	}

	var buf strings.Builder
	if err := DryRun(&buf, b.Build()); err != nil {
		t.Fatalf("DryRun: %v", err)
	}

	got := buf.String()
	wantLines := []string{
		"Plan for myproj:",
		"+ dir   src",
		"+ file  src/main.go (12 bytes)",
		"hook  git init",
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != len(wantLines) {
		t.Fatalf("DryRun output = %q, want %d lines got %d", got, len(wantLines), len(lines))
	}
	for i, want := range wantLines {
		if !strings.Contains(lines[i], want) {
			t.Errorf("line %d = %q, want it to contain %q", i, lines[i], want)
		}
	}
}
