package usecase

import "testing"

func TestSplitVersionPin(t *testing.T) {
	tests := []struct {
		in          string
		name, pin   string
		wantMatched bool
	}{
		{"foo@1.0.0", "foo", "1.0.0", true},
		{"foo", "", "", false},
		{"@1.0.0", "", "", false},
		{"foo@", "", "", false},
		{"a@b@1.0.0", "a@b", "1.0.0", true}, // last "@" wins
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			name, pin, ok := splitVersionPin(tt.in)
			if ok != tt.wantMatched {
				t.Fatalf("splitVersionPin(%q) ok = %v, want %v", tt.in, ok, tt.wantMatched)
			}
			if !ok {
				return
			}
			if name != tt.name || pin != tt.pin {
				t.Errorf("splitVersionPin(%q) = (%q, %q), want (%q, %q)", tt.in, name, pin, tt.name, tt.pin)
			}
		})
	}
}
