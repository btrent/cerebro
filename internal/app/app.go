// Package app wires cerebro's components together and runs the TUI program.
package app

import (
	"fmt"

	"cerebro/internal/config"
	"cerebro/internal/history"
	"cerebro/internal/llm"
	"cerebro/internal/prompts"
	"cerebro/internal/tui"

	// Register the available providers via their init() functions. Adding a new
	// backend means adding an import here and a config entry — no other changes.
	_ "cerebro/internal/llm/ollama"
	_ "cerebro/internal/llm/openai"

	tea "github.com/charmbracelet/bubbletea"
)

// Run loads configuration, constructs dependencies, and runs the TUI until the
// user exits. The managed backend (if any) is shut down on return.
func Run() error {
	cfg, _, err := config.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("%w\n\nEdit or remove the config file to recover, then relaunch", err)
	}

	store, err := prompts.LoadDefault()
	if err != nil {
		return fmt.Errorf("loading header prompts: %w", err)
	}

	spec, err := cfg.ActiveSpec()
	if err != nil {
		return err
	}
	provider, err := llm.New(spec)
	if err != nil {
		return fmt.Errorf("configuring model %q: %w", cfg.ActiveModel, err)
	}

	manager := llm.NewManager(cfg.ServerConfig())
	defer manager.Shutdown()

	session := history.NewSession(cfg.ActiveModel)

	model := tui.New(tui.Deps{
		Config:   &cfg,
		Prompts:  store,
		Session:  session,
		Manager:  manager,
		Provider: provider,
	})

	program := tea.NewProgram(model, tea.WithAltScreen())
	_, err = program.Run()
	// Persist the session transcript on exit (best-effort).
	_ = session.Save()
	return err
}
