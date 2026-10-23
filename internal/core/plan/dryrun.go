package plan

import (
	"fmt"
	"io"
)

// DryRun writes a human-readable preview of p to w, one line per Op in
// order.
func DryRun(w io.Writer, p Plan) error {
	if _, err := fmt.Fprintf(w, "Plan for %s:\n", p.TargetDir); err != nil {
		return err
	}
	for _, op := range p.Ops {
		var line string
		switch o := op.(type) {
		case CreateDirOp:
			line = fmt.Sprintf("  + dir   %s\n", o.Path)
		case WriteFileOp:
			line = fmt.Sprintf("  + file  %s (%d bytes)\n", o.Path, len(o.Content))
		case ModifyFileOp:
			line = fmt.Sprintf("  ~ file  %s (%d bytes)\n", o.Path, len(o.Content))
		case RunHookOp:
			line = fmt.Sprintf("  » hook  %s\n", o.Run)
		default:
			return fmt.Errorf("plan: dry-run: unsupported op type %T", op)
		}
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
	}
	return nil
}
