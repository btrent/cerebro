// Package llm defines the provider abstraction for local LLM backends and the
// concrete clients that implement it. Adding a new backend means adding a
// constructor to the registry; no other code needs to change.
package llm

import "context"

// Role identifies who authored a message in a conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Image is a single image attachment. Data holds the raw file bytes; MIME is the
// detected content type (e.g. "image/png"). Path is kept for display/debugging.
type Image struct {
	Path string
	MIME string
	Data []byte
}

// Message is one turn in a conversation. Images is only meaningful for user
// messages sent to multimodal-capable providers.
type Message struct {
	Role   Role
	Text   string
	Images []Image
}

// ChatRequest is a provider-agnostic inference request.
type ChatRequest struct {
	Model       string
	Messages    []Message
	Stream      bool
	Temperature float64
	MaxTokens   int
}

// Chunk is a single streamed unit of a response. Exactly one of Delta, Done, or
// Err is meaningful per chunk: Delta carries text, Done signals completion, and
// Err carries a terminal error (after which the channel is closed).
type Chunk struct {
	Delta string
	Done  bool
	Err   error
}

// Provider is implemented by every backend. Stream returns a channel that emits
// Chunks until a Done or Err chunk, after which it is closed. Implementations
// must honour ctx cancellation so the UI can abort an in-flight response.
type Provider interface {
	// Name is the human-facing provider kind, e.g. "openai" or "ollama".
	Name() string
	// SupportsImages reports whether this provider+model can accept image input.
	SupportsImages() bool
	// Stream starts inference and returns a channel of response chunks.
	Stream(ctx context.Context, req ChatRequest) (<-chan Chunk, error)
}
