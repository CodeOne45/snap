package plan

import "testing"

// fakeCounter is a trivial, predictable tokens.Counter for tests: one
// "token" per byte, so expected totals are easy to compute by hand.
type fakeCounter struct{}

func (fakeCounter) Count(text string) int { return len(text) }

func TestComputeStats_CountsFilesAndBytes(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.WriteFile("README.md", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := b.WriteFile("src/main.go", []byte("package main"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := b.CreateDir("empty"); err != nil {
		t.Fatalf("CreateDir: %v", err)
	}

	st := ComputeStats(b.Build(), fakeCounter{})
	if st.Files != 2 {
		t.Errorf("Files = %d, want 2 (CreateDirOp doesn't count)", st.Files)
	}
	if st.TextFiles != 2 {
		t.Errorf("TextFiles = %d, want 2", st.TextFiles)
	}
	if st.Bytes != int64(len("hello")+len("package main")) {
		t.Errorf("Bytes = %d, want %d", st.Bytes, len("hello")+len("package main"))
	}
	if st.ApproxTokens != len("hello")+len("package main") {
		t.Errorf("ApproxTokens = %d, want %d", st.ApproxTokens, len("hello")+len("package main"))
	}
}

func TestComputeStats_BinaryCountsBytesNotTokens(t *testing.T) {
	b := NewBuilder("myproj")
	binaryContent := []byte{0x89, 'P', 'N', 'G', 0, 1, 2, 3}
	if err := b.WriteFile("logo.png", binaryContent, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	st := ComputeStats(b.Build(), fakeCounter{})
	if st.Files != 1 {
		t.Errorf("Files = %d, want 1", st.Files)
	}
	if st.TextFiles != 0 {
		t.Errorf("TextFiles = %d, want 0 (binary file)", st.TextFiles)
	}
	if st.Bytes != int64(len(binaryContent)) {
		t.Errorf("Bytes = %d, want %d", st.Bytes, len(binaryContent))
	}
	if st.ApproxTokens != 0 {
		t.Errorf("ApproxTokens = %d, want 0 (binary files contribute no tokens)", st.ApproxTokens)
	}
}

func TestComputeStats_IgnoresHooksAndEmptyPlan(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.RunHook("git init"); err != nil {
		t.Fatalf("RunHook: %v", err)
	}
	st := ComputeStats(b.Build(), fakeCounter{})
	if st != (Stats{}) {
		t.Errorf("Stats = %+v, want the zero value (only a hook, no files)", st)
	}
}

// TestComputeStats_CountsModifyFileOp covers M20: a recipe's dependency
// and env edits are ModifyFileOp, not WriteFileOp, but they still write
// real bytes to disk and must count the same way toward the receipt.
func TestComputeStats_CountsModifyFileOp(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.WriteFile("README.md", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := b.ModifyFile("package.json", "deadbeef", []byte("package.json content"), 0o644); err != nil {
		t.Fatalf("ModifyFile: %v", err)
	}

	st := ComputeStats(b.Build(), fakeCounter{})
	if st.Files != 2 {
		t.Errorf("Files = %d, want 2 (ModifyFileOp counts too)", st.Files)
	}
	if st.TextFiles != 2 {
		t.Errorf("TextFiles = %d, want 2", st.TextFiles)
	}
	wantBytes := int64(len("hello") + len("package.json content"))
	if st.Bytes != wantBytes {
		t.Errorf("Bytes = %d, want %d", st.Bytes, wantBytes)
	}
}

func TestComputeStats_Deterministic(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.WriteFile("a.txt", []byte("abc"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	p := b.Build()
	first := ComputeStats(p, fakeCounter{})
	for i := 0; i < 5; i++ {
		if got := ComputeStats(p, fakeCounter{}); got != first {
			t.Fatalf("ComputeStats is not deterministic: got %+v, want %+v", got, first)
		}
	}
}
