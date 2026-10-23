package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CodeOne45/snap/internal/core/usecase"
)

func newShowCmd(deps func() (Deps, error)) *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "show <template>",
		Short: "Show a template's variables and file tree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := deps()
			if err != nil {
				return internalErrorf("snap show: %w", err)
			}
			result, err := usecase.Show(d.Store, d.FS, args[0])
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if jsonOut {
				data, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return internalErrorf("snap show: %w", err)
				}
				if _, err := fmt.Fprintln(out, string(data)); err != nil {
					return internalErrorf("snap show: %w", err)
				}
				return nil
			}
			if _, err := fmt.Fprint(out, formatShow(result)); err != nil {
				return internalErrorf("snap show: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "print the template's manifest and file tree as JSON")
	return cmd
}

func formatShow(result usecase.ShowResult) string {
	var b strings.Builder
	m := result.Manifest

	fmt.Fprintf(&b, "%s %s\n", m.Name, m.Version)
	if m.Description != "" {
		fmt.Fprintln(&b, m.Description)
	}

	if len(m.Variables) > 0 {
		fmt.Fprintln(&b, "\nVariables:")
		for _, v := range m.Variables {
			def := ""
			if v.HasDefault {
				def = fmt.Sprintf(" (default %q)", v.Default)
			}
			fmt.Fprintf(&b, "  %s\t%s%s\n", v.Name, v.Type, def)
		}
	}

	if len(m.Optional) > 0 {
		fmt.Fprintln(&b, "\nOptional:")
		for _, o := range m.Optional {
			fmt.Fprintf(&b, "  %s\t(default %t)\t%s\n", o.Name, o.Default, strings.Join(o.Paths, ", "))
		}
	}

	fmt.Fprintln(&b, "\nFiles:")
	for _, dir := range result.Dirs {
		fmt.Fprintf(&b, "  %s/\n", dir)
	}
	for _, f := range result.Files {
		fmt.Fprintf(&b, "  %s\n", f)
	}
	return b.String()
}
