package render

// sniffLimit is how many leading bytes are inspected, matching the
// convention used by git and the file(1) command.
const sniffLimit = 8000

// IsBinary reports whether content should be treated as binary: copied
// byte-for-byte and never run through Render.
func IsBinary(content []byte) bool {
	n := len(content)
	if n > sniffLimit {
		n = sniffLimit
	}
	for _, b := range content[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}
