// Package semver parses and compares strict MAJOR.MINOR.PATCH version
// strings (no pre-release or build metadata), as used by [template].version
// in snap.toml and the local template store's per-version directories.
package semver

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a parsed MAJOR.MINOR.PATCH version.
type Version struct {
	Major, Minor, Patch int
}

// String renders v back to "MAJOR.MINOR.PATCH".
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Parse parses s as a strict MAJOR.MINOR.PATCH version: three non-negative
// integers separated by dots, no leading zeros (other than "0" itself), no
// pre-release or build metadata, no "v" prefix.
func Parse(s string) (Version, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("semver: %q: want MAJOR.MINOR.PATCH", s)
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := parseComponent(p)
		if err != nil {
			return Version{}, fmt.Errorf("semver: %q: %w", s, err)
		}
		nums[i] = n
	}
	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2]}, nil
}

func parseComponent(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty version component")
	}
	if len(s) > 1 && s[0] == '0' {
		return 0, fmt.Errorf("component %q has a leading zero", s)
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("component %q is not a non-negative integer", s)
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("component %q: %w", s, err)
	}
	return n, nil
}

// Compare returns -1, 0, or 1 as a is less than, equal to, or greater than
// b.
func Compare(a, b Version) int {
	switch {
	case a.Major != b.Major:
		return compareInt(a.Major, b.Major)
	case a.Minor != b.Minor:
		return compareInt(a.Minor, b.Minor)
	default:
		return compareInt(a.Patch, b.Patch)
	}
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// Level is a part of a version to increment when bumping.
type Level string

// The three bump levels, matching semver's own MAJOR.MINOR.PATCH parts.
const (
	Major Level = "major"
	Minor Level = "minor"
	Patch Level = "patch"
)

// Bump returns a new Version with level incremented and every component to
// its right reset to zero (e.g. bumping 1.2.3's minor gives 1.3.0).
func Bump(v Version, level Level) (Version, error) {
	switch level {
	case Major:
		return Version{Major: v.Major + 1}, nil
	case Minor:
		return Version{Major: v.Major, Minor: v.Minor + 1}, nil
	case Patch:
		return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}, nil
	default:
		return Version{}, fmt.Errorf("semver: unknown bump level %q (want major, minor, or patch)", level)
	}
}
