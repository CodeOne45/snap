package render

import "testing"

func TestPath(t *testing.T) {
	vars := Vars{"name": "acme-site"}
	got, err := Path("{{name}}/src/{{name|pascal}}.go", vars)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	want := "acme-site/src/AcmeSite.go"
	if got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

func TestContent_Text(t *testing.T) {
	vars := Vars{"name": "acme-site"}
	got, err := Content([]byte("package {{name|pascal}}"), vars)
	if err != nil {
		t.Fatalf("Content: %v", err)
	}
	want := "package AcmeSite"
	if string(got) != want {
		t.Errorf("Content = %q, want %q", got, want)
	}
}

func TestContent_BinaryPassthrough(t *testing.T) {
	vars := Vars{"name": "acme-site"}
	// Contains a null byte and an unterminated "{{" that would be a render
	// error if parsed — must be skipped entirely because it's binary.
	binary := []byte{0x00, '{', '{', 'b', 'r', 'o', 'k', 'e', 'n'}

	got, err := Content(binary, vars)
	if err != nil {
		t.Fatalf("Content(binary): %v", err)
	}
	if string(got) != string(binary) {
		t.Errorf("Content(binary) = %v, want unchanged %v", got, binary)
	}
}
