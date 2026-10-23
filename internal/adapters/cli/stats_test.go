package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/CodeOne45/snap/internal/adapters/usagelog"
	"github.com/CodeOne45/snap/internal/core/plan"
)

func withUsageLog(t *testing.T, entries ...usagelog.Entry) {
	t.Helper()
	t.Setenv(usagelog.DisableEnvVar, "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	for _, e := range entries {
		if err := usagelog.Append(e); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
}

func TestStatsCmd_Totals(t *testing.T) {
	now := time.Now()
	withUsageLog(t,
		usagelog.Entry{Template: "api-starter", Stats: plan.Stats{Files: 10, Bytes: 1000, ApproxTokens: 500}, Time: now, Source: "cli"},
		usagelog.Entry{Template: "api-starter", Stats: plan.Stats{Files: 5, Bytes: 500, ApproxTokens: 250}, Time: now, Source: "mcp"},
	)

	cmd := newStatsCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	got := out.String()
	for _, want := range []string{"api-starter", "15 files", "cli", "mcp"} {
		if !strings.Contains(got, want) {
			t.Errorf("output = %q, want it to contain %q", got, want)
		}
	}
}

func TestStatsCmd_NoEntries(t *testing.T) {
	withUsageLog(t)

	cmd := newStatsCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "no usage recorded") {
		t.Errorf("output = %q, want a message about no recorded usage", out.String())
	}
}

func TestStatsCmd_SinceFiltersOldEntries(t *testing.T) {
	withUsageLog(t,
		usagelog.Entry{Template: "old", Stats: plan.Stats{Files: 1}, Time: time.Now().Add(-60 * 24 * time.Hour), Source: "cli"},
		usagelog.Entry{Template: "recent", Stats: plan.Stats{Files: 2}, Time: time.Now().Add(-1 * time.Hour), Source: "cli"},
	)

	cmd := newStatsCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--since", "30d"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	got := out.String()
	if strings.Contains(got, "old") {
		t.Errorf("output = %q, want the 60-day-old entry excluded", got)
	}
	if !strings.Contains(got, "recent") {
		t.Errorf("output = %q, want the 1-hour-old entry included", got)
	}
}

func TestStatsCmd_JSON(t *testing.T) {
	withUsageLog(t,
		usagelog.Entry{Template: "api-starter", Stats: plan.Stats{Files: 3, Bytes: 100, ApproxTokens: 20}, Time: time.Now(), Source: "cli"},
	)

	cmd := newStatsCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	for _, want := range []string{`"total_files": 3`, `"by_template"`, `"by_source"`, `"api-starter"`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output = %s, want it to contain %q", out.String(), want)
		}
	}
}

func TestStatsCmd_InvalidSince(t *testing.T) {
	withUsageLog(t)
	cmd := newStatsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"--since", "not-a-duration"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute with an invalid --since: want error, got nil")
	}
}
