package cli

import (
	"testing"

	"github.com/CodeOne45/snap/internal/core/plan"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{43008, "42.0 KB"}, // 42 * 1024
		{1024 * 1024, "1.0 MB"},
		{5 * 1024 * 1024, "5.0 MB"},
	}
	for _, tt := range tests {
		if got := formatBytes(tt.n); got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.0k"},
		{11800, "11.8k"},
		{999999, "1000.0k"},
		{1000000, "1.0M"},
	}
	for _, tt := range tests {
		if got := formatTokens(tt.n); got != tt.want {
			t.Errorf("formatTokens(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestFormatReceipt(t *testing.T) {
	st := plan.Stats{Files: 31, Bytes: 43008, ApproxTokens: 11800}
	got := formatReceipt(st)
	want := "✓ 31 files · 42.0 KB · ~11.8k tokens written by Snap instead of the agent"
	if got != want {
		t.Errorf("formatReceipt() = %q, want %q", got, want)
	}
}
