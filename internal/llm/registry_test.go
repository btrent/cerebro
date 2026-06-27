package llm_test

import (
	"testing"

	"cerebro/internal/llm"

	// Imported for their init() registration side effects, exactly as the app
	// wires them. This proves config-driven provider selection end to end.
	_ "cerebro/internal/llm/ollama"
	_ "cerebro/internal/llm/openai"
)

func TestSelectOpenAIProvider(t *testing.T) {
	p, err := llm.New(llm.ModelSpec{
		Provider:       "openai",
		Model:          "x",
		BaseURL:        "http://127.0.0.1:8080/v1",
		SupportsImages: true,
	})
	if err != nil {
		t.Fatalf("new openai: %v", err)
	}
	if p.Name() != "openai" {
		t.Errorf("name: %q", p.Name())
	}
	if !p.SupportsImages() {
		t.Error("expected SupportsImages true")
	}
}

func TestSelectOllamaProvider(t *testing.T) {
	p, err := llm.New(llm.ModelSpec{Provider: "ollama", Model: "llama3"})
	if err != nil {
		t.Fatalf("new ollama: %v", err)
	}
	if p.Name() != "ollama" {
		t.Errorf("name: %q", p.Name())
	}
}

func TestUnknownProvider(t *testing.T) {
	if _, err := llm.New(llm.ModelSpec{Provider: "nope", Model: "x"}); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestOpenAIRequiresBaseURL(t *testing.T) {
	if _, err := llm.New(llm.ModelSpec{Provider: "openai", Model: "x"}); err == nil {
		t.Fatal("expected error when base_url is empty")
	}
}

func TestRegisteredIncludesBuiltins(t *testing.T) {
	got := llm.Registered()
	want := map[string]bool{"openai": false, "ollama": false}
	for _, n := range got {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for n, found := range want {
		if !found {
			t.Errorf("provider %q not registered (got %v)", n, got)
		}
	}
}
