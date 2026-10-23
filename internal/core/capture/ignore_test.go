package capture

import "testing"

func TestCompilePatterns_SkipsBlankAndComments(t *testing.T) {
	patterns := compilePatterns([]string{"", "  ", "# a comment", "*.log"}, nil)
	if len(patterns) != 1 {
		t.Fatalf("len(patterns) = %d, want 1 (blank lines and comments skipped)", len(patterns))
	}
}

func TestNewSecretsMatcher(t *testing.T) {
	m := newSecretsMatcher()
	tests := []struct {
		path string
		want bool
	}{
		{".env", true},
		{".env.local", true},
		{".env.example", false},
		{"server.pem", true},
		{"id_rsa", true},
		{"id_rsa.pub", true},
		{"main.go", false},
	}
	for _, tt := range tests {
		if got := m.Match(pathSegments(tt.path), false); got != tt.want {
			t.Errorf("secrets.Match(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestPathSegments(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"a", 1},
		{"a/b/c", 3},
	}
	for _, tt := range tests {
		if got := len(pathSegments(tt.in)); got != tt.want {
			t.Errorf("len(pathSegments(%q)) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
