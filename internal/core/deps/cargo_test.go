package deps

import (
	"errors"
	"testing"
)

func TestAddCargoToml_AppendsToExistingSection(t *testing.T) {
	existing := `[package]
name = "myapp"
version = "0.1.0"

[dependencies]
serde = "1.0"

[dev-dependencies]
tempfile = "3.0"
`
	got, changed, err := AddCargoToml([]byte(existing), Dependency{Name: "sentry", Version: "0.32.0"})
	if err != nil {
		t.Fatalf("AddCargoToml: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}
	want := `[package]
name = "myapp"
version = "0.1.0"

[dependencies]
serde = "1.0"
sentry = "0.32.0"

[dev-dependencies]
tempfile = "3.0"
`
	if string(got) != want {
		t.Errorf("content =\n%s\nwant:\n%s", got, want)
	}
}

func TestAddCargoToml_DevDependency(t *testing.T) {
	existing := "[dependencies]\nserde = \"1.0\"\n\n[dev-dependencies]\ntempfile = \"3.0\"\n"
	got, changed, err := AddCargoToml([]byte(existing), Dependency{Name: "mockall", Version: "0.11.0", Dev: true})
	if err != nil {
		t.Fatalf("AddCargoToml: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}
	want := "[dependencies]\nserde = \"1.0\"\n\n[dev-dependencies]\ntempfile = \"3.0\"\nmockall = \"0.11.0\"\n"
	if string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestAddCargoToml_CreatesSectionIfMissing(t *testing.T) {
	existing := "[package]\nname = \"myapp\"\nversion = \"0.1.0\"\n"
	got, changed, err := AddCargoToml([]byte(existing), Dependency{Name: "serde", Version: "1.0"})
	if err != nil {
		t.Fatalf("AddCargoToml: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}
	want := "[package]\nname = \"myapp\"\nversion = \"0.1.0\"\n\n[dependencies]\nserde = \"1.0\"\n"
	if string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestAddCargoToml_Idempotent(t *testing.T) {
	existing := "[dependencies]\nserde = \"1.0\"\n"
	_, changed, err := AddCargoToml([]byte(existing), Dependency{Name: "serde", Version: "1.0"})
	if err != nil {
		t.Fatalf("AddCargoToml: %v", err)
	}
	if changed {
		t.Error("changed = true, want false (already present at the same version)")
	}
}

func TestAddCargoToml_ConflictDifferentVersion(t *testing.T) {
	existing := "[dependencies]\nserde = \"1.0\"\n"
	_, _, err := AddCargoToml([]byte(existing), Dependency{Name: "serde", Version: "2.0"})
	if err == nil {
		t.Fatal("AddCargoToml with a conflicting version: want error, got nil")
	}
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %v, want *ConflictError", err)
	}
	if conflict.Existing != "1.0" || conflict.Wanted != "2.0" {
		t.Errorf("conflict = %+v, want Existing=1.0 Wanted=2.0", conflict)
	}
}
