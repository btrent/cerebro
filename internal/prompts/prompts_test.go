package prompts

import (
	"path/filepath"
	"testing"

	"cerebro/internal/llm"
)

func TestCRUD(t *testing.T) {
	s := &Store{}

	if err := s.Create("style", "Be concise."); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Create("style", "dup"); err == nil {
		t.Fatal("expected duplicate error")
	}
	if err := s.Create("", "x"); err == nil {
		t.Fatal("expected empty-name error")
	}

	p, err := s.Get("style")
	if err != nil || p.Body != "Be concise." {
		t.Fatalf("get: %v, %+v", err, p)
	}

	if err := s.Update("style", "Be very concise."); err != nil {
		t.Fatalf("update: %v", err)
	}
	if p, _ := s.Get("style"); p.Body != "Be very concise." {
		t.Errorf("update did not apply: %q", p.Body)
	}

	if err := s.Update("missing", "x"); err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUseAndNone(t *testing.T) {
	s := &Store{}
	_ = s.Create("role", "You are a pirate.")

	if err := s.Use("role"); err != nil {
		t.Fatalf("use: %v", err)
	}
	if s.ActiveName() != "role" {
		t.Errorf("active name: %q", s.ActiveName())
	}
	if ap, ok := s.ActivePrompt(); !ok || ap.Body != "You are a pirate." {
		t.Errorf("active prompt wrong: %+v %v", ap, ok)
	}

	s.None()
	if s.ActiveName() != "none" {
		t.Errorf("expected none, got %q", s.ActiveName())
	}
	if _, ok := s.ActivePrompt(); ok {
		t.Error("expected no active prompt after None")
	}

	if err := s.Use("ghost"); err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteClearsActive(t *testing.T) {
	s := &Store{}
	_ = s.Create("a", "A")
	_ = s.Create("b", "B")
	_ = s.Use("a")

	if err := s.Delete("a"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if s.Active != "" {
		t.Errorf("active should be cleared after deleting active prompt, got %q", s.Active)
	}
	if len(s.Prompts) != 1 {
		t.Errorf("expected 1 prompt left, got %d", len(s.Prompts))
	}
	if err := s.Delete("a"); err != ErrNotFound {
		t.Errorf("expected ErrNotFound deleting again, got %v", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "header_prompts.yaml")
	s := &Store{path: path}
	_ = s.Create("style", "Markdown please.")
	_ = s.Use("style")
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.ActiveName() != "style" {
		t.Errorf("active not persisted: %q", got.ActiveName())
	}
	if len(got.Prompts) != 1 || got.Prompts[0].Body != "Markdown please." {
		t.Errorf("prompts not persisted: %+v", got.Prompts)
	}
}

func TestLoadDanglingActiveDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.yaml")
	s := &Store{path: path, Active: "ghost"}
	_ = s.Create("real", "x")
	// Active points at a non-existent prompt; persist raw then reload.
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Active != "" {
		t.Errorf("expected dangling active to be disabled, got %q", got.Active)
	}
}

func TestAssemble(t *testing.T) {
	history := []llm.Message{
		{Role: llm.RoleUser, Text: "hi"},
		{Role: llm.RoleAssistant, Text: "hello"},
	}
	user := llm.Message{Role: llm.RoleUser, Text: "describe", Images: []llm.Image{{Path: "a.png"}}}

	// With header: system first, then history, then user.
	msgs := Assemble("Be terse.", history, user)
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}
	if msgs[0].Role != llm.RoleSystem || msgs[0].Text != "Be terse." {
		t.Errorf("first message should be system header, got %+v", msgs[0])
	}
	if msgs[1].Text != "hi" || msgs[2].Text != "hello" {
		t.Errorf("history order not preserved: %+v", msgs[1:3])
	}
	if last := msgs[3]; last.Text != "describe" || len(last.Images) != 1 {
		t.Errorf("user message/images wrong: %+v", last)
	}

	// Without header: no system message.
	msgs = Assemble("   ", history, user)
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages without header, got %d", len(msgs))
	}
	if msgs[0].Role == llm.RoleSystem {
		t.Error("did not expect a system message when header is blank")
	}
}
