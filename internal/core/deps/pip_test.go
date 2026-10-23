package deps

import (
	"errors"
	"testing"
)

func TestAddRequirementsTxt_Empty(t *testing.T) {
	got, changed, err := AddRequirementsTxt(nil, Dependency{Name: "flask", Version: "2.0.1"})
	if err != nil {
		t.Fatalf("AddRequirementsTxt: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}
	if string(got) != "flask==2.0.1\n" {
		t.Errorf("content = %q, want %q", got, "flask==2.0.1\n")
	}
}

func TestAddRequirementsTxt_NoVersion(t *testing.T) {
	got, changed, err := AddRequirementsTxt(nil, Dependency{Name: "requests"})
	if err != nil {
		t.Fatalf("AddRequirementsTxt: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}
	if string(got) != "requests\n" {
		t.Errorf("content = %q, want %q", got, "requests\n")
	}
}

func TestAddRequirementsTxt_AppendsPreservingExisting(t *testing.T) {
	existing := "# core deps\nrequests>=2.0\n\nclick==8.0.0\n"
	got, changed, err := AddRequirementsTxt([]byte(existing), Dependency{Name: "flask", Version: "2.0.1"})
	if err != nil {
		t.Fatalf("AddRequirementsTxt: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}
	want := existing + "flask==2.0.1\n"
	if string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestAddRequirementsTxt_Idempotent(t *testing.T) {
	existing := "flask==2.0.1\n"
	got, changed, err := AddRequirementsTxt([]byte(existing), Dependency{Name: "flask", Version: "2.0.1"})
	if err != nil {
		t.Fatalf("AddRequirementsTxt: %v", err)
	}
	if changed {
		t.Error("changed = true, want false (already present at the same version)")
	}
	if string(got) != existing {
		t.Errorf("content = %q, want it unchanged: %q", got, existing)
	}
}

func TestAddRequirementsTxt_CaseInsensitiveNameMatch(t *testing.T) {
	existing := "Flask==2.0.1\n"
	_, changed, err := AddRequirementsTxt([]byte(existing), Dependency{Name: "flask", Version: "2.0.1"})
	if err != nil {
		t.Fatalf("AddRequirementsTxt: %v", err)
	}
	if changed {
		t.Error("changed = true, want false (Flask/flask is the same package)")
	}
}

func TestAddRequirementsTxt_ConflictDifferentVersion(t *testing.T) {
	existing := "flask==1.0.0\n"
	_, _, err := AddRequirementsTxt([]byte(existing), Dependency{Name: "flask", Version: "2.0.1"})
	if err == nil {
		t.Fatal("AddRequirementsTxt with a conflicting version: want error, got nil")
	}
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %v, want *ConflictError", err)
	}
	if conflict.Name != "flask" || conflict.Existing != "1.0.0" || conflict.Wanted != "2.0.1" {
		t.Errorf("conflict = %+v, want Name=flask Existing=1.0.0 Wanted=2.0.1", conflict)
	}
}

func TestAddRequirementsTxt_NothingWrittenOnConflict(t *testing.T) {
	existing := "flask==1.0.0\n"
	got, _, err := AddRequirementsTxt([]byte(existing), Dependency{Name: "flask", Version: "2.0.1"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if got != nil {
		t.Errorf("content = %q, want nil on conflict", got)
	}
}
