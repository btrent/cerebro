package prompts

import (
	"strings"

	"cerebro/internal/llm"
)

// Assemble builds the message list sent to the model: the active header prompt
// (if any) as a leading system message, then prior conversation turns, then the
// new user message (which may carry image attachments). It is a pure function so
// the exact wire ordering can be unit-tested.
func Assemble(headerBody string, history []llm.Message, user llm.Message) []llm.Message {
	msgs := make([]llm.Message, 0, len(history)+2)
	if strings.TrimSpace(headerBody) != "" {
		msgs = append(msgs, llm.Message{Role: llm.RoleSystem, Text: headerBody})
	}
	msgs = append(msgs, history...)
	msgs = append(msgs, user)
	return msgs
}
