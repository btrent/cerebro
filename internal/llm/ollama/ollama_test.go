package ollama

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cerebro/internal/llm"
)

func TestStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		io.WriteString(w, `{"message":{"content":"Hel"},"done":false}`+"\n")
		io.WriteString(w, `{"message":{"content":"lo"},"done":false}`+"\n")
		io.WriteString(w, `{"message":{"content":""},"done":true}`+"\n")
	}))
	defer srv.Close()

	p, err := llm.New(llm.ModelSpec{Provider: "ollama", Model: "llama3", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := p.Stream(context.Background(), llm.ChatRequest{Stream: true, Messages: []llm.Message{{Role: llm.RoleUser, Text: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}

	var b strings.Builder
	done := false
	for c := range ch {
		if c.Err != nil {
			t.Fatalf("chunk error: %v", c.Err)
		}
		if c.Done {
			done = true
		}
		b.WriteString(c.Delta)
	}
	if b.String() != "Hello" {
		t.Errorf("text: got %q", b.String())
	}
	if !done {
		t.Error("expected done")
	}
}

func TestStreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"error":"model 'x' not found"}`+"\n")
	}))
	defer srv.Close()

	p, _ := llm.New(llm.ModelSpec{Provider: "ollama", Model: "x", BaseURL: srv.URL})
	ch, err := p.Stream(context.Background(), llm.ChatRequest{Stream: true})
	if err != nil {
		t.Fatal(err)
	}
	var gotErr error
	for c := range ch {
		if c.Err != nil {
			gotErr = c.Err
		}
	}
	if gotErr == nil {
		t.Fatal("expected error chunk")
	}
}
