package tui

import (
	"sort"
	"strings"

	"cerebro/internal/llm"

	tea "github.com/charmbracelet/bubbletea"
)

const helpText = `Commands:
  /help                     show this help
  /model list               list configured models
  /model use <name>         switch active model
  /header list              list header prompts
  /header new <name>        create a header prompt (opens editor)
  /header edit <name>       edit a header prompt (opens editor)
  /header delete <name>     delete a header prompt
  /header use <name>        activate a header prompt
  /header none              disable the active header prompt
  /clear                    clear the conversation
  /exit                     quit (also: exit, quit, Ctrl+C, Ctrl+D)

Keys:
  Enter send  •  Ctrl+J newline  •  Esc stop generating
  PgUp/PgDn scroll  •  Opt+↑/Opt+↓ line up/down  •  Home/End top/bottom
  Ctrl+L clear  •  Ctrl+H toggle help
  Paste >10 lines collapses to a placeholder (full text is still sent).
  Type an image path (.png .jpg .jpeg .webp .gif) to attach it.
  Ctrl+V pastes an image from the clipboard (macOS).`

// runCommand parses and executes a slash command. It returns the updated model
// and any command to run (e.g. tea.Quit).
func (m Model) runCommand(line string) (Model, tea.Cmd) {
	fields := strings.Fields(strings.TrimPrefix(line, "/"))
	if len(fields) == 0 {
		return m, nil
	}
	cmd := strings.ToLower(fields[0])
	args := fields[1:]

	switch cmd {
	case "help":
		m.addSystem(helpText)

	case "exit", "quit":
		m.quitting = true
		return m, tea.Quit

	case "clear":
		return m.clearConversation(), nil

	case "model":
		return m.runModelCommand(args)

	case "header":
		return m.runHeaderCommand(args)

	default:
		m.addError("Unknown command: /" + cmd + " (try /help)")
	}
	return m, nil
}

func (m Model) runModelCommand(args []string) (Model, tea.Cmd) {
	if len(args) == 0 || args[0] == "list" {
		names := make([]string, 0, len(m.deps.Config.Models))
		for name := range m.deps.Config.Models {
			names = append(names, name)
		}
		sort.Strings(names)
		var b strings.Builder
		b.WriteString("Models:")
		for _, name := range names {
			mc := m.deps.Config.Models[name]
			marker := "  "
			if name == m.deps.Config.ActiveModel {
				marker = "▸ "
			}
			b.WriteString("\n" + marker + name + " — " + mc.DisplayName + " (" + mc.Provider + ")")
		}
		m.addSystem(b.String())
		return m, nil
	}

	if args[0] == "use" {
		if len(args) < 2 {
			m.addError("Usage: /model use <name>")
			return m, nil
		}
		name := args[1]
		if err := m.deps.Config.SetActiveModel(name); err != nil {
			m.addError(err.Error())
			return m, nil
		}
		spec, err := m.deps.Config.ActiveSpec()
		if err != nil {
			m.addError(err.Error())
			return m, nil
		}
		provider, err := llm.New(spec)
		if err != nil {
			m.addError("Cannot use model: " + err.Error())
			return m, nil
		}
		m.deps.Provider = provider
		_ = m.deps.Config.Save()
		m.addSystem("Switched to model: " + name)
		return m, nil
	}

	m.addError("Usage: /model list | /model use <name>")
	return m, nil
}

