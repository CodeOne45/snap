package render

import "testing"

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want bool
	}{
		{"plain text", []byte("hello world"), false},
		{"unicode text", []byte("héllo wörld, 日本語"), false},
		{"empty", []byte{}, false},
		{"null byte", []byte{'a', 0x00, 'b'}, true},
		{"png header", []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0d}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBinary(tt.in); got != tt.want {
				t.Errorf("IsBinary(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
