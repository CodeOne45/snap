package merge

import "testing"

func TestMerge_NoChanges(t *testing.T) {
	base := "line1\nline2\nline3\n"
	r := Merge(base, base, base)
	if r.Conflict {
		t.Error("Conflict = true, want false")
	}
	if r.Content != base {
		t.Errorf("Content = %q, want %q", r.Content, base)
	}
}

func TestMerge_OnlyOursChanged(t *testing.T) {
	base := "line1\nline2\nline3\n"
	ours := "line1\nCHANGED\nline3\n"
	r := Merge(base, ours, base)
	if r.Conflict {
		t.Errorf("Conflict = true, want false (only ours changed)")
	}
	if r.Content != ours {
		t.Errorf("Content = %q, want %q", r.Content, ours)
	}
}

func TestMerge_OnlyTheirsChanged(t *testing.T) {
	base := "line1\nline2\nline3\n"
	theirs := "line1\nCHANGED\nline3\n"
	r := Merge(base, base, theirs)
	if r.Conflict {
		t.Errorf("Conflict = true, want false (only theirs changed)")
	}
	if r.Content != theirs {
		t.Errorf("Content = %q, want %q", r.Content, theirs)
	}
}

func TestMerge_BothChangedIdentically(t *testing.T) {
	base := "line1\nline2\nline3\n"
	changed := "line1\nSAME CHANGE\nline3\n"
	r := Merge(base, changed, changed)
	if r.Conflict {
		t.Error("Conflict = true, want false (both sides made the same change)")
	}
	if r.Content != changed {
		t.Errorf("Content = %q, want %q", r.Content, changed)
	}
}

func TestMerge_NonOverlappingChanges(t *testing.T) {
	base := "line1\nline2\nline3\nline4\nline5\n"
	ours := "OURS1\nline2\nline3\nline4\nline5\n"
	theirs := "line1\nline2\nline3\nline4\nTHEIRS5\n"
	r := Merge(base, ours, theirs)
	if r.Conflict {
		t.Errorf("Conflict = true, want false (changes in different parts of the file)")
	}
	want := "OURS1\nline2\nline3\nline4\nTHEIRS5\n"
	if r.Content != want {
		t.Errorf("Content = %q, want %q", r.Content, want)
	}
}

func TestMerge_ConflictingChanges(t *testing.T) {
	base := "line1\nline2\nline3\n"
	ours := "line1\nOURS CHANGE\nline3\n"
	theirs := "line1\nTHEIRS CHANGE\nline3\n"
	r := Merge(base, ours, theirs)
	if !r.Conflict {
		t.Fatal("Conflict = false, want true (both sides changed the same line differently)")
	}
	want := "line1\n<<<<<<< ours\nOURS CHANGE\n=======\nTHEIRS CHANGE\n>>>>>>> theirs\nline3\n"
	if r.Content != want {
		t.Errorf("Content =\n%q\nwant:\n%q", r.Content, want)
	}
}

func TestMerge_PureInsertionInOurs(t *testing.T) {
	base := "line1\nline2\n"
	ours := "line1\nINSERTED\nline2\n"
	r := Merge(base, ours, base)
	if r.Conflict {
		t.Error("Conflict = true, want false")
	}
	if r.Content != ours {
		t.Errorf("Content = %q, want %q", r.Content, ours)
	}
}

func TestMerge_PureDeletionInTheirs(t *testing.T) {
	base := "line1\nline2\nline3\n"
	theirs := "line1\nline3\n"
	r := Merge(base, base, theirs)
	if r.Conflict {
		t.Error("Conflict = true, want false")
	}
	if r.Content != theirs {
		t.Errorf("Content = %q, want %q", r.Content, theirs)
	}
}

func TestMerge_BothDeleteSameLine(t *testing.T) {
	base := "line1\nline2\nline3\n"
	changed := "line1\nline3\n"
	r := Merge(base, changed, changed)
	if r.Conflict {
		t.Error("Conflict = true, want false (both sides deleted the same line)")
	}
	if r.Content != changed {
		t.Errorf("Content = %q, want %q", r.Content, changed)
	}
}

func TestMerge_NoTrailingNewline(t *testing.T) {
	base := "line1\nline2"
	ours := "line1\nOURS"
	r := Merge(base, ours, base)
	if r.Conflict {
		t.Error("Conflict = true, want false")
	}
	if r.Content != ours {
		t.Errorf("Content = %q, want %q", r.Content, ours)
	}
}

func TestMerge_EmptyBase(t *testing.T) {
	r := Merge("", "ours content\n", "")
	if r.Conflict {
		t.Error("Conflict = true, want false")
	}
	if r.Content != "ours content\n" {
		t.Errorf("Content = %q, want %q", r.Content, "ours content\n")
	}
}

func TestMerge_ClusterAbsorbsAThirdHunk(t *testing.T) {
	// ours makes two separate small changes (line b, line e); theirs makes
	// one wide change spanning both (lines b through e). oursHunks[0]
	// overlaps theirsHunks[0] directly, but oursHunks[1] only falls inside
	// the cluster once the overlap region grows to cover theirs' wide
	// hunk -- this exercises the cluster-expansion loop pulling in a
	// hunk beyond the initial overlapping pair.
	base := "a\nb\nc\nd\ne\nf\n"
	ours := "a\nB2\nc\nd\nE2\nf\n"
	theirs := "a\nTHEIRS-WIDE\nf\n"
	r := Merge(base, ours, theirs)
	if !r.Conflict {
		t.Fatal("Conflict = false, want true")
	}
	want := "a\n<<<<<<< ours\nB2\nc\nd\nE2\n=======\nTHEIRS-WIDE\n>>>>>>> theirs\nf\n"
	if r.Content != want {
		t.Errorf("Content =\n%q\nwant:\n%q", r.Content, want)
	}
}

func TestMerge_AdjacentNonOverlappingChangesDoNotConflict(t *testing.T) {
	// ours changes line 2, theirs changes line 3 (adjacent, not overlapping).
	base := "line1\nline2\nline3\nline4\n"
	ours := "line1\nOURS2\nline3\nline4\n"
	theirs := "line1\nline2\nTHEIRS3\nline4\n"
	r := Merge(base, ours, theirs)
	if r.Conflict {
		t.Errorf("Conflict = true, want false (adjacent, non-overlapping changes)")
	}
	want := "line1\nOURS2\nTHEIRS3\nline4\n"
	if r.Content != want {
		t.Errorf("Content = %q, want %q", r.Content, want)
	}
}
