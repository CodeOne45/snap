// Package capture implements the read side of "snap capture": walking a
// source project, applying the ignore chain (.gitignore -> built-in junk
// list -> .snapignore -> extra excludes, with a non-overridable secrets
// gate), and classifying what's left as text or binary. Detectors that turn
// this into placeholder proposals arrive in M5.
package capture

import (
	"fmt"
	"io/fs"
	"path"
	"sort"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"github.com/CodeOne45/snap/internal/core/render"
	"github.com/CodeOne45/snap/internal/ports"
)

// File is one file that survived the ignore chain during a scan.
type File struct {
	Path     string // relative to the scanned root, forward-slash
	Size     int64
	Mode     fs.FileMode
	IsBinary bool
}

// Result is everything a scan found worth capturing.
type Result struct {
	Files []File
	Dirs  []string // empty directories that survive the chain
	// Ignored is the set of top-level (direct children of root only) names
	// excluded by the ignore chain, for display purposes only (e.g.
	// "Ignored: node_modules, .next, .git, .env").
	Ignored []string
}

// Options configures a Scan beyond its built-in defaults.
type Options struct {
	// ExtraExcludes are additional gitignore-style patterns (e.g. from a
	// future --exclude CLI flag), applied after .gitignore, the built-in
	// junk list, and .snapignore.
	ExtraExcludes []string
}

// Scan walks root through fsys and returns every file and empty directory
// that survives the ignore chain, sorted for deterministic output. An
// ignored directory is pruned entirely; its contents are never visited.
func Scan(fsys ports.FS, root string, opts Options) (Result, error) {
	s := &scanner{
		fsys:    fsys,
		root:    root,
		builtin: newBuiltinPatterns(),
		extra:   newExtraPatterns(opts.ExtraExcludes),
		secrets: newSecretsMatcher(),
	}

	var result Result
	if err := s.walk("", nil, nil, &result); err != nil {
		return Result{}, err
	}

	sort.Slice(result.Files, func(i, j int) bool { return result.Files[i].Path < result.Files[j].Path })
	sort.Strings(result.Dirs)
	sort.Strings(result.Ignored)
	return result, nil
}

type scanner struct {
	fsys    ports.FS
	root    string
	builtin []gitignore.Pattern
	extra   []gitignore.Pattern
	secrets gitignore.Matcher
}

// walk processes directory rel (relative to s.root), given the gitignore
// and snapignore patterns accumulated from rel's ancestors. It never
// mutates the caller's slices, so sibling subtrees don't see each other's
// nested ignore files.
func (s *scanner) walk(rel string, gitPatterns, snapPatterns []gitignore.Pattern, result *Result) error {
	dir := path.Join(s.root, rel)
	domain := pathSegments(rel)

	gitPatterns = append(append([]gitignore.Pattern{}, gitPatterns...), readIgnoreFile(s.fsys, dir, GitignoreFile, domain)...)
	snapPatterns = append(append([]gitignore.Pattern{}, snapPatterns...), readIgnoreFile(s.fsys, dir, SnapignoreFile, domain)...)

	matcher := gitignore.NewMatcher(combinePatterns(gitPatterns, s.builtin, snapPatterns, s.extra))

	entries, err := s.fsys.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("capture: scan %q: %w", rel, err)
	}

	survived := false
	for _, e := range entries {
		name := path.Base(e.Path)
		childRel := name
		if rel != "" {
			childRel = rel + "/" + name
		}
		segs := pathSegments(childRel)

		if s.secrets.Match(segs, e.IsDir) || matcher.Match(segs, e.IsDir) {
			if rel == "" {
				result.Ignored = append(result.Ignored, name)
			}
			continue
		}
		survived = true

		if e.IsDir {
			if err := s.walk(childRel, gitPatterns, snapPatterns, result); err != nil {
				return err
			}
			continue
		}

		content, err := s.fsys.ReadFile(path.Join(s.root, childRel))
		if err != nil {
			return fmt.Errorf("capture: scan %q: %w", childRel, err)
		}
		result.Files = append(result.Files, File{
			Path:     childRel,
			Size:     e.Size,
			Mode:     e.Mode,
			IsBinary: render.IsBinary(content),
		})
	}

	if !survived && rel != "" {
		result.Dirs = append(result.Dirs, rel)
	}
	return nil
}

func combinePatterns(groups ...[]gitignore.Pattern) []gitignore.Pattern {
	var out []gitignore.Pattern
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}
