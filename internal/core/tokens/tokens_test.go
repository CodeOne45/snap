package tokens

import "testing"

func TestHeuristicCounter_Empty(t *testing.T) {
	if got := (HeuristicCounter{}).Count(""); got != 0 {
		t.Errorf("Count(\"\") = %d, want 0", got)
	}
}

func TestHeuristicCounter_WhitespaceOnly(t *testing.T) {
	if got := (HeuristicCounter{}).Count("   \n\t  \n"); got != 0 {
		t.Errorf("Count(whitespace) = %d, want 0", got)
	}
}

func TestHeuristicCounter_ShortWord(t *testing.T) {
	// A word of 4 or fewer characters is always at least 1 token, never 0.
	if got := (HeuristicCounter{}).Count("hi"); got != 1 {
		t.Errorf("Count(\"hi\") = %d, want 1", got)
	}
}

func TestHeuristicCounter_LongWordSplitsIntoMultipleTokens(t *testing.T) {
	// 16 letters -> ceil(16/4) = 4 sub-word tokens, approximating how a
	// real BPE tokenizer splits long/rare words into pieces.
	if got := (HeuristicCounter{}).Count("abcdefghijklmnop"); got != 4 {
		t.Errorf("Count(16-char word) = %d, want 4", got)
	}
}

func TestHeuristicCounter_TwoWords(t *testing.T) {
	// "hello" and "world": ceil(5/4) = 2 tokens each.
	if got := (HeuristicCounter{}).Count("hello world"); got != 4 {
		t.Errorf("Count(\"hello world\") = %d, want 4", got)
	}
}

func TestHeuristicCounter_PunctuationCountsSeparately(t *testing.T) {
	// "a" (1) + "." (1, its own symbol token) + "b" (1) = 3.
	if got := (HeuristicCounter{}).Count("a.b"); got != 3 {
		t.Errorf("Count(\"a.b\") = %d, want 3", got)
	}
}

func TestHeuristicCounter_Code(t *testing.T) {
	// "func"(1) "main"(1) "("(1) ")"(1) "{"(1) = 5.
	if got := (HeuristicCounter{}).Count("func main() {"); got != 5 {
		t.Errorf("Count(code snippet) = %d, want 5", got)
	}
}

func TestHeuristicCounter_UnderscoresAreWordChars(t *testing.T) {
	// "my_var_name" is one 11-char identifier: ceil(11/4) = 3.
	if got := (HeuristicCounter{}).Count("my_var_name"); got != 3 {
		t.Errorf("Count(\"my_var_name\") = %d, want 3", got)
	}
}

func TestHeuristicCounter_Deterministic(t *testing.T) {
	c := HeuristicCounter{}
	text := "package main\n\nfunc main() {\n\tfmt.Println(\"hello, world\")\n}\n"
	first := c.Count(text)
	for i := 0; i < 5; i++ {
		if got := c.Count(text); got != first {
			t.Fatalf("Count is not deterministic: got %d, want %d", got, first)
		}
	}
}

func TestDefault_IsHeuristicCounter(t *testing.T) {
	if _, ok := Default.(HeuristicCounter); !ok {
		t.Errorf("Default = %T, want HeuristicCounter", Default)
	}
}
