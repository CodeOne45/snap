package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CodeOne45/snap/internal/adapters/usagelog"
	"github.com/CodeOne45/snap/internal/core/plan"
)

// statsJSON is snap stats --json's wire shape.
type statsJSON struct {
	TotalFiles        int                   `json:"total_files"`
	TotalBytes        int64                 `json:"total_bytes"`
	TotalApproxTokens int                   `json:"total_approx_tokens"`
	ByTemplate        map[string]plan.Stats `json:"by_template,omitempty"`
	BySource          map[string]plan.Stats `json:"by_source,omitempty"`
}

func newStatsCmd() *cobra.Command {
	var since string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show totals of what snap has generated, from the local usage log",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			entries, err := usagelog.ReadAll()
			if err != nil {
				return internalErrorf("snap stats: %w", err)
			}

			if since != "" {
				d, err := parseSinceDuration(since)
				if err != nil {
					return fmt.Errorf("snap stats: %w", err)
				}
				cutoff := time.Now().Add(-d)
				filtered := entries[:0]
				for _, e := range entries {
					if !e.Time.Before(cutoff) {
						filtered = append(filtered, e)
					}
				}
				entries = filtered
			}

			agg := aggregateUsage(entries)
			out := cmd.OutOrStdout()
			if jsonOut {
				data, err := json.MarshalIndent(agg, "", "  ")
				if err != nil {
					return internalErrorf("snap stats: %w", err)
				}
				_, err = fmt.Fprintln(out, string(data))
				return err
			}
			_, err = fmt.Fprint(out, formatStatsSummary(agg))
			return err
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "only include entries from the last duration (e.g. 30d, 24h)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print totals as JSON")
	return cmd
}

// parseSinceDuration extends time.ParseDuration with a "d" (days) suffix,
// since usage history is naturally measured in days, not hours.
func parseSinceDuration(s string) (time.Duration, error) {
	if rest, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(rest)
		if err != nil {
			return 0, fmt.Errorf("invalid --since %q: %w", s, err)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid --since %q: %w", s, err)
	}
	return d, nil
}

func aggregateUsage(entries []usagelog.Entry) statsJSON {
	agg := statsJSON{ByTemplate: map[string]plan.Stats{}, BySource: map[string]plan.Stats{}}
	for _, e := range entries {
		agg.TotalFiles += e.Stats.Files
		agg.TotalBytes += e.Stats.Bytes
		agg.TotalApproxTokens += e.Stats.ApproxTokens

		key := e.Template
		if key == "" {
			key = e.Recipe
		}
		addStats(agg.ByTemplate, key, e.Stats)
		addStats(agg.BySource, e.Source, e.Stats)
	}
	return agg
}

func addStats(m map[string]plan.Stats, key string, s plan.Stats) {
	cur := m[key]
	cur.Files += s.Files
	cur.TextFiles += s.TextFiles
	cur.Bytes += s.Bytes
	cur.ApproxTokens += s.ApproxTokens
	m[key] = cur
}

func formatStatsSummary(agg statsJSON) string {
	if agg.TotalFiles == 0 && len(agg.ByTemplate) == 0 {
		return "no usage recorded yet\n"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Total: %d files · %s · ~%s tokens\n",
		agg.TotalFiles, formatBytes(agg.TotalBytes), formatTokens(agg.TotalApproxTokens))

	fmt.Fprintln(&b, "\nBy template:")
	for _, name := range sortedKeys(agg.ByTemplate) {
		s := agg.ByTemplate[name]
		fmt.Fprintf(&b, "  %s\t%d files · %s · ~%s tokens\n", name, s.Files, formatBytes(s.Bytes), formatTokens(s.ApproxTokens))
	}

	fmt.Fprintln(&b, "\nBy source:")
	for _, name := range sortedKeys(agg.BySource) {
		s := agg.BySource[name]
		fmt.Fprintf(&b, "  %s\t%d files · %s · ~%s tokens\n", name, s.Files, formatBytes(s.Bytes), formatTokens(s.ApproxTokens))
	}
	return b.String()
}

func sortedKeys(m map[string]plan.Stats) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
