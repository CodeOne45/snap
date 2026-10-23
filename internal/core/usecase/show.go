package usecase

import (
	"fmt"
	"path"

	"github.com/CodeOne45/snap/internal/core/template"
	"github.com/CodeOne45/snap/internal/ports"
)

// ShowResult is a template's manifest plus its raw (unrendered) file tree.
type ShowResult struct {
	Manifest template.Manifest `json:"manifest"`
	Files    []string          `json:"files,omitempty"`
	Dirs     []string          `json:"dirs,omitempty"`
}

// Show loads a template's manifest and lists its files, without resolving
// any variables or rendering anything. name is either a bare template name
// (its latest version) or "name@version" (M14's local pinning syntax).
func Show(store ports.Store, fsys ports.FS, name string) (ShowResult, error) {
	root, err := resolveStoreRoot(store, name)
	if err != nil {
		return ShowResult{}, fmt.Errorf("usecase: show: %w", err)
	}
	data, err := fsys.ReadFile(path.Join(root, template.ManifestFile))
	if err != nil {
		return ShowResult{}, fmt.Errorf("usecase: show: read manifest: %w", err)
	}
	m, err := template.ParseManifest(data)
	if err != nil {
		return ShowResult{}, fmt.Errorf("usecase: show: %w", err)
	}
	files, dirs, err := walkTemplate(fsys, root)
	if err != nil {
		return ShowResult{}, fmt.Errorf("usecase: show: %w", err)
	}
	return ShowResult{Manifest: m, Files: files, Dirs: dirs}, nil
}
