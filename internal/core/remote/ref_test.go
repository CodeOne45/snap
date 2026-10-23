package remote

import "testing"

func TestParseRef(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   Ref
		wantOK bool
	}{
		{"local template name", "api-starter", Ref{}, false},
		{"gh shorthand unpinned", "gh:user/repo", Ref{URL: "https://github.com/user/repo.git"}, true},
		{"gh shorthand pinned to tag", "gh:user/repo@v1.2.0", Ref{URL: "https://github.com/user/repo.git", Pin: "v1.2.0"}, true},
		{"gh shorthand pinned to commit", "gh:user/repo@abc1234", Ref{URL: "https://github.com/user/repo.git", Pin: "abc1234"}, true},
		{"https url unpinned", "https://example.com/user/repo.git", Ref{URL: "https://example.com/user/repo.git"}, true},
		{"https url pinned", "https://example.com/user/repo.git@v2.0.0", Ref{URL: "https://example.com/user/repo.git", Pin: "v2.0.0"}, true},
		{"ssh scheme url pinned", "ssh://git@example.com/user/repo.git@v1.0", Ref{URL: "ssh://git@example.com/user/repo.git", Pin: "v1.0"}, true},
		{"scp syntax unpinned", "git@github.com:user/repo.git", Ref{URL: "git@github.com:user/repo.git"}, true},
		{"scp syntax pinned", "git@github.com:user/repo.git@v1.0", Ref{URL: "git@github.com:user/repo.git", Pin: "v1.0"}, true},
		{"http url", "http://example.com/repo.git", Ref{URL: "http://example.com/repo.git"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseRef(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("ParseRef(%q) ok = %v, want %v", tt.in, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got != tt.want {
				t.Errorf("ParseRef(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestRef_String(t *testing.T) {
	tests := []struct {
		ref  Ref
		want string
	}{
		{Ref{URL: "https://github.com/user/repo.git"}, "https://github.com/user/repo.git"},
		{Ref{URL: "https://github.com/user/repo.git", Pin: "abc1234"}, "https://github.com/user/repo.git@abc1234"},
	}
	for _, tt := range tests {
		if got := tt.ref.String(); got != tt.want {
			t.Errorf("Ref%+v.String() = %q, want %q", tt.ref, got, tt.want)
		}
	}
}
