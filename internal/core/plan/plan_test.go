package plan

import "testing"

func TestBuilder_HappyPath(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.CreateDir("src"); err != nil {
		t.Fatalf("CreateDir: %v", err)
	}
	if err := b.WriteFile("src/main.go", []byte("package main"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := b.Build()
	if p.TargetDir != "myproj" {
		t.Fatalf("TargetDir = %q, want %q", p.TargetDir, "myproj")
	}
	if len(p.Ops) != 2 {
		t.Fatalf("len(Ops) = %d, want 2", len(p.Ops))
	}
	if dir, ok := p.Ops[0].(CreateDirOp); !ok || dir.Path != "src" {
		t.Errorf("Ops[0] = %#v, want CreateDirOp{Path: \"src\"}", p.Ops[0])
	}
	wf, ok := p.Ops[1].(WriteFileOp)
	if !ok || wf.Path != "src/main.go" || string(wf.Content) != "package main" {
		t.Errorf("Ops[1] = %#v, want WriteFileOp{src/main.go, \"package main\"}", p.Ops[1])
	}
}

func TestBuilder_RejectsDuplicatePaths(t *testing.T) {
	tests := []struct {
		name  string
		first func(b *Builder) error
		again func(b *Builder) error
	}{
		{
			name:  "write then write same path",
			first: func(b *Builder) error { return b.WriteFile("a.txt", []byte("1"), 0o644) },
			again: func(b *Builder) error { return b.WriteFile("a.txt", []byte("2"), 0o644) },
		},
		{
			name:  "dir then file at same path",
			first: func(b *Builder) error { return b.CreateDir("thing") },
			again: func(b *Builder) error { return b.WriteFile("thing", []byte("x"), 0o644) },
		},
		{
			name:  "file then dir at same path",
			first: func(b *Builder) error { return b.WriteFile("thing", []byte("x"), 0o644) },
			again: func(b *Builder) error { return b.CreateDir("thing") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder("myproj")
			if err := tt.first(b); err != nil {
				t.Fatalf("first op: %v", err)
			}
			if err := tt.again(b); err == nil {
				t.Error("second op on same path: want error, got nil")
			}
		})
	}
}

func TestBuilder_RejectsEscapes(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"parent escape", "../evil"},
		{"nested parent escape", "a/../../evil"},
		{"absolute", "/etc/passwd"},
		{"empty", ""},
		{"dot", "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder("myproj")
			if err := b.CreateDir(tt.path); err == nil {
				t.Errorf("CreateDir(%q): want error, got nil", tt.path)
			}
			if err := b.WriteFile(tt.path, []byte("x"), 0o644); err == nil {
				t.Errorf("WriteFile(%q): want error, got nil", tt.path)
			}
		})
	}
}

func TestBuilder_RunHook(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.RunHook("git init"); err != nil {
		t.Fatalf("RunHook: %v", err)
	}
	if err := b.RunHook("go mod tidy"); err != nil {
		t.Fatalf("RunHook: %v", err)
	}
	p := b.Build()
	if len(p.Ops) != 2 {
		t.Fatalf("len(Ops) = %d, want 2", len(p.Ops))
	}
	h0, ok := p.Ops[0].(RunHookOp)
	if !ok || h0.Run != "git init" {
		t.Errorf("Ops[0] = %#v, want RunHookOp{Run: \"git init\"}", p.Ops[0])
	}
}

func TestBuilder_RunHook_EmptyCommand(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.RunHook("   "); err == nil {
		t.Error("RunHook(blank): want error, got nil")
	}
}

func TestPlan_WithoutHooks(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.WriteFile("README.md", []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := b.RunHook("git init"); err != nil {
		t.Fatalf("RunHook: %v", err)
	}

	p := b.Build().WithoutHooks()
	if len(p.Ops) != 1 {
		t.Fatalf("len(Ops) = %d, want 1 (hook stripped)", len(p.Ops))
	}
	if _, ok := p.Ops[0].(WriteFileOp); !ok {
		t.Errorf("Ops[0] = %#v, want the WriteFileOp to survive", p.Ops[0])
	}
	if len(p.Hooks()) != 0 {
		t.Errorf("Hooks() after WithoutHooks = %+v, want none", p.Hooks())
	}
}
