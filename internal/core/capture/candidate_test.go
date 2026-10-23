package capture

import (
	"reflect"
	"testing"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
)

func TestBuildCandidates_SameWordsMerge(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "acme-site/go.mod", "module github.com/someuser/AcmeSite\n")
	write(t, fsys, "acme-site/main.go", "package main")

	result, err := Scan(fsys, "acme-site", Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	candidates, err := BuildCandidates(fsys, "acme-site", result)
	if err != nil {
		t.Fatalf("BuildCandidates: %v", err)
	}

	// folder-name "acme-site" and go.mod "AcmeSite" share the word set
	// [acme, site] and must merge into one candidate.
	var found *Candidate
	for i := range candidates {
		if candidates[i].Key() == "acme-site" {
			found = &candidates[i]
		}
	}
	if found == nil {
		t.Fatalf("no candidate with key acme-site in %+v", candidates)
	}
	if len(found.Sources) != 2 {
		t.Errorf("Sources = %v, want 2 contributing detectors (folder-name, go.mod)", found.Sources)
	}
}

func TestBuildCandidates_DifferentWordsDontMerge(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "acme-site/package.json", `{"name": "widgets"}`)

	result, err := Scan(fsys, "acme-site", Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	candidates, err := BuildCandidates(fsys, "acme-site", result)
	if err != nil {
		t.Fatalf("BuildCandidates: %v", err)
	}

	keys := make(map[string]bool)
	for _, c := range candidates {
		keys[c.Key()] = true
	}
	if !keys["acme-site"] || !keys["widgets"] {
		t.Errorf("candidates = %v, want both acme-site and widgets as separate entries", keys)
	}
}

func TestBuildCandidates_ScoringAndOrder(t *testing.T) {
	fsys := memfs.New()
	// "acme-site" is a seed from go.mod, the folder name, and README's H1,
	// and shows up repeatedly in paths/content. "widgets" is only a seed
	// from package.json and appears nowhere else. acme-site must score
	// higher and sort first despite both getting the manifest-source bonus.
	write(t, fsys, "acme-site/go.mod", "module github.com/someuser/acme-site\n")
	write(t, fsys, "acme-site/package.json", `{"name": "widgets"}`)
	write(t, fsys, "acme-site/README.md", "# AcmeSite\n\nThe acme-site project.")
	write(t, fsys, "acme-site/cmd/acme-site/main.go", "package main // acme-site")

	result, err := Scan(fsys, "acme-site", Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	candidates, err := BuildCandidates(fsys, "acme-site", result)
	if err != nil {
		t.Fatalf("BuildCandidates: %v", err)
	}
	if len(candidates) < 2 {
		t.Fatalf("len(candidates) = %d, want >= 2: %+v", len(candidates), candidates)
	}
	if candidates[0].Key() != "acme-site" {
		t.Errorf("candidates[0].Key() = %q, want %q (highest score first)", candidates[0].Key(), "acme-site")
	}
	for i := 1; i < len(candidates); i++ {
		if candidates[i-1].Score < candidates[i].Score {
			t.Errorf("candidates not sorted by score desc: %+v", candidates)
		}
	}
}

func TestCandidateForms_DedupesCollisions(t *testing.T) {
	// A single word's kebab, snake, and camel forms all coincide; pascal and
	// title also coincide. Only distinct strings survive.
	forms := CandidateForms([]string{"acme"})
	seen := make(map[string]bool)
	for _, f := range forms {
		if seen[f.Value] {
			t.Errorf("CandidateForms returned duplicate form %q: %+v", f.Value, forms)
		}
		seen[f.Value] = true
	}
	if forms[0].Value != "acme" {
		t.Errorf("CandidateForms[0].Value = %q, want bare kebab form %q first", forms[0].Value, "acme")
	}
}

func TestBuildCandidates_NoSeedsFound(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "/main.txt", "nothing identifying here")

	result, err := Scan(fsys, "", Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	candidates, err := BuildCandidates(fsys, "", result)
	if err != nil {
		t.Fatalf("BuildCandidates: %v", err)
	}
	// folder-name always fires for a non-empty root; for root "" there's no
	// base name, so with nothing else present we expect zero candidates.
	if len(candidates) != 0 {
		t.Errorf("candidates = %+v, want none", candidates)
	}
}

func TestBuildCandidates_WordsPreserved(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "acme-site/main.go", "package main")

	result, err := Scan(fsys, "acme-site", Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	candidates, err := BuildCandidates(fsys, "acme-site", result)
	if err != nil {
		t.Fatalf("BuildCandidates: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1: %+v", len(candidates), candidates)
	}
	want := []string{"acme", "site"}
	if !reflect.DeepEqual(candidates[0].Words, want) {
		t.Errorf("Words = %v, want %v", candidates[0].Words, want)
	}
}
