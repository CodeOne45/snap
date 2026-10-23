package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/CodeOne45/snap/internal/core/recipe"
	"github.com/CodeOne45/snap/internal/core/usecase"
)

func newRecipesCmd(deps func() (Deps, error)) *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "recipes",
		Short: "List available recipes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, err := deps()
			if err != nil {
				return internalErrorf("snap recipes: %w", err)
			}
			recipes, err := usecase.ListRecipes(d.RecipeStore, d.FS)
			if err != nil {
				return err
			}
			return printRecipeList(cmd.OutOrStdout(), recipes, jsonOut)
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "print the recipe list as JSON")
	return cmd
}

// printRecipeList is shared by `snap recipes` and `snap list --recipes`.
func printRecipeList(out io.Writer, recipes []recipe.Recipe, jsonOut bool) error {
	if jsonOut {
		data, err := json.MarshalIndent(recipes, "", "  ")
		if err != nil {
			return internalErrorf("snap recipes: %w", err)
		}
		_, err = fmt.Fprintln(out, string(data))
		return err
	}
	for _, r := range recipes {
		if _, err := fmt.Fprintf(out, "%s\t%s\t%s\n", r.Name, r.Version, r.Description); err != nil {
			return internalErrorf("snap recipes: %w", err)
		}
	}
	return nil
}
