// Package tokens approximates how many LLM tokens a piece of text would
// consume. There is no universal offline tokenizer for every model (and
// Claude's exact one isn't available offline at all) — every Counter here
// is an approximation; display it with "~" wherever it's shown to a user.
package tokens

// Counter approximates a text's token count. Implementations are pure and
// deterministic: the same text always yields the same count.
type Counter interface {
	Count(text string) int
}

// Default is the Counter snap uses unless a caller swaps in something
// else.
var Default Counter = HeuristicCounter{}

// HeuristicCounter approximates token count from word and punctuation
// boundaries, with no external tokenizer or vocabulary: each run of
// letters/digits/underscores ("a word") costs ceil(length/4) tokens — long
// or unusual words cost more, approximating how a real BPE tokenizer
// splits them into sub-word pieces — and every other non-whitespace
// character (punctuation, symbols) costs its own token, since code and
// prose both use these as meaningful boundaries.
type HeuristicCounter struct{}

// Count implements Counter.
func (HeuristicCounter) Count(text string) int {
	total := 0
	wordLen := 0

	flush := func() {
		if wordLen > 0 {
			total += ceilDiv(wordLen, 4)
			wordLen = 0
		}
	}

	for _, r := range text {
		switch {
		case isWordChar(r):
			wordLen++
		case isSpace(r):
			flush()
		default:
			flush()
			total++
		}
	}
	flush()

	return total
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f'
}

func ceilDiv(a, b int) int {
	return (a + b - 1) / b
}
