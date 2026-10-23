package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CodeOne45/snap/internal/core/usecase"
)

// statusJSON is snap status --json's wire shape: one entry per directory
// checked, in the order given.
type statusJSON struct {
	Dir            string `json:"dir"`
	Template       string `json:"template,omitempty"`
	CurrentVersion string `json:"current_version,omitempty"`
	LatestVersion  string `json:"latest_version,omitempty"`
	UpToDate       bool   `json:"up_to_date"`
	Source         string `json:"source,omitempty"`
	Error          string `json:"error,omitempty"`
}

func newStatusCmd(deps func() (Deps, error)) *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "status <dir>...",
		Short: "Report which projects came from which template version, and which are behind",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := deps()
			if err != nil {
				return internalErrorf("snap status: %w", err)
			}

			results := usecase.Status(usecase.StatusParams{Store: d.Store, FS: d.FS, Dirs: args})

			out := cmd.OutOrStdout()
			anyIssue := false
			if jsonOut {
				wire := make([]statusJSON, len(results))
				for i, r := range results {
					w := statusJSON{
						Dir: r.Dir, Template: r.Template, CurrentVersion: r.CurrentVersion,
						LatestVersion: r.LatestVersion, UpToDate: r.UpToDate, Source: r.Source,
					}
					if r.Err != nil {
						w.Error = r.Err.Error()
						anyIssue = true
					} else if r.Source == "local" && !r.UpToDate {
						anyIssue = true
					}
					wire[i] = w
				}
				data, err := json.MarshalIndent(wire, "", "  ")
				if err != nil {
					return internalErrorf("snap status: %w", err)
				}
				if _, err := fmt.Fprintln(out, string(data)); err != nil {
					return internalErrorf("snap status: %w", err)
				}
			} else {
				for _, r := range results {
					line, issue := formatStatusLine(r)
					anyIssue = anyIssue || issue
					if _, err := fmt.Fprintln(out, line); err != nil {
						return internalErrorf("snap status: %w", err)
					}
				}
			}

			if anyIssue {
				return fmt.Errorf("snap status: one or more projects need attention")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "print one JSON object per directory instead of a human-readable summary")
	return cmd
}

func formatStatusLine(r usecase.ProjectStatus) (line string, issue bool) {
	if r.Err != nil {
		return fmt.Sprintf("%s\t(%v)", r.Dir, r.Err), true
	}
	switch {
	case r.Source != "local":
		return fmt.Sprintf("%s\t%s %s\t(remote source, drift unknown)", r.Dir, r.Template, r.CurrentVersion), false
	case r.UpToDate:
		return fmt.Sprintf("%s\t%s %s\t(up to date)", r.Dir, r.Template, r.CurrentVersion), false
	default:
		return fmt.Sprintf("%s\t%s %s -> %s\t(behind)", r.Dir, r.Template, r.CurrentVersion, r.LatestVersion), true
	}
}
