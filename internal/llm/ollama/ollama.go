// Package ollama implements the llm.Provider interface using Ollama's native
// /api/chat endpoint. It is registered so that once Ollama is installed, adding a
// model entry with `provider: ollama` to the config works without code changes.
package ollama

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

	"cerebro/internal/llm"
)

func init() {
	llm.Register("ollama", func(spec llm.ModelSpec) (llm.Provider, error) {
		baseURL := spec.BaseURL
		if baseURL == "" {
			baseURL = "http://127.0.0.1:11434"
		}
		return &Client{
			model:          spec.Model,
			baseURL:        strings.TrimRight(baseURL, "/"),
			supportsImages: spec.SupportsImages,
			http:           &http.Client{},
		}, nil
	})
}

// Client talks to a single Ollama server for a single model.
type Client struct {
	model          string
	baseURL        string
	supportsImages bool
	http           *http.Client
}

func (c *Client) Name() string         { return "ollama" }
func (c *Client) SupportsImages() bool { return c.supportsImages }

type wireMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"` // base64-encoded, no data: prefix
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []wireMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done  bool   `json:"done"`
	Error string `json:"error"`
}

func (c *Client) buildMessages(msgs []llm.Message) []wireMessage {
	out := make([]wireMessage, 0, len(msgs))
	for _, m := range msgs {
		wm := wireMessage{Role: string(m.Role), Content: m.Text}
		if c.supportsImages {
			for _, img := range m.Images {
				wm.Images = append(wm.Images, base64.StdEncoding.EncodeToString(img.Data))
			}
		}
		out = append(out, wm)
	}
	return out
}

func (c *Client) Stream(ctx context.Context, req llm.ChatRequest) (<-chan llm.Chunk, error) {
	payload := chatRequest{
		Model:    c.model,
		Messages: c.buildMessages(req.Messages),
		Stream:   req.Stream,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("contacting Ollama at %s: %w", c.baseURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("ollama returned %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}

	out := make(chan llm.Chunk)
	go c.readStream(resp, out)
	return out, nil
}

// readStream parses Ollama's newline-delimited JSON object stream.
func (c *Client) readStream(resp *http.Response, out chan<- llm.Chunk) {
	defer resp.Body.Close()
	defer close(out)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var cr chatResponse
		if err := json.Unmarshal([]byte(line), &cr); err != nil {
			continue
		}
		if cr.Error != "" {
			out <- llm.Chunk{Err: fmt.Errorf("%s", cr.Error)}
			return
		}
		if cr.Message.Content != "" {
			out <- llm.Chunk{Delta: cr.Message.Content}
		}
		if cr.Done {
			out <- llm.Chunk{Done: true}
			return
		}
	}
	if err := scanner.Err(); err != nil {
		out <- llm.Chunk{Err: fmt.Errorf("reading stream: %w", err)}
		return
	}
	out <- llm.Chunk{Done: true}
}
