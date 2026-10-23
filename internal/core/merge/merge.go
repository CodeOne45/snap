// Package merge implements a line-based, diff3-style three-way merge: the
// same approach git merge-file and GNU diff3 use. It's used by M16's
// snap update to combine a template's changes between versions with a
// user's local edits, producing conflict markers only where the two
// actually overlap.
package merge

import (
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// Result is the outcome of a three-way merge.
type Result struct {
	// Content is the merged text. If Conflict is true, it contains
	// <<<<<<< ours / ======= / >>>>>>> theirs markers around each
	// conflicting region, with every non-conflicting region already
	// resolved.
	Content  string
	Conflict bool
}

// Merge three-way merges ours and theirs against their common base, line
// by line.
func Merge(base, ours, theirs string) Result {
	baseLines := splitLines(base)
	oursHunks := computeHunks(base, ours)
	theirsHunks := computeHunks(base, theirs)
	return merge(baseLines, oursHunks, theirsHunks)
}

// hunk is a region of base (the half-open line range [BaseStart, BaseEnd))
// that one side replaced with Lines. A pure insertion has BaseStart ==
// BaseEnd; a pure deletion has empty Lines.
type hunk struct {
	BaseStart, BaseEnd int
	Lines              []string
}

// computeHunks diffs base against other in line mode (via
// diffmatchpatch's line-hashing trick) and returns every region where
// other actually differs from base — everything else is, by omission,
// unchanged.
func computeHunks(base, other string) []hunk {
	dmp := diffmatchpatch.New()
	c1, c2, lines := dmp.DiffLinesToChars(base, other)
	diffs := dmp.DiffMainRunes([]rune(c1), []rune(c2), false)
	diffs = dmp.DiffCharsToLines(diffs, lines)

	var hunks []hunk
	baseCursor := 0
	i := 0
	for i < len(diffs) {
		d := diffs[i]
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			baseCursor += len(splitLines(d.Text))
			i++
		case diffmatchpatch.DiffDelete:
			start := baseCursor
			baseCursor += len(splitLines(d.Text))
			var insLines []string
			if i+1 < len(diffs) && diffs[i+1].Type == diffmatchpatch.DiffInsert {
				insLines = splitLines(diffs[i+1].Text)
				i += 2
			} else {
				i++
			}
			hunks = append(hunks, hunk{BaseStart: start, BaseEnd: baseCursor, Lines: insLines})
		case diffmatchpatch.DiffInsert:
			insLines := splitLines(d.Text)
			hunks = append(hunks, hunk{BaseStart: baseCursor, BaseEnd: baseCursor, Lines: insLines})
			i++
		}
	}
	return hunks
}

// merge walks oursHunks and theirsHunks in lockstep, keyed by their
// position in baseLines (the diff3 algorithm): a hunk from one side that
// doesn't overlap any hunk from the other is applied cleanly; overlapping
// hunks are clustered together and compared — identical reconstructions
// merge cleanly, differing ones become a conflict region.
func merge(baseLines []string, oursHunks, theirsHunks []hunk) Result {
	var out []string
	conflict := false
	cursor := 0
	oi, ti := 0, 0

	for oi < len(oursHunks) || ti < len(theirsHunks) {
		var oh, th *hunk
		if oi < len(oursHunks) {
			oh = &oursHunks[oi]
		}
		if ti < len(theirsHunks) {
			th = &theirsHunks[ti]
		}

		switch {
		case oh != nil && (th == nil || oh.BaseEnd <= th.BaseStart):
			out = append(out, baseLines[cursor:oh.BaseStart]...)
			out = append(out, oh.Lines...)
			cursor = oh.BaseEnd
			oi++
		case th != nil && (oh == nil || th.BaseEnd <= oh.BaseStart):
			out = append(out, baseLines[cursor:th.BaseStart]...)
			out = append(out, th.Lines...)
			cursor = th.BaseEnd
			ti++
		default:
			oGroup := []hunk{*oh}
			tGroup := []hunk{*th}
			start := min(oh.BaseStart, th.BaseStart)
			end := max(oh.BaseEnd, th.BaseEnd)
			oi++
			ti++
			for {
				grew := false
				for oi < len(oursHunks) && oursHunks[oi].BaseStart < end {
					oGroup = append(oGroup, oursHunks[oi])
					if oursHunks[oi].BaseEnd > end {
						end = oursHunks[oi].BaseEnd
						grew = true
					}
					oi++
				}
				for ti < len(theirsHunks) && theirsHunks[ti].BaseStart < end {
					tGroup = append(tGroup, theirsHunks[ti])
					if theirsHunks[ti].BaseEnd > end {
						end = theirsHunks[ti].BaseEnd
						grew = true
					}
					ti++
				}
				if !grew {
					break
				}
			}

			oursSide := reconstruct(baseLines, oGroup, start, end)
			theirsSide := reconstruct(baseLines, tGroup, start, end)
			out = append(out, baseLines[cursor:start]...)
			if linesEqual(oursSide, theirsSide) {
				out = append(out, oursSide...)
			} else {
				conflict = true
				out = append(out, withNewline("<<<<<<< ours"))
				out = append(out, oursSide...)
				out = append(out, withNewline("======="))
				out = append(out, theirsSide...)
				out = append(out, withNewline(">>>>>>> theirs"))
			}
			cursor = end
		}
	}
	out = append(out, baseLines[cursor:]...)

	return Result{Content: strings.Join(out, ""), Conflict: conflict}
}

// reconstruct rebuilds one side's content over base[start:end), applying
// every hunk in group (sorted by BaseStart) and copying base verbatim
// between them.
func reconstruct(baseLines []string, group []hunk, start, end int) []string {
	sortHunks(group)
	var out []string
	cur := start
	for _, h := range group {
		out = append(out, baseLines[cur:h.BaseStart]...)
		out = append(out, h.Lines...)
		cur = h.BaseEnd
	}
	out = append(out, baseLines[cur:end]...)
	return out
}

func sortHunks(hunks []hunk) {
	for i := 1; i < len(hunks); i++ {
		for j := i; j > 0 && hunks[j-1].BaseStart > hunks[j].BaseStart; j-- {
			hunks[j-1], hunks[j] = hunks[j], hunks[j-1]
		}
	}
}

func linesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// withNewline appends "\n" to a conflict marker line, matching
// splitLines's convention that every line (but possibly the last) carries
// its own trailing newline.
func withNewline(s string) string {
	return s + "\n"
}

// splitLines splits s into lines, each (except possibly the last, if s
// doesn't end with "\n") still carrying its own trailing "\n" — so
// reconstructing a text is just concatenation, never a join separator.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
