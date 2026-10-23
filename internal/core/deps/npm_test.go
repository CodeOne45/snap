package deps

import (
	"errors"
	"testing"
)

func TestAddPackageJSON_AppendsToExistingDependencies(t *testing.T) {
	existing := `{
  "name": "myapp",
  "version": "0.1.0",
  "dependencies": {
    "express": "^4.18.0"
  },
  "devDependencies": {
    "jest": "^29.0.0"
  }
}
`
	got, changed, err := AddPackageJSON([]byte(existing), Dependency{Name: "@sentry/node", Version: "^8.0.0"})
	if err != nil {
		t.Fatalf("AddPackageJSON: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}
	want := `{
  "name": "myapp",
  "version": "0.1.0",
  "dependencies": {
    "express": "^4.18.0",
    "@sentry/node": "^8.0.0"
  },
  "devDependencies": {
    "jest": "^29.0.0"
  }
}
`
	if string(got) != want {
		t.Errorf("content =\n%s\nwant:\n%s", got, want)
	}
}

func TestAddPackageJSON_DevDependency(t *testing.T) {
	existing := "{\n  \"dependencies\": {\n    \"express\": \"^4.18.0\"\n  },\n  \"devDependencies\": {\n    \"jest\": \"^29.0.0\"\n  }\n}\n"
	got, changed, err := AddPackageJSON([]byte(existing), Dependency{Name: "eslint", Version: "^8.0.0", Dev: true})
	if err != nil {
		t.Fatalf("AddPackageJSON: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}
	want := "{\n  \"dependencies\": {\n    \"express\": \"^4.18.0\"\n  },\n  \"devDependencies\": {\n    \"jest\": \"^29.0.0\",\n    \"eslint\": \"^8.0.0\"\n  }\n}\n"
	if string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestAddPackageJSON_CreatesSectionIfMissing(t *testing.T) {
	existing := "{\n  \"name\": \"myapp\",\n  \"version\": \"0.1.0\"\n}\n"
	got, changed, err := AddPackageJSON([]byte(existing), Dependency{Name: "express", Version: "^4.18.0"})
	if err != nil {
		t.Fatalf("AddPackageJSON: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}
	want := "{\n  \"name\": \"myapp\",\n  \"version\": \"0.1.0\",\n  \"dependencies\": {\n    \"express\": \"^4.18.0\"\n  }\n}\n"
	if string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestAddPackageJSON_Idempotent(t *testing.T) {
	existing := "{\n  \"dependencies\": {\n    \"express\": \"^4.18.0\"\n  }\n}\n"
	_, changed, err := AddPackageJSON([]byte(existing), Dependency{Name: "express", Version: "^4.18.0"})
	if err != nil {
		t.Fatalf("AddPackageJSON: %v", err)
	}
	if changed {
		t.Error("changed = true, want false (already present at the same version)")
	}
}

func TestAddPackageJSON_ConflictDifferentVersion(t *testing.T) {
	existing := "{\n  \"dependencies\": {\n    \"express\": \"^4.18.0\"\n  }\n}\n"
	_, _, err := AddPackageJSON([]byte(existing), Dependency{Name: "express", Version: "^5.0.0"})
	if err == nil {
		t.Fatal("AddPackageJSON with a conflicting version: want error, got nil")
	}
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %v, want *ConflictError", err)
	}
	if conflict.Existing != "^4.18.0" || conflict.Wanted != "^5.0.0" {
		t.Errorf("conflict = %+v, want Existing=^4.18.0 Wanted=^5.0.0", conflict)
	}
}

func TestAddPackageJSON_InvalidJSON(t *testing.T) {
	if _, _, err := AddPackageJSON([]byte("not json {{{"), Dependency{Name: "x", Version: "1.0.0"}); err == nil {
		t.Fatal("AddPackageJSON with invalid JSON: want error, got nil")
	}
}

func TestAddPackageJSON_EmptyDependenciesObject(t *testing.T) {
	existing := "{\n  \"dependencies\": {}\n}\n"
	got, changed, err := AddPackageJSON([]byte(existing), Dependency{Name: "express", Version: "^4.18.0"})
	if err != nil {
		t.Fatalf("AddPackageJSON: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}
	want := "{\n  \"dependencies\": {\n    \"express\": \"^4.18.0\"\n  }\n}\n"
	if string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}
