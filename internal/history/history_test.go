package history

import (
	"testing"
)

func TestSessionRoundTrip(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	c := NewSession("gemma4-26b")
	c.Append("user", "hello", []string{"a.png"})
	c.Append("assistant", "hi there", nil)

	if err := c.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	files, err := List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 saved file, got %d", len(files))
	}

	got, err := Load(files[0])
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Model != "gemma4-26b" {
		t.Errorf("model: got %q", got.Model)
	}
	if len(got.Turns) != 2 {
		t.Fatalf("turns: got %d want 2", len(got.Turns))
	}
	if got.Turns[0].Text != "hello" || len(got.Turns[0].Images) != 1 {
		t.Errorf("turn 0 wrong: %+v", got.Turns[0])
	}
	if got.Turns[1].Role != "assistant" {
		t.Errorf("turn 1 role: %q", got.Turns[1].Role)
	}
}

func TestClear(t *testing.T) {
	c := NewSession("m")
	c.Append("user", "x", nil)
	c.Clear()
	if len(c.Turns) != 0 {
		t.Errorf("expected cleared turns, got %d", len(c.Turns))
	}
}

func TestListEmpty(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	files, err := List()
	if err != nil {
		t.Fatalf("list on empty dir: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected no files, got %d", len(files))
	}
}
