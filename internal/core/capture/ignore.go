package capture

import (
	"path"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"github.com/CodeOne45/snap/internal/ports"
)

// GitignoreFile and SnapignoreFile are the two ignore-file names consulted
// at every directory level while scanning, in that priority order (later
// cascades on top of earlier, per the ignore chain in Scan).
const (
	GitignoreFile  = ".gitignore"
	SnapignoreFile = ".snapignore"
)

// builtinJunk is always excluded, with no opt-out: common build output and
// VCS/package-manager directories that should never end up in a template.
// Bare names (no "/") match at any depth, per gitignore semantics.
var builtinJunk = []string{
	".git/",
	".svn/",
	".hg/",
	"node_modules/",
	".next/",
	".nuxt/",
	"dist/",
	"build/",
	"out/",
	"target/",
	"vendor/",
	"__pycache__/",
	"*.pyc",
	".venv/",
	"venv/",
	".DS_Store",
	"Thumbs.db",
	".snap/",
}

// secretPatterns is a non-overridable safety gate (checked independently of
// the main ignore chain): nothing matching these is ever captured, even if
// a project's own .gitignore or .snapignore tries to negate it. The
// .env.example carve-out is the one deliberate exception.
var secretPatterns = []string{
	".env*",
	"*.pem",
	"*.key",
	"id_rsa*",
	"!.env.example",
}

func compilePatterns(lines []string, domain []string) []gitignore.Pattern {
	patterns := make([]gitignore.Pattern, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		patterns = append(patterns, gitignore.ParsePattern(line, domain))
	}
	return patterns
}

// readIgnoreFile reads name from dir (an absolute-from-fsys-root path),
// returning no patterns (and no error) if the file doesn't exist.
func readIgnoreFile(fsys ports.FS, dir, name string, domain []string) []gitignore.Pattern {
	data, err := fsys.ReadFile(path.Join(dir, name))
	if err != nil {
		return nil
	}
	return compilePatterns(strings.Split(string(data), "\n"), domain)
}

func newSecretsMatcher() gitignore.Matcher {
	return gitignore.NewMatcher(compilePatterns(secretPatterns, nil))
}

func newBuiltinPatterns() []gitignore.Pattern {
	return compilePatterns(builtinJunk, nil)
}

func newExtraPatterns(lines []string) []gitignore.Pattern {
	return compilePatterns(lines, nil)
}

// pathSegments splits a forward-slash relative path into the segment form
// gitignore.Pattern.Match expects.
func pathSegments(p string) []string {
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}
