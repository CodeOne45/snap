package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CodeOne45/snap/internal/core/usecase"
)

func newListCmd(deps func() (Deps, error)) *cobra.Command {
	var jsonOut, recipesOut bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available templates",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, err := deps()
			if err != nil {
				return internalErrorf("snap list: %w", err)
			}

			if recipesOut {
				recipes, err := usecase.ListRecipes(d.RecipeStore, d.FS)
				if err != nil {
					return err
				}
				return printRecipeList(cmd.OutOrStdout(), recipes, jsonOut)
			}

			manifests, err := usecase.List(d.Store, d.FS)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if jsonOut {
				data, err := json.MarshalIndent(manifests, "", "  ")
				if err != nil {
					return internalErrorf("snap list: %w", err)
				}
				_, err = fmt.Fprintln(out, string(data))
				if err != nil {
					return internalErrorf("snap list: %w", err)
				}
				return nil
			}
			for _, m := range manifests {
				if _, err := fmt.Fprintf(out, "%s\t%s\t%s\n", m.Name, m.Version, m.Description); err != nil {
					return internalErrorf("snap list: %w", err)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "print the template list as JSON")
	cmd.Flags().BoolVar(&recipesOut, "recipes", false, "list recipes instead of templates (alias: snap recipes)")
	return cmd
}
