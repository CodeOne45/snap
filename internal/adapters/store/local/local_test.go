package local

import (
	"testing"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
)

func TestList(t *testing.T) {
	fsys := memfs.New()
	for _, name := range []string{"web-app", "api-starter"} {
		if err := fsys.MkdirAll("templates/"+name+"/1.0.0", 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	if err := fsys.WriteFile("templates/not-a-template.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	s := New(fsys, "templates")
	got, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"api-starter", "web-app"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("List() = %v, want %v", got, want)
	}
}

func TestPath(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("templates/api-starter/1.0.0", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := fsys.WriteFile("templates/plain-file.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	s := New(fsys, "templates")

	got, err := s.Path("api-starter")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if got != "templates/api-starter/1.0.0" {
		t.Errorf("Path = %q, want %q", got, "templates/api-starter/1.0.0")
	}

	if _, err := s.Path("missing"); err == nil {
		t.Error("Path(missing): want error, got nil")
	}
	if _, err := s.Path("plain-file.txt"); err == nil {
		t.Error("Path(non-directory): want error, got nil")
	}
}

func TestPath_ResolvesLatestVersion(t *testing.T) {
	fsys := memfs.New()
	for _, v := range []string{"0.1.0", "1.0.0", "1.9.0", "1.10.0"} {
		if err := fsys.MkdirAll("templates/api-starter/"+v, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	s := New(fsys, "templates")

	got, err := s.Path("api-starter")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	// Numeric compare, not lexical: 1.10.0 is the latest, not 1.9.0.
	if want := "templates/api-starter/1.10.0"; got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

func TestVersions(t *testing.T) {
	fsys := memfs.New()
	for _, v := range []string{"1.0.0", "0.1.0", "1.2.0"} {
		if err := fsys.MkdirAll("templates/api-starter/"+v, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	s := New(fsys, "templates")

	got, err := s.Versions("api-starter")
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	want := []string{"0.1.0", "1.0.0", "1.2.0"}
	if len(got) != len(want) {
		t.Fatalf("Versions() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Versions()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestVersions_Errors(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("templates/api-starter/not-a-version", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	s := New(fsys, "templates")

	if _, err := s.Versions("missing"); err == nil {
		t.Error("Versions(missing template): want error, got nil")
	}
	if _, err := s.Versions("api-starter"); err == nil {
		t.Error("Versions(a template with a non-semver version directory): want error, got nil")
	}
}

func TestPathVersion(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("templates/api-starter/1.0.0", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	s := New(fsys, "templates")

	got, err := s.PathVersion("api-starter", "1.0.0")
	if err != nil {
		t.Fatalf("PathVersion: %v", err)
	}
	if want := "templates/api-starter/1.0.0"; got != want {
		t.Errorf("PathVersion = %q, want %q", got, want)
	}

	if _, err := s.PathVersion("api-starter", "2.0.0"); err == nil {
		t.Error("PathVersion(a version that doesn't exist): want error, got nil")
	}
}
