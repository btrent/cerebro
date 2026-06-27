// Package config loads, validates, and persists cerebro's user configuration
// (config.yaml): the set of available models, the active/default model, and the
// optional managed-backend server settings.
package config

import (
	"fmt"
	"os"

	"cerebro/internal/llm"
	"cerebro/internal/paths"

	"gopkg.in/yaml.v3"
)

// ModelConfig describes one model the user can select. Adding a model is purely a
// config edit — no code changes — as long as its provider is a registered kind.
type ModelConfig struct {
	DisplayName    string `yaml:"display_name"`
	Provider       string `yaml:"provider"`
	Model          string `yaml:"model"`
	BaseURL        string `yaml:"base_url"`
	SupportsImages bool   `yaml:"supports_images"`
	APIKey         string `yaml:"api_key,omitempty"`
}

// ServerSettings configures the optional cerebro-managed inference server.
type ServerSettings struct {
	AutoLaunch            bool   `yaml:"auto_launch"`
	BaseURL               string `yaml:"base_url"`
	HealthPath            string `yaml:"health_path"`
	StartupTimeoutSeconds int    `yaml:"startup_timeout_seconds"`
	LaunchCommand         string `yaml:"launch_command"`
}

// Config is the top-level config.yaml document.
type Config struct {
	DefaultModel string                 `yaml:"default_model"`
	ActiveModel  string                 `yaml:"active_model"`
	Server       ServerSettings         `yaml:"server"`
	Models       map[string]ModelConfig `yaml:"models"`

	path string `yaml:"-"` // source path, for Save
}

// Default returns the built-in default configuration: Gemma 4 26B-A4B served by
// a cerebro-managed mlx_vlm.server out of the dedicated venv.
func Default() Config {
	return Config{
		DefaultModel: "gemma4-26b",
		ActiveModel:  "gemma4-26b",
		Server: ServerSettings{
			AutoLaunch:            true,
			BaseURL:               "http://127.0.0.1:8080/v1",
			HealthPath:            "/models",
			StartupTimeoutSeconds: 240,
			LaunchCommand: fmt.Sprintf(
				"%s -m mlx_vlm.server --model mlx-community/gemma-4-26b-a4b-it-4bit --port 8080",
				paths.VenvPython()),
		},
		Models: map[string]ModelConfig{
			"gemma4-26b": {
				DisplayName:    "Gemma 4 26B-A4B",
				Provider:       "openai",
				Model:          "mlx-community/gemma-4-26b-a4b-it-4bit",
				BaseURL:        "http://127.0.0.1:8080/v1",
				SupportsImages: true,
			},
		},
	}
}

// Load reads and validates config from path. A missing file is not an error: it
// returns the Default config (marked with the path) so callers can Save it. An
// existing-but-malformed file returns a descriptive error.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		c := Default()
		c.path = path
		return c, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("reading %s: %w", path, err)
	}

	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("invalid config %s: %w", path, err)
	}
	c.path = path
	if err := c.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return c, nil
}

// Validate ensures the config is internally consistent, repairing the active
// model when possible (falling back to default, then to any model).
func (c *Config) Validate() error {
	if len(c.Models) == 0 {
		return fmt.Errorf("no models defined")
	}
	if c.DefaultModel == "" {
		for name := range c.Models {
			c.DefaultModel = name
			break
		}
	}
	if _, ok := c.Models[c.DefaultModel]; !ok {
		return fmt.Errorf("default_model %q not found in models", c.DefaultModel)
	}
	if _, ok := c.Models[c.ActiveModel]; !ok {
		c.ActiveModel = c.DefaultModel
	}
	for name, m := range c.Models {
		if m.Provider == "" {
			return fmt.Errorf("model %q: provider is required", name)
		}
		if m.Model == "" {
			return fmt.Errorf("model %q: model is required", name)
		}
	}
	return nil
}

// Save writes the config back to its source path (creating the directory).
func (c *Config) Save() error {
	path := c.path
	if path == "" {
		path = paths.ConfigFile()
	}
	if err := paths.EnsureDir(paths.ConfigDir()); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Path returns the source path of this config.
func (c *Config) Path() string { return c.path }

// ActiveModelConfig returns the config entry for the active model.
func (c *Config) ActiveModelConfig() (ModelConfig, bool) {
	m, ok := c.Models[c.ActiveModel]
	return m, ok
}

// ActiveSpec resolves the active model into an llm.ModelSpec for provider
// construction.
func (c *Config) ActiveSpec() (llm.ModelSpec, error) {
	m, ok := c.Models[c.ActiveModel]
	if !ok {
		return llm.ModelSpec{}, fmt.Errorf("active model %q not found", c.ActiveModel)
	}
	return llm.ModelSpec{
		Provider:       m.Provider,
		Model:          m.Model,
		BaseURL:        m.BaseURL,
		SupportsImages: m.SupportsImages,
		APIKey:         m.APIKey,
	}, nil
}

// SetActiveModel switches the active model after verifying it exists.
func (c *Config) SetActiveModel(name string) error {
	if _, ok := c.Models[name]; !ok {
		return fmt.Errorf("unknown model %q", name)
	}
	c.ActiveModel = name
	return nil
}

// ServerConfig converts the YAML server settings into an llm.ServerConfig.
func (c *Config) ServerConfig() llm.ServerConfig {
	return llm.ServerConfig{
		AutoLaunch:     c.Server.AutoLaunch,
		BaseURL:        c.Server.BaseURL,
		HealthPath:     c.Server.HealthPath,
		StartupTimeout: secs(c.Server.StartupTimeoutSeconds),
		LaunchCommand:  c.Server.LaunchCommand,
	}
}
