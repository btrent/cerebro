// Package paths centralizes cerebro's on-disk locations, following the XDG Base
// Directory spec on macOS and Linux. It is a dependency-free leaf package so it
// can be imported anywhere without creating import cycles.
//
// Per the project spec we intentionally use the XDG layout on macOS too
// (~/.config, ~/.local/share) rather than ~/Library/Application Support.
package paths

import (
	"os"
	"path/filepath"
)

const appName = "cerebro"

// ConfigDir is ~/.config/cerebro (or $XDG_CONFIG_HOME/cerebro).
func ConfigDir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, appName)
	}
	return filepath.Join(home(), ".config", appName)
}

// DataDir is ~/.local/share/cerebro (or $XDG_DATA_HOME/cerebro).
func DataDir() string {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, appName)
	}
	return filepath.Join(home(), ".local", "share", appName)
}

// ConfigFile is the path to config.yaml.
func ConfigFile() string { return filepath.Join(ConfigDir(), "config.yaml") }

// HeaderPromptsFile is the path to header_prompts.yaml.
func HeaderPromptsFile() string { return filepath.Join(ConfigDir(), "header_prompts.yaml") }

// HistoryDir is the directory holding session transcripts and saved conversations.
func HistoryDir() string { return filepath.Join(DataDir(), "history") }

// VenvDir is the dedicated cerebro Python venv created by `cerebro --setup`.
func VenvDir() string { return filepath.Join(DataDir(), "venv") }

// VenvPython is the python interpreter inside the dedicated venv.
func VenvPython() string { return filepath.Join(VenvDir(), "bin", "python") }

// ServerLogPath is where a cerebro-launched backend's output is written.
func ServerLogPath() string { return filepath.Join(DataDir(), "server.log") }

// EnsureDir creates dir (and parents) with user-only permissions if missing.
func EnsureDir(dir string) error { return os.MkdirAll(dir, 0o755) }

func home() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return "."
	}
	return h
}
