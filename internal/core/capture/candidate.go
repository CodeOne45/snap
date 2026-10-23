package capture

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/CodeOne45/snap/internal/core/render"
	"github.com/CodeOne45/snap/internal/ports"
)

// Candidate is a proposed template variable: a canonical word set detected
// from one or more seeds, scored by how strongly it looks like a real
// identifying name worth templatizing.
type Candidate struct {
	Words       []string // canonical word set, e.g. ["acme","site"]
	RawForms    []string // original strings detectors found, for display
	Sources     []string // detector names that contributed, for display
	FileCount   int
	PathHits    int
	ContentHits int
	Score       int
}

// Key returns the candidate's canonical kebab-case form, used as its
// grouping/map key and as the value stored for the bare {{name}} token.
func (c Candidate) Key() string {
	return render.Apply(c.Words, render.CaseKebab)
}

// manifestSource reports whether name is a structured-manifest detector
// (as opposed to the weaker folder-name/readme-h1 signals), for scoring.
func manifestSource(name string) bool {
	switch name {
	case "package.json", "go.mod", "Cargo.toml", "pyproject.toml", "git-remote":
		return true
	default:
		return false
	}
}

// CaseForm pairs a Case with the string render.Apply produces for it.
type CaseForm struct {
	Case  render.Case
	Value string
}

// CandidateForms returns every distinct case-form of words worth searching
// for and, later, replacing — bare kebab first (so it wins ties when forms
// collide, e.g. a single-word candidate's kebab and snake forms are
// identical; a consumer maps the first element to a bare {{name}} token and
// the rest to {{name|<case>}}).
func CandidateForms(words []string) []CaseForm {
	order := []render.Case{render.CaseKebab, render.CaseSnake, render.CasePascal, render.CaseCamel, render.CaseUpper, render.CaseTitle}
	seen := make(map[string]bool, len(order))
	var forms []CaseForm
	for _, c := range order {
		v := render.Apply(words, c)
		if !seen[v] {
			seen[v] = true
			forms = append(forms, CaseForm{Case: c, Value: v})
		}
	}
	return forms
}

// BuildCandidates runs every seed detector against root, groups raw hits by
// exact word set (case-insensitive), searches result for every case-form of
// each group in both paths and (non-binary) content, scores them, and
// returns the candidates sorted by score (highest first, then by Key for
// determinism).
func BuildCandidates(fsys ports.FS, root string, result Result) ([]Candidate, error) {
	groups := make(map[string]*Candidate)
	var order []string // first-seen order, for determinism before sorting

	for _, d := range SeedDetectors {
		value, ok, err := d.Detect(fsys, root)
		if err != nil {
			return nil, fmt.Errorf("capture: detect %s: %w", d.Name, err)
		}
		if !ok {
			continue
		}
		words := render.SplitWords(value)
		if len(words) == 0 {
			continue
		}
		key := strings.Join(words, "\x00")

		c, exists := groups[key]
		if !exists {
			c = &Candidate{Words: words}
			groups[key] = c
			order = append(order, key)
		}
		c.RawForms = append(c.RawForms, value)
		c.Sources = append(c.Sources, d.Name)
	}

	contents := make(map[string]string, len(result.Files))
	for _, f := range result.Files {
		if f.IsBinary {
			continue
		}
		data, err := fsys.ReadFile(path.Join(root, f.Path))
		if err != nil {
			return nil, fmt.Errorf("capture: build candidates: read %q: %w", f.Path, err)
		}
		contents[f.Path] = string(data)
	}

	candidates := make([]Candidate, 0, len(order))
	for _, key := range order {
		c := *groups[key]
		scoreCandidate(&c, result, contents)
		candidates = append(candidates, c)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].Key() < candidates[j].Key()
	})
	return candidates, nil
}

func scoreCandidate(c *Candidate, result Result, contents map[string]string) {
	forms := CandidateForms(c.Words)
	filesWithHit := make(map[string]bool)

	for _, f := range result.Files {
		hit := false
		for _, form := range forms {
			if n := strings.Count(f.Path, form.Value); n > 0 {
				c.PathHits += n
				hit = true
			}
		}
		if text, ok := contents[f.Path]; ok {
			for _, form := range forms {
				if n := strings.Count(text, form.Value); n > 0 {
					c.ContentHits += n
					hit = true
				}
			}
		}
		if hit {
			filesWithHit[f.Path] = true
		}
	}

	c.FileCount = len(filesWithHit)
	manifestBonus := 0
	for _, s := range c.Sources {
		if manifestSource(s) {
			manifestBonus = 50
			break
		}
	}
	c.Score = c.FileCount*10 + c.PathHits*5 + c.ContentHits + manifestBonus
}
