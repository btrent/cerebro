package paste

import (
	"strings"
	"testing"
)

func TestCountLines(t *testing.T) {
	cases := map[string]int{
		"":           0,
		"one":        1,
		"a\nb":       2,
		"a\nb\nc":    3,
		"trailing\n": 2,
	}
	for in, want := range cases {
		if got := CountLines(in); got != want {
			t.Errorf("CountLines(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestCollapseSmallPasteUnchanged(t *testing.T) {
	b := New()
	small := strings.Repeat("line\n", 9) + "last" // 10 lines
	if got := b.Collapse(small); got != small {
		t.Errorf("10-line paste should pass through unchanged")
	}
	if b.HasCollapsed() {
		t.Error("nothing should be collapsed")
	}
}

func TestCollapseLargePaste(t *testing.T) {
	b := New()
	full := strings.Repeat("line\n", 41) + "last" // 42 lines
	display := b.Collapse(full)

	if display != "[Pasted Content 42 Lines]" {
		t.Fatalf("unexpected placeholder: %q", display)
	}
	if !b.HasCollapsed() {
		t.Error("expected HasCollapsed to be true")
	}

	// The full content must be recoverable for the model.
	typed := "Please review:\n" + display + "\nThanks"
	expanded := b.Expand(typed)
	if !strings.Contains(expanded, full) {
		t.Error("expanded text must contain the full pasted content")
	}
	if strings.Contains(expanded, "[Pasted Content") {
		t.Error("placeholder should be gone after expansion")
	}
}

func TestCollapseDuplicateSizesAreUnique(t *testing.T) {
	b := New()
	a := strings.Repeat("a\n", 11) + "x" // 12 lines
	c := strings.Repeat("b\n", 11) + "y" // 12 lines, different content

	ta := b.Collapse(a)
	tc := b.Collapse(c)
	if ta == tc {
		t.Fatalf("duplicate-size pastes produced identical tokens: %q", ta)
	}

	combined := ta + "\n" + tc
	expanded := b.Expand(combined)
	if !strings.Contains(expanded, a) || !strings.Contains(expanded, c) {
		t.Error("both pastes must expand to their own content")
	}
}

func TestReset(t *testing.T) {
	b := New()
	b.Collapse(strings.Repeat("x\n", 20))
	b.Reset()
	if b.HasCollapsed() {
		t.Error("Reset should clear stored pastes")
	}
}
