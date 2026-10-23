package cli

import (
	"fmt"

	"github.com/CodeOne45/snap/internal/core/plan"
)

// formatReceipt renders st as the one-line confirmation shown after a
// successful generation: "Snap wrote this instead of the agent."
func formatReceipt(st plan.Stats) string {
	return fmt.Sprintf("✓ %d files · %s · ~%s tokens written by Snap instead of the agent",
		st.Files, formatBytes(st.Bytes), formatTokens(st.ApproxTokens))
}

func formatBytes(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

func formatTokens(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d", n)
	case n < 1_000_000:
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	default:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
}
