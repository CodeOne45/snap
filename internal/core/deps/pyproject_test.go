package deps

import (
	"errors"
	"testing"
)

func TestAddPyprojectToml_AppendsToExistingArray(t *testing.T) {
	existing := `[project]
name = "myapp"
version = "0.1.0"
dependencies = [
    "requests>=2.0",
    "click==8.0.0",
]
`
	got, changed, err := AddPyprojectToml([]byte(existing), Dependency{Name: "flask", Version: "2.0.1"})
	if err != nil {
		t.Fatalf("AddPyprojectToml: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}
	want := `[project]
name = "myapp"
version = "0.1.0"
dependencies = [
    "requests>=2.0",
    "click==8.0.0",
    "flask==2.0.1",
]
`
	if string(got) != want {
		t.Errorf("content =\n%s\nwant:\n%s", got, want)
	}
}

func TestAddPyprojectToml_CreatesArrayIfMissing(t *testing.T) {
	existing := "[project]\nname = \"myapp\"\nversion = \"0.1.0\"\n"
	got, changed, err := AddPyprojectToml([]byte(existing), Dependency{Name: "flask", Version: "2.0.1"})
	if err != nil {
		t.Fatalf("AddPyprojectToml: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}
	want := "[project]\nname = \"myapp\"\nversion = \"0.1.0\"\ndependencies = [\n    \"flask==2.0.1\",\n]\n"
	if string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestAddPyprojectToml_Idempotent(t *testing.T) {
	existing := "[project]\ndependencies = [\n    \"flask==2.0.1\",\n]\n"
	_, changed, err := AddPyprojectToml([]byte(existing), Dependency{Name: "flask", Version: "2.0.1"})
	if err != nil {
		t.Fatalf("AddPyprojectToml: %v", err)
	}
	if changed {
		t.Error("changed = true, want false (already present at the same version)")
	}
}

func TestAddPyprojectToml_ConflictDifferentVersion(t *testing.T) {
	existing := "[project]\ndependencies = [\n    \"flask==1.0.0\",\n]\n"
	_, _, err := AddPyprojectToml([]byte(existing), Dependency{Name: "flask", Version: "2.0.1"})
	if err == nil {
		t.Fatal("AddPyprojectToml with a conflicting version: want error, got nil")
	}
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %v, want *ConflictError", err)
	}
	if conflict.Existing != "1.0.0" || conflict.Wanted != "2.0.1" {
		t.Errorf("conflict = %+v, want Existing=1.0.0 Wanted=2.0.1", conflict)
	}
}

func TestAddPyprojectToml_CaseInsensitiveNameMatch(t *testing.T) {
	existing := "[project]\ndependencies = [\n    \"Flask==2.0.1\",\n]\n"
	_, changed, err := AddPyprojectToml([]byte(existing), Dependency{Name: "flask", Version: "2.0.1"})
	if err != nil {
		t.Fatalf("AddPyprojectToml: %v", err)
	}
	if changed {
		t.Error("changed = true, want false (Flask/flask is the same package)")
	}
}

func TestAddPyprojectToml_NoProjectSectionIsError(t *testing.T) {
	existing := "[tool.poetry]\nname = \"myapp\"\n"
	if _, _, err := AddPyprojectToml([]byte(existing), Dependency{Name: "flask", Version: "2.0.1"}); err == nil {
		t.Fatal("AddPyprojectToml with no [project] section: want error, got nil")
	}
}
