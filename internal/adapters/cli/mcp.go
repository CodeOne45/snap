package cli

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/CodeOne45/snap/internal/adapters/hookrunner"
	snapmcp "github.com/CodeOne45/snap/internal/adapters/mcp"
)

func newMCPCmd(deps func() (Deps, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run snap as a Model Context Protocol server over stdio",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, err := deps()
			if err != nil {
				return internalErrorf("snap mcp: %w", err)
			}
			server := snapmcp.NewServer(d.FS, d.Store, hookrunner.New(), version)
			if err := server.Run(cmd.Context(), &sdkmcp.StdioTransport{}); err != nil {
				return internalErrorf("snap mcp: %w", err)
			}
			return nil
		},
	}
}
