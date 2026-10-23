package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewRootCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(--help): %v", err)
	}
	const wantSubstr = "Capture existing projects as templates"
	if got := buf.String(); !strings.Contains(got, wantSubstr) {
		t.Errorf("help output = %q, want it to contain %q", got, wantSubstr)
	}
	if cmd.Use != "snap" {
		t.Errorf("cmd.Use = %q, want %q", cmd.Use, "snap")
	}
}
