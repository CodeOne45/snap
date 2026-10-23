package plan

import (
	"strings"
	"testing"
)

func TestBuilder_ModifyFile(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.ModifyFile("package.json", "abc123", []byte(`{"name":"x"}`), 0o644); err != nil {
		t.Fatalf("ModifyFile: %v", err)
	}

	p := b.Build()
	if len(p.Ops) != 1 {
		t.Fatalf("len(Ops) = %d, want 1", len(p.Ops))
	}
	mf, ok := p.Ops[0].(ModifyFileOp)
	if !ok {
		t.Fatalf("Ops[0] = %#v, want ModifyFileOp", p.Ops[0])
	}
	if mf.Path != "package.json" || mf.ExpectedSHA256 != "abc123" || string(mf.Content) != `{"name":"x"}` {
		t.Errorf("ModifyFileOp = %+v, want Path=package.json ExpectedSHA256=abc123", mf)
	}
	if mf.Kind() != OpModifyFile {
		t.Errorf("Kind() = %q, want %q", mf.Kind(), OpModifyFile)
	}
}

func TestBuilder_ModifyFile_SamePathAsWriteFileIsDuplicate(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.WriteFile("package.json", []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := b.ModifyFile("package.json", "abc123", []byte("{}"), 0o644); err == nil {
		t.Fatal("ModifyFile at a path already claimed by WriteFile: want error, got nil")
	}
}

func TestJSON_ModifyFileOp_RoundTrip(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.ModifyFile("package.json", "abc123def456", []byte(`{"dependencies":{}}`), 0o644); err != nil {
		t.Fatalf("ModifyFile: %v", err)
	}

	data, err := ToJSON(b.Build())
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	got, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	if len(got.Ops) != 1 {
		t.Fatalf("len(Ops) = %d, want 1", len(got.Ops))
	}
	mf, ok := got.Ops[0].(ModifyFileOp)
	if !ok {
		t.Fatalf("Ops[0] = %#v, want ModifyFileOp", got.Ops[0])
	}
	if mf.Path != "package.json" || mf.ExpectedSHA256 != "abc123def456" || string(mf.Content) != `{"dependencies":{}}` || mf.Mode != 0o644 {
		t.Errorf("round-tripped ModifyFileOp = %+v, want the original", mf)
	}
}

// TestJSON_ModifyFileSchemaStability pins modify_file's exact byte shape,
// per M20's addition to docs/cli-json-contract.md.
func TestJSON_ModifyFileSchemaStability(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.ModifyFile("package.json", "deadbeef", []byte(`{}`), 0o644); err != nil {
		t.Fatalf("ModifyFile: %v", err)
	}
	data, err := ToJSON(b.Build())
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	want := `{
  "target_dir": "myproj",
  "ops": [
    {
      "kind": "modify_file",
      "path": "package.json",
      "content": "e30=",
      "mode": 420,
      "expected_sha256": "deadbeef"
    }
  ]
}`
	if string(data) != want {
		t.Errorf("ModifyFileOp JSON shape changed — update docs/cli-json-contract.md if deliberate.\ngot:\n%s\nwant:\n%s", data, want)
	}
}

func TestDryRun_ModifyFile(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.ModifyFile("package.json", "abc", []byte(`{"a":1}`), 0o644); err != nil {
		t.Fatalf("ModifyFile: %v", err)
	}
	var buf strings.Builder
	if err := DryRun(&buf, b.Build()); err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "package.json") || !strings.Contains(got, "7 bytes") {
		t.Errorf("DryRun output = %q, want it to mention package.json and its new size", got)
	}
}