func (m Model) runHeaderCommand(args []string) (Model, tea.Cmd) {
	if len(args) == 0 {
		m.addError("Usage: /header list|new|edit|delete|use|none")
		return m, nil
	}
	sub := strings.ToLower(args[0])
	rest := args[1:]

	switch sub {
	case "list":
		names := m.deps.Prompts.Names()
		if len(names) == 0 {
			m.addSystem("No header prompts yet. Create one with /header new <name>.")
			return m, nil
		}
		var b strings.Builder
		b.WriteString("Header prompts:")
		for _, n := range names {
			marker := "  "
			if n == m.deps.Prompts.Active {
				marker = "▸ "
			}
			b.WriteString("\n" + marker + n)
		}
		b.WriteString("\nActive: " + m.deps.Prompts.ActiveName())
		m.addSystem(b.String())

	case "new":
		if len(rest) == 0 {
			m.addError("Usage: /header new <name>")
			return m, nil
		}
		name := strings.Join(rest, " ")
		if _, err := m.deps.Prompts.Get(name); err == nil {
			m.addError("Header prompt already exists: " + name + " (use /header edit)")
			return m, nil
		}
		return m.enterEditMode(name, "", true), nil

	case "edit":
		if len(rest) == 0 {
			m.addError("Usage: /header edit <name>")
			return m, nil
		}
		name := strings.Join(rest, " ")
		p, err := m.deps.Prompts.Get(name)
		if err != nil {
			m.addError("No such header prompt: " + name)
			return m, nil
		}
		return m.enterEditMode(p.Name, p.Body, false), nil

	case "delete":
		if len(rest) == 0 {
			m.addError("Usage: /header delete <name>")
			return m, nil
		}
		name := strings.Join(rest, " ")
		if err := m.deps.Prompts.Delete(name); err != nil {
			m.addError("Cannot delete: " + err.Error())
			return m, nil
		}
		_ = m.deps.Prompts.Save()
		m.addSystem("Deleted header prompt: " + name)

	case "use":
		if len(rest) == 0 {
			m.addError("Usage: /header use <name> (or /header none to disable)")
			return m, nil
		}
		name := strings.Join(rest, " ")
		// Treat "/header use none" as a convenient alias for disabling.
		if strings.EqualFold(name, "none") {
			m.deps.Prompts.None()
			_ = m.deps.Prompts.Save()
			m.addSystem("Header prompt disabled (Header: none).")
			return m, nil
		}
		if err := m.deps.Prompts.Use(name); err != nil {
			m.addError("Cannot use: " + err.Error())
			return m, nil
		}
		_ = m.deps.Prompts.Save()
		m.addSystem("Active header prompt: " + name)

	case "none":
		m.deps.Prompts.None()
		_ = m.deps.Prompts.Save()
		m.addSystem("Header prompt disabled (Header: none).")

	default:
		m.addError("Unknown /header subcommand: " + sub)
	}
	return m, nil
}

// enterEditMode opens the header-prompt editor.
func (m Model) enterEditMode(name, body string, isNew bool) Model {
	m.mode = modeHeaderEdit
	m.editName = name
	m.editIsNew = isNew
	m.editor.SetValue(body)
	m.editor.Focus()
	m.input.Blur()
	m.applyLayout()
	return m
}

// handleEditKey handles keys while editing a header prompt.
func (m Model) handleEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeChat
		m.editor.Blur()
		m.input.Focus()
		m.addSystem("Edit cancelled.")
		m.applyLayout()
		m.refreshViewport()
		return m, nil

	case "ctrl+s", "enter":
		body := m.editor.Value()
		if err := m.deps.Prompts.Upsert(m.editName, body); err != nil {
			m.addError("Cannot save: " + err.Error())
			return m, nil
		}
		_ = m.deps.Prompts.Save()
		m.mode = modeChat
		m.editor.Blur()
		m.input.Focus()
		verb := "Updated"
		if m.editIsNew {
			verb = "Created"
		}
		m.addSystem(verb + " header prompt: " + m.editName + " (activate with /header use " + m.editName + ")")
		m.applyLayout()
		m.refreshViewport()
		return m, nil
	}

	// Strip terminal report noise (e.g. OSC color responses) from editor input.
	if msg.Type == tea.KeyRunes || msg.Paste {
		clean, pure := cleanInput(msg)
		if pure {
			return m, nil
		}
		if clean != string(msg.Runes) {
			m.editor.InsertString(clean)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	return m, cmd
}
