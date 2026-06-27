// Package openai implements the llm.Provider interface for any OpenAI-compatible
// chat-completions endpoint. This covers mlx_vlm.server (the default local
// backend for Gemma), LM Studio, llama.cpp's server, and Ollama's /v1 shim.
package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cerebro/internal/llm"
)

func init() {
	factory := func(spec llm.ModelSpec) (llm.Provider, error) {
		if spec.BaseURL == "" {
			return nil, fmt.Errorf("openai provider: base_url is required for model %q", spec.Model)
		}
		return &Client{
			model:          spec.Model,
			baseURL:        strings.TrimRight(spec.BaseURL, "/"),
			apiKey:         spec.APIKey,
			supportsImages: spec.SupportsImages,
			http:           &http.Client{Timeout: 0}, // streaming: no overall timeout; ctx governs
		}, nil
	}
	llm.Register("openai", factory)
	// Friendly aliases for the same wire protocol.
	llm.Register("openai-compatible", factory)
	llm.Register("mlx", factory)
}

// Client talks to a single OpenAI-compatible endpoint for a single model.
type Client struct {
	model          string
	baseURL        string
	apiKey         string
	supportsImages bool
	http           *http.Client
}

func (c *Client) Name() string         { return "openai" }
func (c *Client) SupportsImages() bool { return c.supportsImages }

// --- wire types ---------------------------------------------------------------

type wireMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string, or []contentPart for multimodal
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []wireMessage `json:"messages"`
	Stream      bool          `json:"stream"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type streamResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Error *apiError `json:"error"`
}

type fullResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *apiError `json:"error"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

func (e *apiError) Error() string { return e.Message }

// --- request building ---------------------------------------------------------

func (c *Client) buildMessages(msgs []llm.Message) []wireMessage {
	out := make([]wireMessage, 0, len(msgs))
	for _, m := range msgs {
		// Only attach images when the backend supports them and there are any.
		if c.supportsImages && len(m.Images) > 0 {
			parts := make([]contentPart, 0, len(m.Images)+1)
			if m.Text != "" {
				parts = append(parts, contentPart{Type: "text", Text: m.Text})
			}
			for _, img := range m.Images {
				dataURL := fmt.Sprintf("data:%s;base64,%s", img.MIME, base64.StdEncoding.EncodeToString(img.Data))
				parts = append(parts, contentPart{Type: "image_url", ImageURL: &imageURL{URL: dataURL}})
			}
			out = append(out, wireMessage{Role: string(m.Role), Content: parts})
			continue
		}
		out = append(out, wireMessage{Role: string(m.Role), Content: m.Text})
	}
	return out
}

func (c *Client) Stream(ctx context.Context, req llm.ChatRequest) (<-chan llm.Chunk, error) {
	payload := chatRequest{
		Model:       c.model,
		Messages:    c.buildMessages(req.Messages),
		Stream:      req.Stream,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("contacting backend at %s: %w", c.baseURL, err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("backend returned %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}

	out := make(chan llm.Chunk)
	if req.Stream {
		go c.readSSE(resp, out)
	} else {
		go c.readFull(resp, out)
	}
	return out, nil
}

// readSSE parses a Server-Sent Events stream of chat completion deltas.
func (c *Client) readSSE(resp *http.Response, out chan<- llm.Chunk) {
	defer resp.Body.Close()
	defer close(out)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			out <- llm.Chunk{Done: true}
			return
		}
		var sr streamResponse
		if err := json.Unmarshal([]byte(data), &sr); err != nil {
			continue // skip malformed keep-alive / comment lines
		}
		if sr.Error != nil {
			out <- llm.Chunk{Err: sr.Error}
			return
		}
		for _, ch := range sr.Choices {
			if ch.Delta.Content != "" {
				out <- llm.Chunk{Delta: ch.Delta.Content}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		out <- llm.Chunk{Err: fmt.Errorf("reading stream: %w", err)}
		return
	}
	out <- llm.Chunk{Done: true}
}

// readFull parses a single non-streaming JSON response.
func (c *Client) readFull(resp *http.Response, out chan<- llm.Chunk) {
	defer resp.Body.Close()
	defer close(out)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		out <- llm.Chunk{Err: fmt.Errorf("reading response: %w", err)}
		return
	}
	var fr fullResponse
	if err := json.Unmarshal(body, &fr); err != nil {
		out <- llm.Chunk{Err: fmt.Errorf("decoding response: %w", err)}
		return
	}
	if fr.Error != nil {
		out <- llm.Chunk{Err: fr.Error}
		return
	}
	if len(fr.Choices) > 0 {
		out <- llm.Chunk{Delta: fr.Choices[0].Message.Content}
	}
	out <- llm.Chunk{Done: true}
}

// Reachable performs a lightweight GET against the models endpoint to check
// whether the backend is up. It is used by the server manager's health poll.
func Reachable(ctx context.Context, baseURL string) bool {
	baseURL = strings.TrimRight(baseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}
