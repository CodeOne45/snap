package plan

import (
	"reflect"
	"strings"
	"testing"
)

func TestJSON_RoundTrip(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.CreateDir("src"); err != nil {
		t.Fatalf("CreateDir: %v", err)
	}
	if err := b.WriteFile("src/main.go", []byte("package main"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	want := b.Build()

	data, err := ToJSON(want)
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	got, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}

	if got.TargetDir != want.TargetDir {
		t.Errorf("TargetDir = %q, want %q", got.TargetDir, want.TargetDir)
	}
	if len(got.Ops) != len(want.Ops) {
		t.Fatalf("len(Ops) = %d, want %d", len(got.Ops), len(want.Ops))
	}
	if !reflect.DeepEqual(got.Ops[0], want.Ops[0]) {
		t.Errorf("Ops[0] = %#v, want %#v", got.Ops[0], want.Ops[0])
	}
	if !reflect.DeepEqual(got.Ops[1], want.Ops[1]) {
		t.Errorf("Ops[1] = %#v, want %#v", got.Ops[1], want.Ops[1])
	}
}

func TestJSON_TaggedUnionShape(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.CreateDir("src"); err != nil {
		t.Fatalf("CreateDir: %v", err)
	}
	data, err := ToJSON(b.Build())
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"kind": "create_dir"`) {
		t.Errorf("JSON output missing kind discriminator: %s", s)
	}
	if !strings.Contains(s, `"path": "src"`) {
		t.Errorf("JSON output missing path field: %s", s)
	}
}

// TestJSON_SchemaStability pins the exact byte shape of Plan's JSON output
// for a canonical input. This is the CLI JSON contract M7 locks down before
// the MCP server depends on it (docs/cli-json-contract.md documents the
// shape in prose); a deliberate format change must update both.
func TestJSON_SchemaStability(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.CreateDir("src"); err != nil {
		t.Fatalf("CreateDir: %v", err)
	}
	if err := b.WriteFile("src/main.go", []byte("package main"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	data, err := ToJSON(b.Build())
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	want := `{
  "target_dir": "myproj",
  "ops": [
    {
      "kind": "create_dir",
      "path": "src"
    },
    {
      "kind": "write_file",
      "path": "src/main.go",
      "content": "cGFja2FnZSBtYWlu",
      "mode": 420
    }
  ]
}`
	if string(data) != want {
		t.Errorf("Plan JSON shape changed — update docs/cli-json-contract.md if this is deliberate.\ngot:\n%s\nwant:\n%s", data, want)
	}
}

func TestFromJSON_UnknownKind(t *testing.T) {
	data := []byte(`{"target_dir":"myproj","ops":[{"kind":"delete_file","path":"ignored"}]}`)
	if _, err := FromJSON(data); err == nil {
		t.Fatal("FromJSON with unknown op kind: want error, got nil")
	}
}

func TestJSON_RunHookOp(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.RunHook("git init"); err != nil {
		t.Fatalf("RunHook: %v", err)
	}
	if err := b.RunHook("go mod tidy"); err != nil {
		t.Fatalf("RunHook: %v", err)
	}

	data, err := ToJSON(b.Build())
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	got, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	hooks := got.Hooks()
	if len(hooks) != 2 || hooks[0].Run != "git init" || hooks[1].Run != "go mod tidy" {
		t.Errorf("Hooks() = %+v, want [git init, go mod tidy] in order", hooks)
	}
}

// TestJSON_RunHookSchemaStability pins run_hook's exact byte shape, per
// M9's addition to docs/cli-json-contract.md.
func TestJSON_RunHookSchemaStability(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.RunHook("git init"); err != nil {
		t.Fatalf("RunHook: %v", err)
	}
	data, err := ToJSON(b.Build())
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	want := `{
  "target_dir": "myproj",
  "ops": [
    {
      "kind": "run_hook",
      "run": "git init"
    }
  ]
}`
	if string(data) != want {
		t.Errorf("run_hook JSON shape changed — update docs/cli-json-contract.md if this is deliberate.\ngot:\n%s\nwant:\n%s", data, want)
	}
}
