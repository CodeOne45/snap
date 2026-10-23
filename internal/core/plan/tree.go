package plan

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// Tree renders p as an indented file tree, for interactive previews (the
// TUI). Unlike DryRun's op-order listing, entries are sorted by path for a
// stable, readable tree regardless of the order Ops were built in.
func Tree(p Plan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s/\n", p.TargetDir)

	type entry struct {
		path  string
		isDir bool
	}
	dirs := make(map[string]bool)
	var entries []entry
	for _, op := range p.Ops {
		switch o := op.(type) {
		case CreateDirOp:
			dirs[o.Path] = true
		case WriteFileOp:
			entries = append(entries, entry{o.Path, false})
			// A file's parent directories are implied even when nothing
			// in Ops creates them explicitly (the executor creates them
			// as needed) — show them in the tree regardless.
			for dir := path.Dir(o.Path); dir != "."; dir = path.Dir(dir) {
				dirs[dir] = true
			}
		}
	}
	for d := range dirs {
		entries = append(entries, entry{d, true})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })

	for _, e := range entries {
		depth := strings.Count(e.path, "/")
		name := path.Base(e.path)
		if e.isDir {
			name += "/"
		}
		fmt.Fprintf(&b, "%s%s\n", strings.Repeat("  ", depth+1), name)
	}

	if hooks := p.Hooks(); len(hooks) > 0 {
		b.WriteString("\nHooks:\n")
		for _, h := range hooks {
			fmt.Fprintf(&b, "  » %s\n", h.Run)
		}
	}
	return b.String()
}
