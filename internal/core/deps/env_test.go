package deps

import (
	"errors"
	"testing"
)

func TestAddEnvEntry_AppendsNewKey(t *testing.T) {
	existing := "DATABASE_URL=postgres://localhost/app\n"
	got, changed, err := AddEnvEntry([]byte(existing), "SENTRY_DSN", "")
	if err != nil {
		t.Fatalf("AddEnvEntry: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}
	want := "DATABASE_URL=postgres://localhost/app\nSENTRY_DSN=\n"
	if string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestAddEnvEntry_AppendsToEmptyFile(t *testing.T) {
	got, changed, err := AddEnvEntry([]byte(""), "SENTRY_DSN", "https://example.com")
	if err != nil {
		t.Fatalf("AddEnvEntry: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}
	if want := "SENTRY_DSN=https://example.com\n"; string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestAddEnvEntry_MissingTrailingNewlineBeforeAppend(t *testing.T) {
	got, _, err := AddEnvEntry([]byte("DATABASE_URL=x"), "SENTRY_DSN", "")
	if err != nil {
		t.Fatalf("AddEnvEntry: %v", err)
	}
	if want := "DATABASE_URL=x\nSENTRY_DSN=\n"; string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestAddEnvEntry_Idempotent(t *testing.T) {
	existing := "SENTRY_DSN=https://example.com\n"
	_, changed, err := AddEnvEntry([]byte(existing), "SENTRY_DSN", "https://example.com")
	if err != nil {
		t.Fatalf("AddEnvEntry: %v", err)
	}
	if changed {
		t.Error("changed = true, want false (already present at the same value)")
	}
}

func TestAddEnvEntry_ConflictDifferentValue(t *testing.T) {
	existing := "SENTRY_DSN=https://old.example.com\n"
	_, _, err := AddEnvEntry([]byte(existing), "SENTRY_DSN", "https://new.example.com")
	if err == nil {
		t.Fatal("AddEnvEntry with a conflicting value: want error, got nil")
	}
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %v, want *ConflictError", err)
	}
	if conflict.Existing != "https://old.example.com" || conflict.Wanted != "https://new.example.com" {
		t.Errorf("conflict = %+v, want Existing=https://old.example.com Wanted=https://new.example.com", conflict)
	}
}

func TestAddEnvEntry_IgnoresCommentsAndBlankLines(t *testing.T) {
	existing := "# comment\n\nDATABASE_URL=x\n"
	got, changed, err := AddEnvEntry([]byte(existing), "SENTRY_DSN", "y")
	if err != nil {
		t.Fatalf("AddEnvEntry: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}
	want := "# comment\n\nDATABASE_URL=x\nSENTRY_DSN=y\n"
	if string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}
