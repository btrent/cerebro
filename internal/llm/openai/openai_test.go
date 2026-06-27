package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cerebro/internal/llm"
)

func collect(t *testing.T, ch <-chan llm.Chunk) (text string, done bool, err error) {
	t.Helper()
	var b strings.Builder
	for c := range ch {
		switch {
		case c.Err != nil:
			err = c.Err
		case c.Done:
			done = true
		default:
			b.WriteString(c.Delta)
		}
	}
	return b.String(), done, err
}

func TestStreamSSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	p, err := llm.New(llm.ModelSpec{Provider: "openai", Model: "m", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := p.Stream(context.Background(), llm.ChatRequest{Stream: true, Messages: []llm.Message{{Role: llm.RoleUser, Text: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	text, done, cerr := collect(t, ch)
	if cerr != nil {
		t.Fatalf("chunk error: %v", cerr)
	}
	if text != "Hello" {
		t.Errorf("text: got %q want %q", text, "Hello")
	}
	if !done {
		t.Error("expected done")
	}
}

func TestNonStreaming(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{"choices":[{"message":{"content":"full answer"}}]}`
		io.WriteString(w, resp)
	}))
	defer srv.Close()

	p, _ := llm.New(llm.ModelSpec{Provider: "openai", Model: "m", BaseURL: srv.URL})
	ch, err := p.Stream(context.Background(), llm.ChatRequest{Stream: false})
	if err != nil {
		t.Fatal(err)
	}
	text, done, cerr := collect(t, ch)
	if cerr != nil {
		t.Fatalf("err: %v", cerr)
	}
	if text != "full answer" || !done {
		t.Errorf("got %q done=%v", text, done)
	}
}

func TestHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not loaded", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	p, _ := llm.New(llm.ModelSpec{Provider: "openai", Model: "m", BaseURL: srv.URL})
	_, err := p.Stream(context.Background(), llm.ChatRequest{Stream: true})
	if err == nil {
		t.Fatal("expected error on non-200 response")
	}
}

func TestImagePayloadEncoding(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	p, _ := llm.New(llm.ModelSpec{Provider: "openai", Model: "m", BaseURL: srv.URL, SupportsImages: true})
	ch, err := p.Stream(context.Background(), llm.ChatRequest{
		Stream: true,
		Messages: []llm.Message{{
			Role:   llm.RoleUser,
			Text:   "what is this",
			Images: []llm.Image{{Path: "a.png", MIME: "image/png", Data: []byte{0x1, 0x2, 0x3}}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	collect(t, ch)

	msgs, ok := captured["messages"].([]any)
	if !ok || len(msgs) == 0 {
		t.Fatalf("messages not captured: %v", captured)
	}
	content, ok := msgs[0].(map[string]any)["content"].([]any)
	if !ok {
		t.Fatalf("expected array content for multimodal message, got %T", msgs[0].(map[string]any)["content"])
	}
	foundImage := false
	for _, part := range content {
		if m, ok := part.(map[string]any); ok && m["type"] == "image_url" {
			foundImage = true
			url := m["image_url"].(map[string]any)["url"].(string)
			if !strings.HasPrefix(url, "data:image/png;base64,") {
				t.Errorf("bad data url: %q", url)
			}
		}
	}
	if !foundImage {
		t.Error("image_url part not found in payload")
	}
}
