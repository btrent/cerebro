// Package paste implements the "collapse large pastes" behaviour. When the user
// pastes more than Threshold lines, the input area shows a compact placeholder
// like "[Pasted Content 42 Lines]" while the full text is preserved and expanded
// back in just before the prompt is sent to the model.
//
// This logic is intentionally separate from the TUI so it can be unit-tested and
// reused regardless of how paste events are detected by the terminal.
package paste

import (
	"fmt"
	"strings"
)

// Threshold is the number of lines a paste may have before it is collapsed.
// "More than 10 lines" collapses, so 10 or fewer lines pass through unchanged.
const Threshold = 10

// Buffer maps placeholder tokens to their full pasted content for one input
// session. A new Buffer is cheap; create one per input area.
type Buffer struct {
	items map[string]string
}

// New returns an empty Buffer.
func New() *Buffer { return &Buffer{items: map[string]string{}} }

// CountLines reports the number of lines in text (empty string is 0 lines).
func CountLines(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

// Collapse decides whether pasted text should be collapsed. If it has more than
// Threshold lines, Collapse stores the full text and returns a placeholder token
// for display; otherwise it returns the original text unchanged.
func (b *Buffer) Collapse(text string) string {
	n := CountLines(text)
	if n <= Threshold {
		return text
	}
	token := fmt.Sprintf("[Pasted Content %d Lines]", n)
	// Guarantee uniqueness so two equal-sized pastes expand to the right content.
	if _, exists := b.items[token]; exists {
		for i := 2; ; i++ {
			alt := fmt.Sprintf("[Pasted Content %d Lines (%d)]", n, i)
			if _, dup := b.items[alt]; !dup {
				token = alt
				break
			}
		}
	}
	b.items[token] = text
	return token
}

// Expand replaces every known placeholder token found in s with its full text.
// Tokens that aren't present in s are simply ignored.
func (b *Buffer) Expand(s string) string {
	if len(b.items) == 0 {
		return s
	}
	for token, full := range b.items {
		s = strings.ReplaceAll(s, token, full)
	}
	return s
}

// HasCollapsed reports whether any paste has been collapsed in this buffer.
func (b *Buffer) HasCollapsed() bool { return len(b.items) > 0 }

// Reset clears all stored pastes (e.g. after the prompt is submitted).
func (b *Buffer) Reset() { b.items = map[string]string{} }
