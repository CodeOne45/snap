package usecase

import (
	"fmt"
	"path"
	"sort"

	"github.com/CodeOne45/snap/internal/core/template"
	"github.com/CodeOne45/snap/internal/ports"
)

// walkTemplate recursively lists every file and directory under root,
// relative to root, excluding the manifest itself. Both slices are sorted
// for deterministic output. Empty directories are returned in dirs; a
// directory that contains files does not need its own CreateDir op since
// the executor creates parent directories implicitly.
func walkTemplate(fsys ports.FS, root string) (files, dirs []string, err error) {
	if err := walkDir(fsys, root, "", &files, &dirs); err != nil {
		return nil, nil, err
	}
	sort.Strings(files)
	sort.Strings(dirs)
	return files, dirs, nil
}

// walkAllFiles recursively lists every file under root, relative to root,
// sorted. Unlike walkTemplate, nothing is excluded — root is an arbitrary
// generated project directory (M16's Update reads "ours" through this),
// not a template.
func walkAllFiles(fsys ports.FS, root string) ([]string, error) {
	var files []string
	if err := walkAllDir(fsys, root, "", &files); err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func walkAllDir(fsys ports.FS, root, rel string, files *[]string) error {
	entries, err := fsys.ReadDir(path.Join(root, rel))
	if err != nil {
		return fmt.Errorf("usecase: walk %q: %w", rel, err)
	}
	for _, e := range entries {
		name := path.Base(e.Path)
		childRel := name
		if rel != "" {
			childRel = path.Join(rel, name)
		}
		if e.IsDir {
			if err := walkAllDir(fsys, root, childRel, files); err != nil {
				return err
			}
			continue
		}
		*files = append(*files, childRel)
	}
	return nil
}

func walkDir(fsys ports.FS, root, rel string, files, dirs *[]string) error {
	entries, err := fsys.ReadDir(path.Join(root, rel))
	if err != nil {
		return fmt.Errorf("usecase: walk %q: %w", rel, err)
	}

	if len(entries) == 0 {
		if rel != "" {
			*dirs = append(*dirs, rel)
		}
		return nil
	}

	for _, e := range entries {
		name := path.Base(e.Path)
		childRel := name
		if rel != "" {
			childRel = path.Join(rel, name)
		}
		if e.IsDir {
			if err := walkDir(fsys, root, childRel, files, dirs); err != nil {
				return err
			}
			continue
		}
		if rel == "" && name == template.ManifestFile {
			continue
		}
		*files = append(*files, childRel)
	}
	return nil
}
