package prompt

import (
	"bytes"
	"strings"
	"testing"
)

func TestAsk(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultVal string
		want       string
	}{
		{"explicit value", "northwind-site\n", "acme-site", "northwind-site"},
		{"empty uses default", "\n", "acme-site", "acme-site"},
		{"trims whitespace", "  spaced  \n", "", "spaced"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			p := New(strings.NewReader(tt.input), &out)
			got, err := p.Ask("Project name", tt.defaultVal)
			if err != nil {
				t.Fatalf("Ask: %v", err)
			}
			if got != tt.want {
				t.Errorf("Ask() = %q, want %q", got, tt.want)
			}
			if !strings.Contains(out.String(), "Project name") {
				t.Errorf("prompt output = %q, want it to contain the message", out.String())
			}
		})
	}
}

func TestConfirm(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultYes bool
		want       bool
		wantErr    bool
	}{
		{"yes", "y\n", false, true, false},
		{"no", "n\n", true, false, false},
		{"empty uses default true", "\n", true, true, false},
		{"empty uses default false", "\n", false, false, false},
		{"garbage errors", "maybe\n", true, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			p := New(strings.NewReader(tt.input), &out)
			got, err := p.Confirm("Include Docker?", tt.defaultYes)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Confirm: want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Confirm: %v", err)
			}
			if got != tt.want {
				t.Errorf("Confirm() = %v, want %v", got, tt.want)
			}
		})
	}
}
