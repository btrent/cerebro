package config

import (
	"fmt"
	"os"
	"time"

	"cerebro/internal/paths"
)

func secs(n int) time.Duration { return time.Duration(n) * time.Second }

// commentedDefault is the human-friendly config file written on first run. It
// parses back into the same values as Default() but includes guidance and a
// commented Ollama example so users can extend it by editing, not coding.
func commentedDefault() string {
	return fmt.Sprintf(`# cerebro configuration
# Docs: see README.md. Add a model by adding an entry under "models:" whose
# "provider" is a registered backend ("openai" for any OpenAI-compatible server,
# or "ollama"). No code changes are needed.

default_model: gemma4-26b
active_model: gemma4-26b

# Optional cerebro-managed backend. When auto_launch is true and base_url is not
# already reachable, cerebro runs launch_command and waits for health_path.
server:
  auto_launch: true
  base_url: http://127.0.0.1:8080/v1
  health_path: /models
  startup_timeout_seconds: 240
  launch_command: %s -m mlx_vlm.server --model mlx-community/gemma-4-26b-a4b-it-4bit --port 8080

models:
  gemma4-26b:
    display_name: Gemma 4 26B-A4B
    provider: openai            # OpenAI-compatible (served by mlx_vlm.server)
    model: mlx-community/gemma-4-26b-a4b-it-4bit
    base_url: http://127.0.0.1:8080/v1
    supports_images: true

  # Example: an Ollama model (uncomment after 'ollama pull'):
  # llama3:
  #   display_name: Llama 3
  #   provider: ollama
  #   model: llama3
  #   base_url: http://127.0.0.1:11434
  #   supports_images: false
`, paths.VenvPython())
}

// LoadOrCreate loads the config at the standard path, writing a commented
// default file first if none exists. The returned bool reports whether a new
// file was created.
func LoadOrCreate() (Config, bool, error) {
	path := paths.ConfigFile()
	created := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := paths.EnsureDir(paths.ConfigDir()); err != nil {
			return Config{}, false, err
		}
		if err := os.WriteFile(path, []byte(commentedDefault()), 0o644); err != nil {
			return Config{}, false, err
		}
		created = true
	}
	c, err := Load(path)
	return c, created, err
}
