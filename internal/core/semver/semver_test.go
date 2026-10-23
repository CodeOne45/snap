package semver

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		in      string
		want    Version
		wantErr bool
	}{
		{"1.2.3", Version{1, 2, 3}, false},
		{"0.1.0", Version{0, 1, 0}, false},
		{"0.0.0", Version{0, 0, 0}, false},
		{"10.20.30", Version{10, 20, 30}, false},
		{"", Version{}, true},
		{"1.2", Version{}, true},
		{"1.2.3.4", Version{}, true},
		{"v1.2.3", Version{}, true},
		{"1.2.3-rc1", Version{}, true},
		{"1.02.3", Version{}, true},
		{"1.-2.3", Version{}, true},
		{"1.2.3a", Version{}, true},
		{"a.b.c", Version{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := Parse(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("Parse(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestVersion_String(t *testing.T) {
	if got := (Version{1, 2, 3}).String(); got != "1.2.3" {
		t.Errorf("String() = %q, want %q", got, "1.2.3")
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"1.2.0", "1.10.0", -1}, // numeric, not lexical
		{"1.2.3", "1.2.4", -1},
		{"1.9.0", "1.10.0", -1},
		{"2.0.0", "1.99.99", 1},
	}
	for _, tt := range tests {
		a, err := Parse(tt.a)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.a, err)
		}
		b, err := Parse(tt.b)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.b, err)
		}
		if got := Compare(a, b); got != tt.want {
			t.Errorf("Compare(%s, %s) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestBump(t *testing.T) {
	tests := []struct {
		in    string
		level Level
		want  string
	}{
		{"1.2.3", Major, "2.0.0"},
		{"1.2.3", Minor, "1.3.0"},
		{"1.2.3", Patch, "1.2.4"},
		{"0.1.0", Patch, "0.1.1"},
		{"0.1.0", Minor, "0.2.0"},
	}
	for _, tt := range tests {
		v, err := Parse(tt.in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.in, err)
		}
		got, err := Bump(v, tt.level)
		if err != nil {
			t.Fatalf("Bump(%q, %s): %v", tt.in, tt.level, err)
		}
		if got.String() != tt.want {
			t.Errorf("Bump(%q, %s) = %q, want %q", tt.in, tt.level, got.String(), tt.want)
		}
	}
}

func TestBump_UnknownLevel(t *testing.T) {
	v, _ := Parse("1.0.0")
	if _, err := Bump(v, "patchh"); err == nil {
		t.Error("Bump with an unknown level: want error, got nil")
	}
}
