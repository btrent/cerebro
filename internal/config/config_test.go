package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultIsValid(t *testing.T) {
	c := Default()
	if err := c.Validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
	if _, ok := c.Models[c.DefaultModel]; !ok {
		t.Fatalf("default model %q missing from models", c.DefaultModel)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	c := Default()
	c.path = path
	c.ActiveModel = "gemma4-26b"
	if err := c.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.ActiveModel != c.ActiveModel {
		t.Errorf("active model: got %q want %q", got.ActiveModel, c.ActiveModel)
	}
	if got.Server.BaseURL != c.Server.BaseURL {
		t.Errorf("base url: got %q want %q", got.Server.BaseURL, c.Server.BaseURL)
	}
	if len(got.Models) != len(c.Models) {
		t.Errorf("models count: got %d want %d", len(got.Models), len(c.Models))
	}
}

func TestLoadMissingReturnsDefault(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if c.DefaultModel != "gemma4-26b" {
		t.Errorf("expected default config, got default model %q", c.DefaultModel)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("default_model: [unclosed\n  : :"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestValidateRepairsActiveModel(t *testing.T) {
	c := Default()
	c.ActiveModel = "does-not-exist"
	if err := c.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if c.ActiveModel != c.DefaultModel {
		t.Errorf("active model not repaired: got %q", c.ActiveModel)
	}
}

func TestSetActiveModel(t *testing.T) {
	c := Default()
	if err := c.SetActiveModel("missing"); err == nil {
		t.Fatal("expected error for unknown model")
	}
	if err := c.SetActiveModel("gemma4-26b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestActiveSpec(t *testing.T) {
	c := Default()
	spec, err := c.ActiveSpec()
	if err != nil {
		t.Fatalf("active spec: %v", err)
	}
	if spec.Provider != "openai" || !spec.SupportsImages {
		t.Errorf("unexpected spec: %+v", spec)
	}
}

func TestLoadOrCreate(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	c, created, err := LoadOrCreate()
	if err != nil {
		t.Fatalf("first LoadOrCreate: %v", err)
	}
	if !created {
		t.Error("expected created=true on first run")
	}
	if c.DefaultModel != "gemma4-26b" {
		t.Errorf("unexpected default model %q", c.DefaultModel)
	}

	_, created2, err := LoadOrCreate()
	if err != nil {
		t.Fatalf("second LoadOrCreate: %v", err)
	}
	if created2 {
		t.Error("expected created=false on second run")
	}
}
