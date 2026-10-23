package render

// Path renders a template path (file or directory name, possibly with
// separators): every "{{...}}" token in it is substituted via Render.
func Path(path string, vars Vars) (string, error) {
	return Render(path, vars)
}

// Content renders file content. Binary files are returned unchanged,
// regardless of what byte sequences they happen to contain.
func Content(content []byte, vars Vars) ([]byte, error) {
	if IsBinary(content) {
		return content, nil
	}
	out, err := Render(string(content), vars)
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}
