// Package remote parses a remote template reference — "gh:user/repo", any
// git URL, optionally pinned to a tag or commit with "@". Pure string
// parsing, no I/O: internal/adapters/gitcache does the actual cloning.
package remote

import "strings"

// Ref is a parsed remote template reference.
type Ref struct {
	URL string
	Pin string // "" means unpinned: resolve the remote's default branch HEAD
}

// String renders ref back to its canonical "<url>@<pin>" / "<url>" form,
// used as .snap/origin.toml's "source" field.
func (r Ref) String() string {
	if r.Pin == "" {
		return r.URL
	}
	return r.URL + "@" + r.Pin
}

// ParseRef parses s as a remote template reference. ok is false if s
// doesn't look like one at all (e.g. a plain local template name) — the
// caller should fall back to the local template store.
func ParseRef(s string) (Ref, bool) {
	if !looksRemote(s) {
		return Ref{}, false
	}
	url, pin := splitPin(s)
	return Ref{URL: expandGHShorthand(url), Pin: pin}, true
}

func looksRemote(s string) bool {
	return strings.HasPrefix(s, "gh:") || strings.Contains(s, "://") || looksLikeSCP(s)
}

// looksLikeSCP reports whether s looks like SCP-style git syntax,
// "user@host:path" (e.g. "git@github.com:user/repo.git"). Such a
// reference can be used as a remote URL but can't be pinned via a
// trailing "@tag": that "@" would be ambiguous with the one already
// separating user from host. Use an https:// URL or gh: shorthand to pin.
func looksLikeSCP(s string) bool {
	if strings.Contains(s, "://") {
		return false
	}
	at := strings.Index(s, "@")
	if at < 0 {
		return false
	}
	return strings.Contains(s[at:], ":")
}

// splitPin splits a trailing "@pin" off s, but only when the part before
// it already looks like a complete reference on its own (has a scheme,
// starts with "gh:", or ends in ".git") — so the "@" in SCP syntax
// ("git@host:path", with no pin) is never mistaken for a pin separator.
func splitPin(s string) (url, pin string) {
	at := strings.LastIndex(s, "@")
	if at < 0 {
		return s, ""
	}
	before := s[:at]
	if strings.Contains(before, "://") || strings.HasPrefix(before, "gh:") || strings.HasSuffix(before, ".git") {
		return before, s[at+1:]
	}
	return s, ""
}

func expandGHShorthand(url string) string {
	if rest, ok := strings.CutPrefix(url, "gh:"); ok {
		return "https://github.com/" + rest + ".git"
	}
	return url
}
